// Package api exposes the StableGuard REST API.
package api

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	"stableguard-backend/ai"
	"stableguard-backend/alerts"
	"stableguard-backend/auth"
	"stableguard-backend/config"
	"stableguard-backend/hub"
	"stableguard-backend/jupiter"
	"stableguard-backend/llm"
	"stableguard-backend/onchain"
	"stableguard-backend/pipeline"
	"stableguard-backend/prediction"
	"stableguard-backend/pyth"
	"stableguard-backend/risk"
	solanaexec "stableguard-backend/solana"
	"stableguard-backend/store"
	"stableguard-backend/yield"

	"github.com/gofiber/fiber/v2"
)

func executionUnavailableNote() string {
	return "Market execution is unavailable under the current custody architecture: vault token accounts are program-owned, so external routers like Jupiter cannot spend them directly. Use mode=record only for an explicit accounting-only allocation shift."
}

func authDebugResponsesEnabled() bool {
	switch os.Getenv("STABLEGUARD_DEV_AUTH_DEBUG") {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func liveYieldStatus(cfg *config.Config) (string, string) {
	readiness := cfg.YieldExecutionReadiness()
	return readiness.Mode, readiness.Note
}

type controlProfile struct {
	Name                string
	StrategyMode        uint8
	AutoExecute         bool
	YieldEnabled        bool
	YieldEntryRisk      float64
	YieldExitRisk       float64
	CircuitBreakerPause float64
	RiskThreshold       uint64
	Description         string
}

func controlModeProfile(mode string) (controlProfile, bool) {
	switch mode {
	case "MANUAL":
		return controlProfile{
			Name:                "MANUAL",
			StrategyMode:        0,
			AutoExecute:         false,
			YieldEnabled:        false,
			YieldEntryRisk:      25,
			YieldExitRisk:       40,
			CircuitBreakerPause: 1.0,
			RiskThreshold:       70,
			Description:         "AI only monitors, explains, and alerts. No automatic execution path is enabled.",
		}, true
	case "GUARDED":
		return controlProfile{
			Name:                "GUARDED",
			StrategyMode:        0,
			AutoExecute:         true,
			YieldEnabled:        false,
			YieldEntryRisk:      20,
			YieldExitRisk:       35,
			CircuitBreakerPause: 0.7,
			RiskThreshold:       60,
			Description:         "Capital-preservation mode. AI reacts conservatively and focuses on extreme-risk protection.",
		}, true
	case "BALANCED":
		return controlProfile{
			Name:                "BALANCED",
			StrategyMode:        1,
			AutoExecute:         true,
			YieldEnabled:        false,
			YieldEntryRisk:      35,
			YieldExitRisk:       55,
			CircuitBreakerPause: 1.5,
			RiskThreshold:       40,
			Description:         "Balanced automation. AI records and acts on moderate risk while keeping yield disabled.",
		}, true
	case "YIELD_MAX":
		return controlProfile{
			Name:                "YIELD_MAX",
			StrategyMode:        2,
			AutoExecute:         true,
			YieldEnabled:        true,
			YieldEntryRisk:      45,
			YieldExitRisk:       65,
			CircuitBreakerPause: 2.0,
			RiskThreshold:       25,
			Description:         "Aggressive automation. AI prioritizes yield and tolerates more volatility before exiting.",
		}, true
	default:
		return controlProfile{}, false
	}
}

func strategyName(mode uint8) string {
	switch mode {
	case 0:
		return "SAFE"
	case 2:
		return "YIELD"
	default:
		return "BALANCED"
	}
}

func deriveControlMode(cfg *config.Config) string {
	if cfg == nil {
		return "UNKNOWN"
	}
	switch {
	case !cfg.AutoExecute && !cfg.YieldEnabled:
		return "MANUAL"
	case cfg.StrategyMode == 0 && cfg.AutoExecute && !cfg.YieldEnabled:
		return "GUARDED"
	case cfg.StrategyMode == 2 && cfg.YieldEnabled:
		return "YIELD_MAX"
	default:
		return "BALANCED"
	}
}

func onChainStrategyMode(mode uint8) uint8 {
	if mode == 0 {
		return 0
	}
	return 1
}

// Handler holds all service dependencies.
type Handler struct {
	pyth         *pyth.Monitor
	llm          *llm.Client
	executor     *solanaexec.Executor
	cfg          *config.Config
	pipe         *pipeline.Engine          // optional
	store        *store.DB                 // optional
	alerter      *alerts.Client            // optional
	feedHub      *hub.Hub                  // optional — SSE broadcast hub
	yieldAgg     *yield.Aggregator         // optional — yield APY aggregator
	whaleAgg     *onchain.Aggregator       // optional — on-chain whale signals
	slippageAnal *onchain.SlippageAnalyzer // optional — Jupiter slippage analyzer
}

// New creates a new API handler.
func New(p *pyth.Monitor, l *llm.Client, e *solanaexec.Executor) *Handler {
	return &Handler{pyth: p, llm: l, executor: e}
}

func (h *Handler) WithConfig(cfg *config.Config) *Handler {
	h.cfg = cfg
	return h
}

// WithPipeline attaches the real-time pipeline to the handler (for /risk/v2 etc.).
func (h *Handler) WithPipeline(p *pipeline.Engine) *Handler {
	h.pipe = p
	return h
}

// WithStore attaches the persistent store to the handler (for /history/* etc.).
func (h *Handler) WithStore(s *store.DB) *Handler {
	h.store = s
	return h
}

// WithAlerter attaches the alert client to the handler.
func (h *Handler) WithAlerter(a *alerts.Client) *Handler {
	h.alerter = a
	return h
}

// WithYield attaches the yield aggregator to the handler.
func (h *Handler) WithYield(agg *yield.Aggregator) *Handler {
	h.yieldAgg = agg
	return h
}

// WithHub attaches the SSE broadcast hub to the handler.
func (h *Handler) WithHub(feedHub *hub.Hub) *Handler {
	h.feedHub = feedHub
	return h
}

// WithWhales attaches the on-chain whale aggregator to the handler.
func (h *Handler) WithWhales(agg *onchain.Aggregator) *Handler {
	h.whaleAgg = agg
	return h
}

// WithSlippage attaches the slippage analyzer to the handler.
func (h *Handler) WithSlippage(a *onchain.SlippageAnalyzer) *Handler {
	h.slippageAnal = a
	return h
}

// Register mounts all routes on the given Fiber app.
func (h *Handler) Register(app *fiber.App) {
	v1 := app.Group("/api/v1")

	v1.Get("/health", h.health)
	v1.Get("/prices", h.prices)
	v1.Get("/tokens", h.tokensList)              // all monitored tokens + live prices
	v1.Get("/risk", h.riskScore)                 // v1: simple deviation scorer (unchanged)
	v1.Get("/risk/v2", h.riskScoreV2)            // windowed scorer with trend/velocity/volatility
	v1.Get("/pipeline/status", h.pipelineStatus) // last AI decision + score
	v1.Post("/decide", h.decide)
	v1.Get("/vault", h.vaultState)

	// Core on-chain actions
	v1.Post("/rebalance", authRequired, h.rebalance)
	v1.Post("/strategy", authRequired, h.setStrategy)
	v1.Post("/send", authRequired, h.sendPayment)
	v1.Post("/threshold", authRequired, h.updateThreshold)
	v1.Post("/emergency", authRequired, h.emergencyWithdraw)
	v1.Post("/register-token", authRequired, h.registerToken)

	// History endpoints (require store)
	v1.Get("/history/prices", h.historyPrices)
	v1.Get("/history/decisions", h.historyDecisions)
	v1.Get("/history/rebalances", h.historyRebalances)
	v1.Get("/history/risk-events", h.historyRiskEvents)
	v1.Get("/history/stats", h.historyStats)

	// Real-time SSE feed
	v1.Get("/stream", h.streamFeed)

	// Yield optimizer
	v1.Get("/yield/opportunities", h.yieldOpportunities)
	v1.Get("/yield/position", h.yieldPosition)
	v1.Get("/yield/history", h.yieldHistory)

	// Settings
	v1.Get("/settings", authRequired, h.getSettings)
	v1.Post("/settings/control-mode", authRequired, h.applyControlMode)
	v1.Post("/settings/autopilot", authRequired, h.applyAutopilot)
	v1.Post("/settings/telegram", authRequired, h.setTelegram)
	v1.Post("/settings/discord", authRequired, h.setDiscord)
	v1.Post("/settings/test-alert", authRequired, h.testAlert)

	// AI Chat + Intent Engine
	v1.Post("/chat", h.chat)
	v1.Post("/intent", h.parseIntent)

	// Goals
	v1.Get("/goals", h.listGoals)
	v1.Post("/goals", h.createGoal)
	v1.Patch("/goals/:id/progress", h.updateGoalProgress)
	v1.Delete("/goals/:id", h.deleteGoal)

	// Auth
	v1.Post("/auth/register", h.authRegister)
	v1.Post("/auth/login", h.authLogin)
	v1.Post("/auth/send-otp", h.authSendOTP)
	v1.Post("/auth/verify-otp", h.authVerifyOTP)

	// Jupiter quote (mainnet, best-effort)
	v1.Get("/quote", h.jupiterQuote)

	// ML Prediction (requires Python ml-service running)
	v1.Get("/prediction/depeg", h.depegPrediction)

	// On-chain whale intelligence
	v1.Get("/onchain/whales", h.whaleIntelligence)

	// On-chain slippage / liquidity depth (Jupiter mainnet data)
	v1.Get("/onchain/slippage", h.slippageAnalysis)

	// Wallet auth (Web3 sign-message flow)
	v1.Post("/auth/wallet-login", h.walletLogin)

	// Delegate agent on-chain
	v1.Post("/delegate-agent", authRequired, h.delegateAgent)

	// Vault LUT management
	v1.Post("/vault/init-lut", authRequired, h.initVaultLUT)
	v1.Get("/vault/lut", authRequired, h.vaultLUT)
}

// GET /api/v1/yield/opportunities — live APY from Kamino, Marginfi, Drift
func (h *Handler) yieldOpportunities(c *fiber.Ctx) error {
	if h.yieldAgg == nil {
		return c.JSON(fiber.Map{"opportunities": []interface{}{}})
	}
	opps := h.yieldAgg.Opportunities(c.Context())
	return c.JSON(fiber.Map{
		"opportunities": opps,
		"count":         len(opps),
		"updated_at":    time.Now().Unix(),
	})
}

// GET /api/v1/yield/position — currently active yield position
func (h *Handler) yieldPosition(c *fiber.Ctx) error {
	if h.store == nil {
		return c.JSON(fiber.Map{"position": nil})
	}
	pos, err := h.store.ActiveYieldPosition()
	if err != nil {
		return c.JSON(fiber.Map{"position": nil})
	}
	return c.JSON(fiber.Map{"position": pos})
}

// GET /api/v1/yield/history — recent yield positions
func (h *Handler) yieldHistory(c *fiber.Ctx) error {
	if h.store == nil {
		return c.JSON(fiber.Map{"positions": []interface{}{}})
	}
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	positions, err := h.store.RecentYieldPositions(limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"positions": positions})
}

// GET /api/v1/health
func (h *Handler) health(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok", "service": "stableguard-backend"})
}

// GET /api/v1/prices — fetch latest Pyth prices
func (h *Handler) prices(c *fiber.Ctx) error {
	snap, err := h.pyth.FetchSnapshot()
	if err != nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, fmt.Sprintf("pyth: %v", err))
	}
	return c.JSON(fiber.Map{
		"usdc":          fiber.Map{"price": snap.USDC.Price, "confidence": snap.USDC.Confidence, "publish_time": snap.USDC.PublishTime},
		"usdt":          fiber.Map{"price": snap.USDT.Price, "confidence": snap.USDT.Confidence, "publish_time": snap.USDT.PublishTime},
		"deviation_pct": snap.Deviation(),
		"fetched_at":    snap.FetchedAt,
	})
}

// GET /api/v1/risk — fetch prices and compute risk score
func (h *Handler) riskScore(c *fiber.Ctx) error {
	snap, err := h.pyth.FetchSnapshot()
	if err != nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, fmt.Sprintf("pyth: %v", err))
	}

	var balances []uint64
	var strategyMode uint8
	vs, err := h.fetchVault(c.Context())
	if err == nil {
		n := int(vs.NumTokens)
		balances = make([]uint64, n)
		for i := 0; i < n; i++ {
			balances[i] = vs.Balances[i]
		}
		strategyMode = vs.StrategyMode
	}

	score := risk.Compute(snap, balances, strategyMode)
	return c.JSON(fiber.Map{
		"risk_level":         score.RiskLevel,
		"deviation_pct":      score.Deviation,
		"from_index":         score.FromIndex,
		"to_index":           score.ToIndex,
		"suggested_fraction": score.SuggestedFraction,
		"action":             score.Action,
		"summary":            score.Summary,
		"strategy_mode":      strategyMode,
	})
}

// POST /api/v1/decide — fetch prices, score risk, ask Claude for a decision
func (h *Handler) decide(c *fiber.Ctx) error {
	snap, err := h.pyth.FetchSnapshot()
	if err != nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, fmt.Sprintf("pyth: %v", err))
	}

	var balances []uint64
	var strategyMode uint8
	vs, err := h.fetchVault(c.Context())
	if err == nil {
		n := int(vs.NumTokens)
		balances = make([]uint64, n)
		for i := 0; i < n; i++ {
			balances[i] = vs.Balances[i]
		}
		strategyMode = vs.StrategyMode
	}

	score := risk.Compute(snap, balances, strategyMode)
	decision, err := h.llm.Decide(c.Context(), snap, score, strategyMode)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("llm: %v", err))
	}

	return c.JSON(fiber.Map{
		"action":        decision.Action,
		"rationale":     decision.Rationale,
		"confidence":    decision.Confidence,
		"risk":          score,
		"strategy_mode": strategyMode,
		"prices":        fiber.Map{"usdc": snap.USDC.Price, "usdt": snap.USDT.Price},
	})
}

// GET /api/v1/vault — fetch on-chain vault state
func (h *Handler) vaultState(c *fiber.Ctx) error {
	vs, err := h.fetchVault(c.Context())
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, fmt.Sprintf("vault: %v", err))
	}

	n := int(vs.NumTokens)
	balances := make([]uint64, n)
	mints := make([]string, n)
	for i := 0; i < n; i++ {
		balances[i] = vs.Balances[i]
		mints[i] = solanaexec.PubkeyToBase58(vs.Mints[i])
	}

	authority := h.executor.WalletAddress()
	return c.JSON(fiber.Map{
		"authority":           authority.String(),
		"num_tokens":          vs.NumTokens,
		"mints":               mints,
		"balances":            balances,
		"total_deposited":     vs.TotalDeposited,
		"rebalance_threshold": vs.RebalanceThreshold,
		"max_deposit":         vs.MaxDeposit,
		"decision_count":      vs.DecisionCount,
		"total_rebalances":    vs.TotalRebalances,
		"is_paused":           vs.IsPaused,
		"strategy_mode":       vs.StrategyMode,
	})
}

// POST /api/v1/rebalance — execute virtual rebalance on-chain
// body: {"from_index": 0, "to_index": 1, "amount": 50000000}
func (h *Handler) rebalance(c *fiber.Ctx) error {
	var req struct {
		FromIndex uint8  `json:"from_index"`
		ToIndex   uint8  `json:"to_index"`
		Amount    uint64 `json:"amount"`
		Mode      string `json:"mode"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if req.FromIndex == req.ToIndex {
		return fiber.NewError(fiber.StatusBadRequest, "from_index and to_index must differ")
	}
	if req.Amount == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "amount must be > 0")
	}

	if req.Mode == "" {
		req.Mode = "market"
	}

	switch req.Mode {
	case "record":
		sig, err := h.executor.ExecuteRebalance(c.Context(), req.FromIndex, req.ToIndex, req.Amount)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("execute_rebalance: %v", err))
		}
		if h.store != nil {
			_ = h.store.SaveRebalance(int(req.FromIndex), int(req.ToIndex), req.Amount, sig, 0)
		}
		return c.JSON(fiber.Map{
			"mode":       "record",
			"kind":       "virtual_allocation_shift",
			"signature":  sig,
			"from_index": req.FromIndex,
			"to_index":   req.ToIndex,
			"amount":     req.Amount,
			"message":    "This records an internal allocation shift on-chain. It does not execute a market swap.",
			"explorer":   fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", sig),
		})
	case "market":
		resp := fiber.Map{
			"mode":       "market",
			"kind":       "execution_unavailable",
			"from_index": req.FromIndex,
			"to_index":   req.ToIndex,
			"amount":     req.Amount,
			"message":    executionUnavailableNote(),
		}
		if preview := h.quotePreview(req.FromIndex, req.ToIndex, req.Amount); preview != nil {
			resp["quote_preview"] = preview
		}
		return c.Status(fiber.StatusConflict).JSON(resp)
	default:
		return fiber.NewError(fiber.StatusBadRequest, "mode must be one of: market, record")
	}
}

// POST /api/v1/strategy — change vault strategy mode
// body: {"mode": 0}  → 0=safe, 1=balanced, 2=yield
func (h *Handler) setStrategy(c *fiber.Ctx) error {
	var req struct {
		Mode uint8 `json:"mode"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body: expected {mode}")
	}
	if req.Mode > 2 {
		return fiber.NewError(fiber.StatusBadRequest, "mode must be 0 (safe), 1 (balanced), or 2 (yield)")
	}

	sig, err := h.executor.SendSetStrategy(c.Context(), onChainStrategyMode(req.Mode))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("set_strategy: %v", err))
	}
	if h.cfg != nil {
		h.cfg.StrategyMode = req.Mode
	}

	return c.JSON(fiber.Map{
		"mode":            strategyName(req.Mode),
		"mode_id":         req.Mode,
		"onchain_mode_id": onChainStrategyMode(req.Mode),
		"signature":       sig,
		"explorer":        fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", sig),
	})
}

// POST /api/v1/send — send tokens from vault to recipient
// body: {"token_index": 0, "amount": 1000000, "recipient": "<token_account_pubkey>"}
func (h *Handler) sendPayment(c *fiber.Ctx) error {
	var req struct {
		TokenIndex uint8  `json:"token_index"`
		Amount     uint64 `json:"amount"`
		Recipient  string `json:"recipient"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if req.Amount == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "amount must be > 0")
	}
	if req.Recipient == "" {
		return fiber.NewError(fiber.StatusBadRequest, "recipient is required")
	}

	sig, err := h.executor.SendPayment(c.Context(), req.Amount, req.Recipient, req.TokenIndex)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("send_payment: %v", err))
	}

	return c.JSON(fiber.Map{
		"signature":   sig,
		"token_index": req.TokenIndex,
		"amount":      req.Amount,
		"recipient":   req.Recipient,
		"explorer":    fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", sig),
	})
}

// POST /api/v1/threshold — update rebalance threshold (1–100)
// body: {"threshold": 50}
func (h *Handler) updateThreshold(c *fiber.Ctx) error {
	var req struct {
		Threshold uint64 `json:"threshold"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if req.Threshold < 1 || req.Threshold > 100 {
		return fiber.NewError(fiber.StatusBadRequest, "threshold must be between 1 and 100")
	}

	var oldThreshold uint64
	vs, err := h.fetchVault(c.Context())
	if err == nil {
		oldThreshold = vs.RebalanceThreshold
	}

	sig, err := h.executor.SendUpdateThreshold(c.Context(), req.Threshold)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("update_threshold: %v", err))
	}

	return c.JSON(fiber.Map{
		"old":       oldThreshold,
		"new":       req.Threshold,
		"signature": sig,
		"explorer":  fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", sig),
	})
}

// POST /api/v1/emergency — emergency withdraw all vault tokens to authority
// body: {"authority_token_accounts": ["<pubkey0>", "<pubkey1>", ...]}
func (h *Handler) emergencyWithdraw(c *fiber.Ctx) error {
	var req struct {
		AuthorityTokenAccounts []string `json:"authority_token_accounts"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if len(req.AuthorityTokenAccounts) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "authority_token_accounts required")
	}

	var balances []uint64
	vs, err := h.fetchVault(c.Context())
	if err == nil {
		n := int(vs.NumTokens)
		balances = make([]uint64, n)
		for i := 0; i < n; i++ {
			balances[i] = vs.Balances[i]
		}
	}

	sig, err := h.executor.SendEmergencyWithdraw(c.Context(), req.AuthorityTokenAccounts)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("emergency_withdraw: %v", err))
	}

	return c.JSON(fiber.Map{
		"balances":  balances,
		"signature": sig,
		"explorer":  fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", sig),
	})
}

// POST /api/v1/register-token — register a new token mint into the vault
// body: {"mint": "<pubkey>", "token_index": 0}
func (h *Handler) registerToken(c *fiber.Ctx) error {
	var req struct {
		Mint       string `json:"mint"`
		TokenIndex uint8  `json:"token_index"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if req.Mint == "" {
		return fiber.NewError(fiber.StatusBadRequest, "mint is required")
	}

	sig, err := h.executor.SendRegisterToken(c.Context(), req.Mint, req.TokenIndex)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("register_token: %v", err))
	}

	return c.JSON(fiber.Map{
		"mint":        req.Mint,
		"token_index": req.TokenIndex,
		"signature":   sig,
		"explorer":    fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", sig),
	})
}

// GET /api/v1/risk/v2 — windowed risk score with trend, velocity, volatility
func (h *Handler) riskScoreV2(c *fiber.Ctx) error {
	if h.pipe == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "pipeline not running")
	}
	s := h.pipe.LastScore()
	return c.JSON(fiber.Map{
		"risk_level":         s.RiskLevel,
		"deviation_pct":      s.Deviation,
		"trend":              s.Trend,
		"velocity":           s.Velocity,
		"volatility":         s.Volatility,
		"from_index":         s.FromIndex,
		"to_index":           s.ToIndex,
		"suggested_fraction": s.SuggestedFraction,
		"action":             s.Action,
		"summary":            s.Summary,
		"window_size":        s.WindowSize,
	})
}

// GET /api/v1/pipeline/status — last AI decision + risk score from the pipeline
func (h *Handler) pipelineStatus(c *fiber.Ctx) error {
	if h.pipe == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "pipeline not running")
	}

	score := h.pipe.LastScore()
	decision := h.pipe.LastDecision()
	execSig := h.pipe.LastExecSig()
	execStatus, execNote := h.pipe.LastExecMeta()

	resp := fiber.Map{
		"risk": fiber.Map{
			"risk_level":    score.RiskLevel,
			"deviation_pct": score.Deviation,
			"trend":         score.Trend,
			"velocity":      score.Velocity,
			"volatility":    score.Volatility,
			"action":        score.Action,
			"summary":       score.Summary,
		},
		"policy":           h.pipe.LastPolicyEval(),
		"last_exec_sig":    execSig,
		"last_exec_status": execStatus,
		"last_exec_note":   execNote,
	}

	if decision != nil {
		resp["decision"] = fiber.Map{
			"action":             decision.Action,
			"from_index":         decision.FromIndex,
			"to_index":           decision.ToIndex,
			"suggested_fraction": decision.SuggestedFraction,
			"rationale":          decision.Rationale,
			"confidence":         decision.Confidence,
			"risk_analysis":      decision.RiskAnalysis,
			"yield_analysis":     decision.YieldAnalysis,
		}
	} else {
		resp["decision"] = nil
	}

	return c.JSON(resp)
}

func (h *Handler) quotePreview(fromIndex, toIndex uint8, amount uint64) fiber.Map {
	if int(fromIndex) >= len(pyth.ActiveFeeds) || int(toIndex) >= len(pyth.ActiveFeeds) {
		return nil
	}
	fromFeed := pyth.ActiveFeeds[fromIndex]
	toFeed := pyth.ActiveFeeds[toIndex]

	quote, err := jupiter.GetQuote(jupiter.QuoteRequest{
		InputMint:  fromFeed.MainnetMint,
		OutputMint: toFeed.MainnetMint,
		Amount:     amount,
	})
	if err != nil {
		return fiber.Map{
			"available": false,
			"input":     fromFeed.Symbol,
			"output":    toFeed.Symbol,
			"error":     err.Error(),
		}
	}

	return fiber.Map{
		"available":        true,
		"input":            fromFeed.Symbol,
		"output":           toFeed.Symbol,
		"input_mint":       fromFeed.MainnetMint,
		"output_mint":      toFeed.MainnetMint,
		"out_amount":       quote.OutAmount,
		"price_impact_pct": quote.PriceImpactPct,
		"route_hops":       len(quote.RoutePlan),
		"slippage_bps":     quote.SlippageBps,
	}
}

// riskScore backward-compat: uses old single-snapshot scorer
func (h *Handler) _riskScoreOld(c *fiber.Ctx) (fiber.Map, error) {
	snap, err := h.pyth.FetchSnapshot()
	if err != nil {
		return nil, err
	}
	var balances []uint64
	var strategyMode uint8
	vs, err := h.fetchVault(c.Context())
	if err == nil {
		n := int(vs.NumTokens)
		balances = make([]uint64, n)
		for i := 0; i < n; i++ {
			balances[i] = vs.Balances[i]
		}
		strategyMode = vs.StrategyMode
	}
	score := risk.Compute(snap, balances, strategyMode)
	return fiber.Map{
		"risk_level":         score.RiskLevel,
		"deviation_pct":      score.Deviation,
		"from_index":         score.FromIndex,
		"to_index":           score.ToIndex,
		"suggested_fraction": score.SuggestedFraction,
		"action":             score.Action,
		"summary":            score.Summary,
		"strategy_mode":      strategyMode,
	}, nil
}

// actionToOnChain maps an AI action string to the on-chain action code.
func actionToOnChain(action string) uint8 {
	switch action {
	case ai.ActionProtect:
		return 1
	case ai.ActionOptimize:
		return 2
	default:
		return 0
	}
}

func (h *Handler) fetchVault(ctx context.Context) (*solanaexec.VaultState, error) {
	authority := h.executor.WalletAddress()
	vaultPDA, _, err := h.executor.DeriveVaultPDA(authority)
	if err != nil {
		return nil, err
	}
	return h.executor.FetchVaultState(ctx, vaultPDA)
}

// GET /api/v1/tokens — list all monitored tokens with current live prices
func (h *Handler) tokensList(c *fiber.Ctx) error {
	snap, err := h.pyth.FetchSnapshot()
	if err != nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, fmt.Sprintf("pyth: %v", err))
	}

	type tokenInfo struct {
		Symbol       string  `json:"symbol"`
		Name         string  `json:"name"`
		VaultSlot    int     `json:"vault_slot"`
		MainnetMint  string  `json:"mainnet_mint"`
		Price        float64 `json:"price"`
		Confidence   float64 `json:"confidence"`
		DeviationPct float64 `json:"deviation_pct"` // vs USDC
	}

	tokens := make([]tokenInfo, 0, len(pyth.ActiveFeeds))
	for _, f := range pyth.ActiveFeeds {
		pd := snap.All[f.Symbol]
		tokens = append(tokens, tokenInfo{
			Symbol:       f.Symbol,
			Name:         f.Name,
			VaultSlot:    f.VaultSlot,
			MainnetMint:  f.MainnetMint,
			Price:        pd.Price,
			Confidence:   pd.Confidence,
			DeviationPct: snap.DeviationBetween(f.Symbol, "USDC"),
		})
	}

	return c.JSON(fiber.Map{
		"tokens":        tokens,
		"fetched_at":    snap.FetchedAt,
		"max_deviation": snap.MaxDeviation(),
	})
}

// ── History endpoints ──────────────────────────────────────────────────────

func (h *Handler) requireStore(c *fiber.Ctx) bool {
	if h.store == nil {
		_ = c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "store not available"})
		return false
	}
	return true
}

func limitParam(c *fiber.Ctx, def int) int {
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			return n
		}
	}
	return def
}

// GET /api/v1/history/prices?symbol=USDC&limit=100
func (h *Handler) historyPrices(c *fiber.Ctx) error {
	if !h.requireStore(c) {
		return nil
	}
	symbol := c.Query("symbol", "USDC")
	sinceStr := c.Query("since", "")
	limit := limitParam(c, 200)

	var rows []store.PriceRow
	var err error
	if sinceStr != "" {
		if ts, e := strconv.ParseInt(sinceStr, 10, 64); e == nil {
			rows, err = h.store.PricesSince(symbol, time.Unix(ts, 0))
		} else {
			rows, err = h.store.RecentPrices(symbol, limit)
		}
	} else {
		rows, err = h.store.RecentPrices(symbol, limit)
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	type point struct {
		Ts    int64   `json:"ts"`
		Price float64 `json:"price"`
		Conf  float64 `json:"conf"`
	}
	pts := make([]point, len(rows))
	for i, r := range rows {
		pts[i] = point{Ts: r.Ts.Unix(), Price: r.Price, Conf: r.Confidence}
	}
	return c.JSON(fiber.Map{"symbol": symbol, "data": pts})
}

// GET /api/v1/history/decisions?limit=20
func (h *Handler) historyDecisions(c *fiber.Ctx) error {
	if !h.requireStore(c) {
		return nil
	}
	limit := limitParam(c, 20)
	rows, err := h.store.RecentDecisions(limit)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	type dec struct {
		ID                int64   `json:"id"`
		Ts                int64   `json:"ts"`
		Action            string  `json:"action"`
		FromIndex         int     `json:"from_index"`
		ToIndex           int     `json:"to_index"`
		SuggestedFraction float64 `json:"suggested_fraction"`
		Confidence        int     `json:"confidence"`
		Rationale         string  `json:"rationale"`
		RiskAnalysis      string  `json:"risk_analysis"`
		YieldAnalysis     string  `json:"yield_analysis"`
		RiskLevel         float64 `json:"risk_level"`
		ExecSig           string  `json:"exec_sig"`
	}
	out := make([]dec, len(rows))
	for i, r := range rows {
		out[i] = dec{
			ID: r.ID, Ts: r.Ts.Unix(), Action: r.Action,
			FromIndex: r.FromIndex, ToIndex: r.ToIndex,
			SuggestedFraction: r.SuggestedFraction, Confidence: r.Confidence,
			Rationale: r.Rationale, RiskAnalysis: r.RiskAnalysis,
			YieldAnalysis: r.YieldAnalysis, RiskLevel: r.RiskLevel, ExecSig: r.ExecSig,
		}
	}
	return c.JSON(fiber.Map{"decisions": out})
}

// GET /api/v1/history/rebalances?limit=20
func (h *Handler) historyRebalances(c *fiber.Ctx) error {
	if !h.requireStore(c) {
		return nil
	}
	limit := limitParam(c, 20)
	rows, err := h.store.RecentRebalances(limit)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	type reb struct {
		ID        int64   `json:"id"`
		Ts        int64   `json:"ts"`
		FromIndex int     `json:"from_index"`
		ToIndex   int     `json:"to_index"`
		Amount    uint64  `json:"amount"`
		Signature string  `json:"signature"`
		RiskLevel float64 `json:"risk_level"`
	}
	out := make([]reb, len(rows))
	for i, r := range rows {
		out[i] = reb{r.ID, r.Ts.Unix(), r.FromIndex, r.ToIndex, r.Amount, r.Signature, r.RiskLevel}
	}
	return c.JSON(fiber.Map{"rebalances": out})
}

// GET /api/v1/history/risk-events?limit=50
func (h *Handler) historyRiskEvents(c *fiber.Ctx) error {
	if !h.requireStore(c) {
		return nil
	}
	limit := limitParam(c, 50)
	rows, err := h.store.RecentRiskEvents(limit)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	type ev struct {
		ID        int64   `json:"id"`
		Ts        int64   `json:"ts"`
		RiskLevel float64 `json:"risk_level"`
		Deviation float64 `json:"deviation_pct"`
		Summary   string  `json:"summary"`
		Action    string  `json:"action"`
	}
	out := make([]ev, len(rows))
	for i, r := range rows {
		out[i] = ev{r.ID, r.Ts.Unix(), r.RiskLevel, r.Deviation, r.Summary, r.Action}
	}
	return c.JSON(fiber.Map{"events": out})
}

// GET /api/v1/history/stats
func (h *Handler) historyStats(c *fiber.Ctx) error {
	if !h.requireStore(c) {
		return nil
	}
	stats, err := h.store.GetStats()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	m := fiber.Map{
		"total_decisions":   stats.TotalDecisions,
		"total_rebalances":  stats.TotalRebalances,
		"total_risk_events": stats.TotalRiskEvents,
		"avg_risk_level":    stats.AvgRiskLevel,
	}
	if stats.LastDecisionTs != nil {
		m["last_decision_ts"] = stats.LastDecisionTs.Unix()
	}
	return c.JSON(m)
}

// ── SSE Real-time feed ─────────────────────────────────────────────────────

// GET /api/v1/stream — SSE endpoint, pushes FeedMessage JSON on every pipeline tick.
func (h *Handler) streamFeed(c *fiber.Ctx) error {
	if h.feedHub == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "hub not running")
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Access-Control-Allow-Origin", "*")

	ch := h.feedHub.Subscribe()

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer h.feedHub.Unsubscribe(ch)
		// Send initial ping
		fmt.Fprintf(w, "data: {\"ping\":true}\n\n")
		w.Flush()

		for data := range ch {
			fmt.Fprintf(w, "data: %s\n\n", data)
			if err := w.Flush(); err != nil {
				return // client disconnected
			}
		}
	})
	return nil
}

// ── Settings ───────────────────────────────────────────────────────────────

// GET /api/v1/settings
func (h *Handler) getSettings(c *fiber.Ctx) error {
	telegramEnabled := h.alerter != nil && h.alerter.Enabled()
	controlMode := deriveControlMode(h.cfg)
	yieldMode, yieldModeNote := liveYieldStatus(h.cfg)
	yieldReadiness := h.cfg.YieldExecutionReadiness()
	return c.JSON(fiber.Map{
		"alerts_enabled":          telegramEnabled,
		"circuit_breaker_enabled": h.cfg != nil && h.cfg.CircuitBreakerEnabled,
		"pipeline_running":        h.pipe != nil,
		"control_mode":            controlMode,
		"strategy_mode": func() uint8 {
			if h.cfg == nil {
				return 1
			}
			return h.cfg.StrategyMode
		}(),
		"strategy_name": func() string {
			if h.cfg == nil {
				return "BALANCED"
			}
			return strategyName(h.cfg.StrategyMode)
		}(),
		"auto_execute": func() bool {
			return h.cfg != nil && h.cfg.AutoExecute
		}(),
		"yield_enabled": func() bool {
			return h.cfg != nil && h.cfg.YieldEnabled
		}(),
		"yield_live_mode":             yieldMode,
		"yield_live_note":             yieldModeNote,
		"yield_mainnet_rpc":           yieldReadiness.MainnetRPC,
		"yield_ready_for_live":        yieldReadiness.ReadyForLive,
		"yield_missing_strategy_atas": yieldReadiness.MissingStrategyATAs,
		"yield_missing_kamino_vaults": yieldReadiness.MissingKaminoVaults,
		"yield_entry_risk": func() float64 {
			if h.cfg == nil {
				return 35
			}
			return h.cfg.YieldEntryRisk
		}(),
		"yield_exit_risk": func() float64 {
			if h.cfg == nil {
				return 55
			}
			return h.cfg.YieldExitRisk
		}(),
		"circuit_breaker_pause_pct": func() float64 {
			if h.cfg == nil {
				return 1.5
			}
			return h.cfg.CircuitBreakerPausePct
		}(),
		"alert_risk_threshold": func() float64 {
			if h.cfg == nil {
				return 80
			}
			return h.cfg.AlertRiskThreshold
		}(),
		"execution_mode": "record_only",
		"custody_model":  "program_owned_vault_accounts",
		"execution_note": executionUnavailableNote(),
		"hub_subscribers": func() int {
			if h.feedHub == nil {
				return 0
			}
			return h.feedHub.Subscribers()
		}(),
	})
}

// POST /api/v1/settings/control-mode
func (h *Handler) applyControlMode(c *fiber.Ctx) error {
	if h.cfg == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "runtime config not attached")
	}

	var req struct {
		Mode string `json:"mode"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	profile, ok := controlModeProfile(req.Mode)
	if !ok {
		return fiber.NewError(fiber.StatusBadRequest, "mode must be one of: MANUAL, GUARDED, BALANCED, YIELD_MAX")
	}

	h.cfg.StrategyMode = profile.StrategyMode
	h.cfg.AutoExecute = profile.AutoExecute
	h.cfg.YieldEnabled = profile.YieldEnabled
	h.cfg.YieldEntryRisk = profile.YieldEntryRisk
	h.cfg.YieldExitRisk = profile.YieldExitRisk
	h.cfg.CircuitBreakerPausePct = profile.CircuitBreakerPause
	h.cfg.AlertRiskThreshold = float64(profile.RiskThreshold)

	sig, err := h.executor.SendSetStrategy(c.Context(), onChainStrategyMode(profile.StrategyMode))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("set_strategy: %v", err))
	}
	thresholdSig, err := h.executor.SendUpdateThreshold(c.Context(), profile.RiskThreshold)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("update_threshold: %v", err))
	}

	return c.JSON(fiber.Map{
		"ok":                   true,
		"control_mode":         profile.Name,
		"strategy_mode":        profile.StrategyMode,
		"strategy_name":        strategyName(profile.StrategyMode),
		"auto_execute":         profile.AutoExecute,
		"yield_enabled":        profile.YieldEnabled,
		"yield_entry_risk":     profile.YieldEntryRisk,
		"yield_exit_risk":      profile.YieldExitRisk,
		"circuit_breaker_pct":  profile.CircuitBreakerPause,
		"risk_threshold":       profile.RiskThreshold,
		"description":          profile.Description,
		"set_strategy_sig":     sig,
		"update_threshold_sig": thresholdSig,
	})
}

// POST /api/v1/settings/autopilot
func (h *Handler) applyAutopilot(c *fiber.Ctx) error {
	if h.cfg == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "runtime config not attached")
	}

	var req struct {
		StrategyMode      int     `json:"strategy_mode"`
		RiskThreshold     uint64  `json:"risk_threshold"`
		YieldEntryRisk    float64 `json:"yield_entry_risk"`
		YieldExitRisk     float64 `json:"yield_exit_risk"`
		CircuitBreakerPct float64 `json:"circuit_breaker_pct"`
		AutoExecute       *bool   `json:"auto_execute,omitempty"`
		YieldEnabled      *bool   `json:"yield_enabled,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if req.StrategyMode < 0 || req.StrategyMode > 2 {
		return fiber.NewError(fiber.StatusBadRequest, "strategy_mode must be 0, 1, or 2")
	}
	if req.RiskThreshold == 0 || req.RiskThreshold > 100 {
		return fiber.NewError(fiber.StatusBadRequest, "risk_threshold must be 1..100")
	}

	h.cfg.StrategyMode = uint8(req.StrategyMode)
	h.cfg.AlertRiskThreshold = float64(req.RiskThreshold)
	h.cfg.YieldEntryRisk = req.YieldEntryRisk
	h.cfg.YieldExitRisk = req.YieldExitRisk
	h.cfg.CircuitBreakerPausePct = req.CircuitBreakerPct

	if req.AutoExecute != nil {
		h.cfg.AutoExecute = *req.AutoExecute
	} else {
		h.cfg.AutoExecute = req.StrategyMode != 0
	}
	if req.YieldEnabled != nil {
		h.cfg.YieldEnabled = *req.YieldEnabled
	} else {
		h.cfg.YieldEnabled = req.StrategyMode == 2
	}

	sig, err := h.executor.SendSetStrategy(c.Context(), onChainStrategyMode(uint8(req.StrategyMode)))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("set_strategy: %v", err))
	}
	thresholdSig, err := h.executor.SendUpdateThreshold(c.Context(), req.RiskThreshold)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("update_threshold: %v", err))
	}

	return c.JSON(fiber.Map{
		"ok":                   true,
		"control_mode":         deriveControlMode(h.cfg),
		"strategy_mode":        h.cfg.StrategyMode,
		"strategy_name":        strategyName(h.cfg.StrategyMode),
		"auto_execute":         h.cfg.AutoExecute,
		"yield_enabled":        h.cfg.YieldEnabled,
		"yield_entry_risk":     h.cfg.YieldEntryRisk,
		"yield_exit_risk":      h.cfg.YieldExitRisk,
		"circuit_breaker_pct":  h.cfg.CircuitBreakerPausePct,
		"risk_threshold":       req.RiskThreshold,
		"set_strategy_sig":     sig,
		"update_threshold_sig": thresholdSig,
	})
}

// POST /api/v1/settings/telegram
// body: {"bot_token": "...", "chat_id": "..."}
func (h *Handler) setTelegram(c *fiber.Ctx) error {
	var req struct {
		BotToken string `json:"bot_token"`
		ChatID   string `json:"chat_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if h.alerter == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "alerter not initialized")
	}
	h.alerter.UpdateTelegram(req.BotToken, req.ChatID)
	return c.JSON(fiber.Map{"ok": true, "message": "Telegram credentials updated"})
}

// POST /api/v1/settings/discord
// body: {"webhook_url": "..."}
func (h *Handler) setDiscord(c *fiber.Ctx) error {
	var req struct {
		WebhookURL string `json:"webhook_url"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if h.alerter == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "alerter not initialized")
	}
	h.alerter.UpdateDiscord(req.WebhookURL)
	return c.JSON(fiber.Map{"ok": true, "message": "Discord webhook updated"})
}

// POST /api/v1/settings/test-alert
func (h *Handler) testAlert(c *fiber.Ctx) error {
	if h.alerter == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "alerter not initialized")
	}
	if !h.alerter.Enabled() {
		return fiber.NewError(fiber.StatusBadRequest, "no alert channels configured (set Telegram or Discord first)")
	}
	h.alerter.SendForce(alerts.LevelInfo, "✅ Test alert from StableGuard — alerts are working!")
	return c.JSON(fiber.Map{"ok": true, "message": "Test alert sent"})
}

// ── AI Chat ────────────────────────────────────────────────────────────────

// POST /api/v1/chat — conversational AI with full portfolio context
func (h *Handler) chat(c *fiber.Ctx) error {
	var req ai.ChatRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if req.Message == "" {
		return fiber.NewError(fiber.StatusBadRequest, "message required")
	}

	// Build portfolio context
	pc := ai.PortfolioContext{}

	if h.pipe != nil {
		score, snap, _ := h.pipe.LastState()
		pc.RiskLevel = score.RiskLevel
		pc.RiskSummary = score.Summary
		pc.Action = string(score.Action)
		if snap != nil {
			pc.MaxDeviation = snap.MaxDeviation()
			pc.Prices = make(map[string]float64)
			for sym, pd := range snap.All {
				pc.Prices[sym] = pd.Price
			}
		}
	}

	if h.store != nil {
		decisions, _ := h.store.RecentDecisions(5)
		pc.LastDecisions = decisions
		stats, _ := h.store.GetStats()
		pc.TotalDecisions = stats.TotalDecisions
		pc.TotalRebalances = stats.TotalRebalances

		if pos, err := h.store.ActiveYieldPosition(); err == nil && pos != nil {
			pc.YieldProtocol = pos.Protocol
			pc.YieldAPY = pos.EntryAPY
			// Live earned calculation
			elapsed := time.Since(pos.DepositedAt).Seconds()
			pc.YieldEarned = pos.Amount * (pos.EntryAPY / 100.0) / (365.25 * 24 * 3600) * elapsed
		}
	}

	var agents *ai.MultiAgentSystem
	if h.pipe != nil {
		agents = h.pipe.Agents()
	}
	if agents == nil {
		return c.JSON(ai.ChatResponse{Reply: "AI agents are offline — start the backend pipeline."})
	}

	resp, err := agents.Chat(c.Context(), req, pc)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(resp)
}

// POST /api/v1/intent — parse natural language goal into vault config
func (h *Handler) parseIntent(c *fiber.Ctx) error {
	var req struct {
		Intent string `json:"intent"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if req.Intent == "" {
		return fiber.NewError(fiber.StatusBadRequest, "intent required")
	}

	var agents *ai.MultiAgentSystem
	if h.pipe != nil {
		agents = h.pipe.Agents()
	}
	if agents == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "agents not running")
	}

	cfg, err := agents.ParseIntent(c.Context(), req.Intent)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(cfg)
}

// ── Goals ──────────────────────────────────────────────────────────────────

// GET /api/v1/goals
func (h *Handler) listGoals(c *fiber.Ctx) error {
	if h.store == nil {
		return c.JSON(fiber.Map{"goals": []interface{}{}})
	}
	goals, err := h.store.ActiveGoals()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	// Enrich with live progress for monthly_yield type
	totalEarned := h.store.TotalYieldEarned()
	for i, g := range goals {
		if g.GoalType == "monthly_yield" {
			// Approximate this month's earned yield
			goals[i].Progress = totalEarned
		}
	}

	if goals == nil {
		goals = []store.Goal{}
	}
	return c.JSON(fiber.Map{"goals": goals, "total_earned": totalEarned})
}

// POST /api/v1/goals
func (h *Handler) createGoal(c *fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "store not available")
	}
	var req struct {
		Name     string  `json:"name"`
		GoalType string  `json:"goal_type"`
		Target   float64 `json:"target"`
		Deadline *int64  `json:"deadline,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if req.Name == "" || req.Target <= 0 {
		return fiber.NewError(fiber.StatusBadRequest, "name and target required")
	}
	if req.GoalType == "" {
		req.GoalType = "monthly_yield"
	}
	id, err := h.store.CreateGoal(req.Name, req.GoalType, req.Target, req.Deadline)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"id": id, "ok": true})
}

// PATCH /api/v1/goals/:id/progress
func (h *Handler) updateGoalProgress(c *fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "store not available")
	}
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	var req struct {
		Progress float64 `json:"progress"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if err := h.store.UpdateGoalProgress(id, req.Progress); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

// DELETE /api/v1/goals/:id
func (h *Handler) deleteGoal(c *fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "store not available")
	}
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	if err := h.store.DeleteGoal(id); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

// ── Auth ───────────────────────────────────────────────────────────────────

// POST /api/v1/auth/register
func (h *Handler) authRegister(c *fiber.Ctx) error {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		OrgName  string `json:"org_name"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if body.Email == "" || body.Password == "" {
		return fiber.NewError(fiber.StatusBadRequest, "email and password required")
	}
	if len(body.Password) < 8 {
		return fiber.NewError(fiber.StatusBadRequest, "password must be at least 8 characters")
	}

	if h.store != nil {
		if _, err := h.store.FindUserByEmail(body.Email); err == nil {
			return fiber.NewError(fiber.StatusConflict, "email already registered")
		}
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not hash password")
	}

	var userID int64 = 1
	if h.store != nil {
		userID, err = h.store.CreateUser(body.Email, hash, body.OrgName)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
	}

	// Generate and store OTP
	otp := fmt.Sprintf("%06d", rand.Intn(1000000))
	if h.store != nil {
		_ = h.store.SaveOTP(body.Email, otp)
	}

	// In production, send via email service. For now, return in response (dev mode).
	token, _ := auth.GenerateToken(userID, body.Email)
	resp := fiber.Map{
		"token":   token,
		"user_id": userID,
		"email":   body.Email,
	}
	if authDebugResponsesEnabled() {
		resp["otp"] = otp
		resp["otp_debug"] = true
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// POST /api/v1/auth/login
func (h *Handler) authLogin(c *fiber.Ctx) error {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}

	if h.store == nil {
		// Dev fallback: accept any credentials
		token, _ := auth.GenerateToken(1, body.Email)
		return c.JSON(fiber.Map{"token": token, "email": body.Email})
	}

	user, err := h.store.FindUserByEmail(body.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid email or password")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	if !auth.CheckPassword(body.Password, user.PasswordHash) {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid email or password")
	}

	token, _ := auth.GenerateToken(user.ID, user.Email)
	return c.JSON(fiber.Map{
		"token":    token,
		"user_id":  user.ID,
		"email":    user.Email,
		"org_name": user.OrgName,
	})
}

// POST /api/v1/auth/send-otp — (re)send OTP to email
func (h *Handler) authSendOTP(c *fiber.Ctx) error {
	var body struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&body); err != nil || body.Email == "" {
		return fiber.NewError(fiber.StatusBadRequest, "email required")
	}
	otp := fmt.Sprintf("%06d", rand.Intn(1000000))
	if h.store != nil {
		_ = h.store.SaveOTP(body.Email, otp)
	}
	resp := fiber.Map{"ok": true}
	if authDebugResponsesEnabled() {
		resp["otp"] = otp
		resp["otp_debug"] = true
	}
	return c.JSON(resp)
}

// POST /api/v1/auth/verify-otp
func (h *Handler) authVerifyOTP(c *fiber.Ctx) error {
	var body struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}

	if h.store == nil || h.store.VerifyOTP(body.Email, body.Code) {
		return c.JSON(fiber.Map{"ok": true})
	}
	return fiber.NewError(fiber.StatusUnauthorized, "invalid or expired code")
}

// ── Jupiter quote ──────────────────────────────────────────────────────────

// GET /api/v1/quote?inputMint=...&outputMint=...&amount=...
func (h *Handler) jupiterQuote(c *fiber.Ctx) error {
	inputMint := c.Query("inputMint")
	outputMint := c.Query("outputMint")
	amountStr := c.Query("amount")
	if inputMint == "" || outputMint == "" || amountStr == "" {
		return fiber.NewError(fiber.StatusBadRequest, "inputMint, outputMint, amount required")
	}
	amount, err := strconv.ParseUint(amountStr, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid amount")
	}

	quote, err := jupiter.GetQuote(jupiter.QuoteRequest{
		InputMint:  inputMint,
		OutputMint: outputMint,
		Amount:     amount,
	})
	if err != nil {
		// Jupiter is mainnet-only; return graceful error for devnet usage
		return c.JSON(fiber.Map{
			"error":   err.Error(),
			"devnet":  true,
			"message": "Jupiter quotes are mainnet-only. This is expected on devnet.",
		})
	}
	return c.JSON(quote)
}

// ── ML Prediction ──────────────────────────────────────────────────────────

// GET /api/v1/prediction/depeg?symbol=USDC&limit=200
// Fetches historical prices from SQLite and calls the Python Chronos service.
func (h *Handler) depegPrediction(c *fiber.Ctx) error {
	symbol := c.Query("symbol", "USDC")
	limit, _ := strconv.Atoi(c.Query("limit", "200"))
	if limit < 10 {
		limit = 10
	}

	// Check if ML service is alive
	if !prediction.Healthy() {
		return c.JSON(fiber.Map{
			"available": false,
			"message":   "ML service offline. Run: cd ml-service && python main.py",
		})
	}

	// Get historical price points (RecentPrices returns newest-first, reverse it)
	var prices []float64
	if h.store != nil {
		rows, err := h.store.RecentPrices(symbol, limit)
		if err == nil {
			// rows are newest-first; reverse so oldest→newest for ML
			for i := len(rows) - 1; i >= 0; i-- {
				prices = append(prices, rows[i].Price)
			}
		}
	}

	// Need at least 10 points
	if len(prices) < 10 {
		return c.JSON(fiber.Map{
			"available": false,
			"message":   "Not enough historical data yet — pipeline needs to run for a few minutes",
		})
	}

	result, err := prediction.Predict(symbol, prices)
	if err != nil {
		return c.JSON(fiber.Map{
			"available": false,
			"message":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"available":  true,
		"symbol":     symbol,
		"history":    prices[max(0, len(prices)-50):], // last 50 points for chart
		"prediction": result,
	})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ── On-chain whale intelligence ────────────────────────────────────────────

// GET /api/v1/onchain/whales
func (h *Handler) whaleIntelligence(c *fiber.Ctx) error {
	if h.whaleAgg == nil {
		return c.JSON(fiber.Map{
			"score":   0,
			"alerts":  []interface{}{},
			"summary": "Whale monitor not enabled",
		})
	}

	sig := h.whaleAgg.Fetch()
	return c.JSON(fiber.Map{
		"score":      sig.Score,
		"alerts":     sig.Alerts,
		"summary":    sig.Summary,
		"updated_at": sig.UpdatedAt.Unix(),
	})
}

// GET /api/v1/onchain/slippage — Jupiter liquidity depth analysis
func (h *Handler) slippageAnalysis(c *fiber.Ctx) error {
	if h.slippageAnal == nil {
		return c.JSON(fiber.Map{
			"liquidity_score": 100,
			"impact_10k":      0,
			"impact_100k":     0,
			"impact_1m":       0,
			"drain_detected":  false,
			"note":            "slippage analyzer not enabled",
		})
	}

	inputMint := c.Query("input", onchain.MintUSDC)
	outputMint := c.Query("output", onchain.MintUSDT)

	m := h.slippageAnal.Measure(inputMint, outputMint)
	window := h.slippageAnal.Window(inputMint, outputMint)

	return c.JSON(fiber.Map{
		"current":    m,
		"window":     window,
		"note":       "Jupiter mainnet data — devnet returns impact=0 (score=100)",
		"updated_at": time.Now().Unix(),
	})
}

// POST /api/v1/auth/wallet-login — Web3 wallet signature authentication
func (h *Handler) walletLogin(c *fiber.Ctx) error {
	var body struct {
		Address   string `json:"address"`
		Message   string `json:"message"`
		Signature string `json:"signature"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}
	if body.Address == "" || body.Message == "" || body.Signature == "" {
		return c.Status(400).JSON(fiber.Map{"error": "address, message, and signature required"})
	}

	if err := auth.VerifyWalletSignature(body.Address, body.Message, body.Signature); err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "signature verification failed: " + err.Error()})
	}

	token, err := auth.GenerateWalletToken(body.Address)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to generate token"})
	}

	return c.JSON(fiber.Map{
		"token":  token,
		"wallet": body.Address,
	})
}

// POST /api/v1/delegate-agent — set delegated agent on-chain
func (h *Handler) delegateAgent(c *fiber.Ctx) error {
	var body struct {
		AgentPubkey string `json:"agent_pubkey"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}
	if body.AgentPubkey == "" {
		return c.Status(400).JSON(fiber.Map{"error": "agent_pubkey required"})
	}

	agentPK, err := solanaexec.ParsePublicKey(body.AgentPubkey)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid agent_pubkey: " + err.Error()})
	}

	sig, err := h.executor.SendDelegateAgent(c.Context(), agentPK)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"sig":          sig,
		"agent_pubkey": body.AgentPubkey,
	})
}

// POST /api/v1/vault/init-lut — create address lookup table for vault accounts
func (h *Handler) initVaultLUT(c *fiber.Ctx) error {
	lutAddr, sig, err := h.executor.InitVaultLUT(c.Context())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"lut_address": lutAddr.String(),
		"sig":         sig,
	})
}

// GET /api/v1/vault/lut — return configured LUT address
func (h *Handler) vaultLUT(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"lut_address": c.Locals("lut_address"),
		"note":        "Set VAULT_LUT_ADDRESS env var after running /vault/init-lut",
	})
}
