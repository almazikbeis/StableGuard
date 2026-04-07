// Package api exposes the StableGuard REST API.
package api

import (
	"bufio"
	"context"
	cryptorand "crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"stableguard-backend/ai"
	"stableguard-backend/alerts"
	"stableguard-backend/auth"
	"stableguard-backend/config"
	"stableguard-backend/execution"
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

	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gofiber/fiber/v2"
)

func executionUnavailableNote() string {
	return "Market execution is unavailable under the current custody architecture: vault token accounts are program-owned, so external routers like Jupiter cannot spend them directly. Use mode=record only for an explicit accounting-only allocation shift."
}

func executionScaffoldNote() string {
	return "Trusted execution custody can now isolate source assets outside the program-owned vault. StableGuard can stage funds into dedicated execution custody, accept an operator-supplied external swap transaction, and settle the output back into treasury."
}

func authDebugResponsesEnabled() bool {
	switch os.Getenv("STABLEGUARD_DEV_AUTH_DEBUG") {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func authSubject(c *fiber.Ctx) (string, string, error) {
	authType, _ := c.Locals("auth_type").(string)
	switch authType {
	case "wallet":
		wallet, _ := c.Locals("auth_wallet").(string)
		if strings.TrimSpace(wallet) == "" {
			return "", "", fiber.NewError(fiber.StatusUnauthorized, "wallet subject missing")
		}
		return authType, wallet, nil
	case "user":
		email, _ := c.Locals("auth_email").(string)
		if strings.TrimSpace(email) == "" {
			return "", "", fiber.NewError(fiber.StatusUnauthorized, "user subject missing")
		}
		return authType, email, nil
	default:
		return "", "", fiber.NewError(fiber.StatusUnauthorized, "auth subject missing")
	}
}

func generateTelegramLinkToken() (string, error) {
	var raw [12]byte
	if _, err := cryptorand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
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
	ExecutionApproval   string
	YieldEntryRisk      float64
	YieldExitRisk       float64
	CircuitBreakerPause float64
	RiskThreshold       uint64
	Description         string
}

func modeReadiness(cfg *config.Config, mode string) fiber.Map {
	profile, ok := controlModeProfile(mode)
	if !ok {
		return fiber.Map{
			"mode":     mode,
			"ready":    false,
			"risk":     "unknown",
			"summary":  "Unknown control mode.",
			"blockers": []string{"mode is not defined"},
		}
	}

	simulated := &config.Config{}
	if cfg != nil {
		copyCfg := *cfg
		simulated = &copyCfg
	}
	applyRuntimeModeProfile(simulated, profile)

	execReadiness := simulated.ExecutionReadiness()
	yieldReadiness := simulated.YieldExecutionReadiness()
	growthReadiness := simulated.GrowthReadiness()

	ready := true
	risk := "low"
	blockers := make([]string, 0, 4)
	summary := profile.Description

	switch mode {
	case "MANUAL":
		summary = "Advisory-only mode. Safe to enable as soon as the backend and wallet are online."
	case "GUARDED":
		if !execReadiness.ReadyForAutoSwap {
			ready = false
			risk = "medium"
			blockers = append(blockers, execReadiness.Note)
		}
		summary = "Protective automation only. Requires a live market execution path to act autonomously."
	case "BALANCED":
		if !execReadiness.ReadyForAutoSwap {
			ready = false
			risk = "medium"
			blockers = append(blockers, execReadiness.Note)
		}
		summary = "Protective automation with moderate risk tolerance. Yield remains gated off."
	case "YIELD_MAX":
		if !execReadiness.ReadyForAutoSwap {
			ready = false
			blockers = append(blockers, execReadiness.Note)
		}
		if !yieldReadiness.ReadyForLive {
			ready = false
			blockers = append(blockers, yieldReadiness.Note)
		}
		if simulated.GrowthSleeveEnabled && !growthReadiness.ReadyForLive {
			ready = false
			blockers = append(blockers, growthReadiness.Note)
		}
		risk = "high"
		summary = "Full automation for execution and yield. Only safe to enable when execution and yield readiness are both green."
	}

	if len(blockers) == 0 {
		blockers = []string{}
	}

	return fiber.Map{
		"mode":     mode,
		"ready":    ready,
		"risk":     risk,
		"summary":  summary,
		"blockers": blockers,
	}
}

func controlModeProfile(mode string) (controlProfile, bool) {
	switch mode {
	case "MANUAL":
		return controlProfile{
			Name:                "MANUAL",
			StrategyMode:        0,
			AutoExecute:         false,
			YieldEnabled:        false,
			ExecutionApproval:   "manual",
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
			ExecutionApproval:   "auto",
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
			ExecutionApproval:   "auto",
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
			ExecutionApproval:   "auto",
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

func applyRuntimeModeProfile(cfg *config.Config, profile controlProfile) {
	if cfg == nil {
		return
	}
	cfg.StrategyMode = profile.StrategyMode
	cfg.AutoExecute = profile.AutoExecute
	cfg.YieldEnabled = profile.YieldEnabled
	cfg.YieldEntryRisk = profile.YieldEntryRisk
	cfg.YieldExitRisk = profile.YieldExitRisk
	cfg.CircuitBreakerPausePct = profile.CircuitBreakerPause
	cfg.AlertRiskThreshold = float64(profile.RiskThreshold)
	cfg.ExecutionApprovalMode = profile.ExecutionApproval

	// Fail closed: non-yield modes must not keep a live growth sleeve armed.
	if profile.Name != "YIELD_MAX" {
		cfg.GrowthSleeveLiveExecution = false
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

func tokenSymbolByIndex(idx uint8) (string, bool) {
	feed, ok := pyth.FeedBySlot(int(idx))
	if !ok {
		return "", false
	}
	return feed.Symbol, true
}

func tokenFeedBySymbol(symbol string) (pyth.TokenFeed, bool) {
	for _, feed := range pyth.ActiveFeeds {
		if feed.Symbol == symbol {
			return feed, true
		}
	}
	return pyth.TokenFeed{}, false
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

func (h *Handler) currentOperatorSettings() store.OperatorSettings {
	settings := store.OperatorSettings{}
	if h.cfg != nil {
		settings.StrategyMode = h.cfg.StrategyMode
		settings.AutoExecute = h.cfg.AutoExecute
		settings.YieldEnabled = h.cfg.YieldEnabled
		settings.YieldEntryRisk = h.cfg.YieldEntryRisk
		settings.YieldExitRisk = h.cfg.YieldExitRisk
		settings.CircuitBreakerPausePct = h.cfg.CircuitBreakerPausePct
		settings.AlertRiskThreshold = h.cfg.AlertRiskThreshold
		settings.ExecutionApprovalMode = h.cfg.ExecutionApprovalMode
		settings.AIAgentModel = h.cfg.AIAgentModel
		settings.AIDecisionProfile = h.cfg.AIDecisionProfile
		settings.GrowthSleeveEnabled = h.cfg.GrowthSleeveEnabled
		settings.GrowthSleeveBudgetPct = h.cfg.GrowthSleeveBudgetPct
		settings.GrowthSleeveMaxAssetPct = h.cfg.GrowthSleeveMaxAssetPct
		settings.GrowthSleeveAllowedAssets = strings.Join(h.cfg.GrowthSleeveAllowedAssets, ",")
		settings.GrowthSleeveLiveExecution = h.cfg.GrowthSleeveLiveExecution
	}
	if h.alerter != nil {
		snapshot := h.alerter.Snapshot()
		settings.TelegramBotToken = snapshot.TelegramToken
		settings.TelegramChatID = snapshot.TelegramChatID
		settings.DiscordWebhookURL = snapshot.DiscordWebhook
	}
	return settings
}

func (h *Handler) persistOperatorSettings() error {
	if h.store == nil {
		return nil
	}
	return h.store.SaveOperatorSettings(h.currentOperatorSettings())
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
	operatorAuth := operatorRequired(h.cfg, h.executor.WalletAddress().String())
	userAuth := userRequired()

	v1.Get("/health", h.health)
	v1.Get("/prices", h.prices)
	v1.Get("/tokens", h.tokensList)              // all monitored tokens + live prices
	v1.Get("/risk", h.riskScore)                 // v1: simple deviation scorer (unchanged)
	v1.Get("/risk/v2", h.riskScoreV2)            // windowed scorer with trend/velocity/volatility
	v1.Get("/pipeline/status", h.pipelineStatus) // last AI decision + score
	v1.Post("/decide", h.decide)
	v1.Get("/vault", h.vaultState)

	// Core on-chain actions
	v1.Post("/rebalance", operatorAuth, h.rebalance)
	v1.Post("/strategy", operatorAuth, h.setStrategy)
	v1.Post("/send", operatorAuth, h.sendPayment)
	v1.Post("/threshold", operatorAuth, h.updateThreshold)
	v1.Post("/emergency", operatorAuth, h.emergencyWithdraw)
	v1.Post("/register-token", operatorAuth, h.registerToken)

	// History endpoints (require store)
	v1.Get("/history/prices", h.historyPrices)
	v1.Get("/history/decisions", h.historyDecisions)
	v1.Get("/history/rebalances", h.historyRebalances)
	v1.Get("/history/execution-jobs", operatorAuth, h.historyExecutionJobs)
	v1.Get("/history/risk-events", h.historyRiskEvents)
	v1.Get("/history/stats", h.historyStats)

	// Real-time SSE feed
	v1.Get("/stream", h.streamFeed)

	// Yield optimizer
	v1.Get("/yield/opportunities", h.yieldOpportunities)
	v1.Get("/yield/position", h.yieldPosition)
	v1.Get("/yield/history", h.yieldHistory)

	// Settings
	v1.Get("/settings", operatorAuth, h.getSettings)
	v1.Post("/settings/control-mode", operatorAuth, h.applyControlMode)
	v1.Post("/settings/autopilot", operatorAuth, h.applyAutopilot)
	v1.Post("/settings/growth-sleeve", operatorAuth, h.applyGrowthSleeve)
	v1.Post("/settings/telegram", operatorAuth, h.setTelegram)
	v1.Post("/settings/discord", operatorAuth, h.setDiscord)
	v1.Post("/settings/test-alert", operatorAuth, h.testAlert)

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
	v1.Get("/notifications/telegram/status", userAuth, h.telegramNotificationStatus)
	v1.Get("/notifications/telegram/link", userAuth, h.telegramNotificationLink)
	v1.Post("/notifications/telegram/register", userAuth, h.registerTelegramNotification)
	v1.Post("/notifications/test-alert", userAuth, h.userNotificationTestAlert)

	// Delegate agent on-chain
	v1.Post("/delegate-agent", operatorAuth, h.delegateAgent)

	// Real SPL vault deposit / withdraw (requires on-chain token accounts)
	v1.Post("/vault/deposit", operatorAuth, h.vaultDeposit)
	v1.Post("/vault/withdraw", operatorAuth, h.vaultWithdraw)

	// Vault LUT management
	v1.Post("/vault/init-lut", operatorAuth, h.initVaultLUT)
	v1.Get("/vault/lut", operatorAuth, h.vaultLUT)

	// External execution job lifecycle
	v1.Get("/execution/jobs/:id", operatorAuth, h.executionJob)
	v1.Post("/execution/jobs/:id/build-swap", operatorAuth, h.buildExecutionSwap)
	v1.Post("/execution/jobs/:id/execute-swap", operatorAuth, h.executeExecutionSwap)
	v1.Post("/execution/jobs/:id/submit-swap", operatorAuth, h.submitExecutionSwap)
	v1.Post("/execution/jobs/:id/settle", operatorAuth, h.settleExecutionJob)

	// Demo / hackathon — live autonomous loop simulation (no auth required)
	v1.Post("/demo/simulate-event", h.simulateEvent)
	v1.Post("/demo/simulate-depeg", h.simulateDepeg)
	v1.Post("/demo/simulate-crash", h.simulateCrash)
	v1.Get("/demo/latest-proof", h.latestOnChainProof)
	v1.Post("/demo/init-vault", h.demoInitVault)
	v1.Post("/demo/full-rebalance", h.demoFullRebalance)
	v1.Get("/demo/token", h.demoOperatorToken)
	v1.Post("/demo/register-token", h.demoRegisterToken)
	v1.Post("/demo/deposit", h.demoDeposit)
	v1.Post("/demo/set-balances", h.demoSetBalances)
	v1.Post("/demo/reset-count", h.demoResetDecisionCount)
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
	executionReadiness := config.ExecutionReadiness{}
	yieldMode, yieldNote := "disabled", "runtime config not attached"
	growthReadiness := config.GrowthReadiness{}
	controlMode := "UNKNOWN"
	aiConfigured := false
	aiMode := "mock_or_unconfigured"
	rpcURL := ""
	mainnetRPC := false

	if h.cfg != nil {
		executionReadiness = h.cfg.ExecutionReadiness()
		yieldMode, yieldNote = liveYieldStatus(h.cfg)
		growthReadiness = h.cfg.GrowthReadiness()
		controlMode = deriveControlMode(h.cfg)
		aiConfigured = strings.TrimSpace(h.cfg.AnthropicAPIKey) != ""
		if aiConfigured {
			aiMode = "live"
		}
		rpcURL = h.cfg.SolanaRPCURL
		mainnetRPC = config.IsMainnetRPC(h.cfg.SolanaRPCURL)
	}

	assets := make([]fiber.Map, 0, len(pyth.ActiveFeeds))
	for _, feed := range pyth.ActiveFeeds {
		assets = append(assets, fiber.Map{
			"slot":       feed.VaultSlot,
			"symbol":     feed.Symbol,
			"asset_type": feed.AssetType,
		})
	}

	return c.JSON(fiber.Map{
		"status":           "ok",
		"service":          "stableguard-backend",
		"pipeline_running": h.pipe != nil,
		"store_attached":   h.store != nil,
		"control_mode":     controlMode,
		"ai_configured":    aiConfigured,
		"ai_mode":          aiMode,
		"solana_rpc_url":   rpcURL,
		"mainnet_rpc":      mainnetRPC,
		"tracked_assets":   assets,
		"execution": fiber.Map{
			"mode":                     executionReadiness.Mode,
			"ready_for_staging":        executionReadiness.ReadyForStaging,
			"ready_for_auto_swap":      executionReadiness.ReadyForAutoSwap,
			"approval_mode":            executionReadiness.ApprovalMode,
			"auto_execution_enabled":   executionReadiness.AutoExecutionEnabled,
			"missing_custody_accounts": executionReadiness.MissingCustodyAccounts,
			"note":                     executionReadiness.Note,
		},
		"yield": fiber.Map{
			"mode": yieldMode,
			"note": yieldNote,
		},
		"growth_sleeve": fiber.Map{
			"mode":              growthReadiness.Mode,
			"budget_pct":        growthReadiness.BudgetPct,
			"max_asset_pct":     growthReadiness.MaxAssetPct,
			"allowed_assets":    growthReadiness.AllowedAssets,
			"ready_for_live":    growthReadiness.ReadyForLive,
			"live_execution":    growthReadiness.LiveExecution,
			"requires_operator": growthReadiness.RequiresOperator,
			"note":              growthReadiness.Note,
		},
	})
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

// POST /api/v1/rebalance — record rebalance intent or start market execution
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
			"kind":       "rebalance_intent",
			"signature":  sig,
			"from_index": req.FromIndex,
			"to_index":   req.ToIndex,
			"amount":     req.Amount,
			"message":    "This records a rebalance intent on-chain. Vault balances change only when real SPL transfers settle.",
			"explorer":   fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", sig),
		})
	case "market":
		if h.store == nil {
			return fiber.NewError(fiber.StatusServiceUnavailable, "store not available")
		}
		hasActive, err := h.store.HasActiveExecutionJob()
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("execution job status: %v", err))
		}
		if hasActive {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"mode":    "market",
				"kind":    "execution_job_active",
				"message": "An execution job is already active. Settle the staged funds back into treasury before starting another market rebalance.",
			})
		}

		executionReadiness := h.cfg.ExecutionReadiness()
		fromSymbol, okFrom := tokenSymbolByIndex(req.FromIndex)
		toSymbol, okTo := tokenSymbolByIndex(req.ToIndex)
		if !okFrom || !okTo {
			return fiber.NewError(fiber.StatusBadRequest, "token index is not mapped to an active feed")
		}

		execSvc := h.executionService()
		preview := h.quotePreview(req.FromIndex, req.ToIndex, req.Amount)
		custodyAccount := execSvc.CustodyAccount(fromSymbol)
		resp := fiber.Map{
			"mode":            "market",
			"execution_mode":  executionReadiness.Mode,
			"from_index":      req.FromIndex,
			"to_index":        req.ToIndex,
			"from_symbol":     fromSymbol,
			"to_symbol":       toSymbol,
			"amount":          req.Amount,
			"quote_preview":   preview,
			"custody_account": custodyAccount,
		}

		if !executionReadiness.ReadyForStaging || custodyAccount == "" {
			resp["kind"] = "execution_unavailable"
			resp["message"] = executionReadiness.Note
			resp["missing_custody_accounts"] = executionReadiness.MissingCustodyAccounts
			return c.Status(fiber.StatusConflict).JSON(resp)
		}

		sig, err := h.executor.SendPayment(c.Context(), req.Amount, custodyAccount, req.FromIndex)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("stage_execution_custody: %v", err))
		}

		note := fmt.Sprintf("Staged %s into trusted execution custody. Await external swap execution and treasury settlement into %s.", fromSymbol, toSymbol)
		job := store.ExecutionJobRow{
			FromIndex:      int(req.FromIndex),
			ToIndex:        int(req.ToIndex),
			Amount:         req.Amount,
			Stage:          "custody_staged",
			FundingSig:     sig,
			SourceSymbol:   fromSymbol,
			TargetSymbol:   toSymbol,
			CustodyAccount: custodyAccount,
			Note:           note,
		}
		if preview != nil {
			if outAmount, ok := preview["out_amount"].(string); ok {
				job.QuoteOutAmount = outAmount
			}
			if impact, ok := preview["price_impact_pct"].(string); ok {
				job.PriceImpactPct = impact
			}
		}
		jobID, err := h.store.SaveExecutionJob(job)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("save execution job: %v", err))
		}

		resp["kind"] = "custody_staged"
		resp["job_id"] = jobID
		resp["signature"] = sig
		resp["message"] = note
		resp["next_actions"] = []string{
			"POST /api/v1/execution/jobs/:id/submit-swap with an operator-supplied swap transaction",
			"POST /api/v1/execution/jobs/:id/settle to deposit the output token back into treasury",
		}
		resp["explorer"] = fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", sig)
		return c.Status(fiber.StatusAccepted).JSON(resp)
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
	if h.alerter != nil {
		if symbol, ok := tokenSymbolByIndex(req.TokenIndex); ok {
			h.alerter.Send("treasury_payment_sent", alerts.LevelInfo,
				fmt.Sprintf("💸 Treasury payment sent\nAsset: *%s*\nAmount: *%d* base units\nRecipient: `%s`\nTx: `%s`",
					symbol, req.Amount, req.Recipient, sig))
		}
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
		"stable_risk":        s.StableRisk,
		"volatile_risk":      s.VolatileRisk,
		"volatile_prices":    s.VolatilePrices,
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
			"risk_level":      score.RiskLevel,
			"deviation_pct":   score.Deviation,
			"trend":           score.Trend,
			"velocity":        score.Velocity,
			"volatility":      score.Volatility,
			"stable_risk":     score.StableRisk,
			"volatile_risk":   score.VolatileRisk,
			"volatile_prices": score.VolatilePrices,
			"action":          score.Action,
			"summary":         score.Summary,
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
	fromFeed, okFrom := pyth.FeedBySlot(int(fromIndex))
	toFeed, okTo := pyth.FeedBySlot(int(toIndex))
	if !okFrom || !okTo {
		return nil
	}

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

func (h *Handler) executionService() *execution.Service {
	return execution.New(h.executor, h.cfg, h.store)
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
		AssetType    string  `json:"asset_type"`
		Price        float64 `json:"price"`
		Confidence   float64 `json:"confidence"`
		DeviationPct float64 `json:"deviation_pct"` // vs USDC (stable) or % change (volatile)
	}

	tokens := make([]tokenInfo, 0, len(pyth.ActiveFeeds))
	for _, f := range pyth.ActiveFeeds {
		pd := snap.All[f.Symbol]
		devPct := snap.DeviationBetween(f.Symbol, "USDC")
		if f.IsVolatile() {
			devPct = 0 // peg deviation is meaningless for volatile assets
		}
		tokens = append(tokens, tokenInfo{
			Symbol:       f.Symbol,
			Name:         f.Name,
			VaultSlot:    f.VaultSlot,
			MainnetMint:  f.MainnetMint,
			AssetType:    f.AssetType,
			Price:        pd.Price,
			Confidence:   pd.Confidence,
			DeviationPct: devPct,
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

func sanitizeAssetSymbols(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		sym := strings.ToUpper(strings.TrimSpace(item))
		if sym == "" {
			continue
		}
		valid := true
		for _, r := range sym {
			if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func executionJobPayload(job store.ExecutionJobRow) fiber.Map {
	return fiber.Map{
		"id":                     job.ID,
		"ts":                     job.Ts.Unix(),
		"updated_ts":             job.UpdatedTs.Unix(),
		"from_index":             job.FromIndex,
		"to_index":               job.ToIndex,
		"amount":                 job.Amount,
		"stage":                  job.Stage,
		"funding_sig":            job.FundingSig,
		"swap_sig":               job.SwapSig,
		"settlement_sig":         job.SettlementSig,
		"settled_amount":         job.SettledAmount,
		"source_symbol":          job.SourceSymbol,
		"target_symbol":          job.TargetSymbol,
		"custody_account":        job.CustodyAccount,
		"target_custody_account": job.TargetCustodyAccount,
		"quote_out_amount":       job.QuoteOutAmount,
		"min_out_amount":         job.MinOutAmount,
		"price_impact_pct":       job.PriceImpactPct,
		"source_balance_before":  job.SourceBalanceBefore,
		"source_balance_after":   job.SourceBalanceAfter,
		"target_balance_before":  job.TargetBalanceBefore,
		"target_balance_after":   job.TargetBalanceAfter,
		"simulation_units":       job.SimulationUnits,
		"note":                   job.Note,
	}
}

func parseIDParam(c *fiber.Ctx, key string) (int64, error) {
	raw := c.Params(key)
	if raw == "" {
		return 0, fmt.Errorf("%s is required", key)
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return id, nil
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

// GET /api/v1/history/execution-jobs?limit=20
func (h *Handler) historyExecutionJobs(c *fiber.Ctx) error {
	if !h.requireStore(c) {
		return nil
	}
	limit := limitParam(c, 20)
	rows, err := h.store.RecentExecutionJobs(limit)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	out := make([]fiber.Map, len(rows))
	for i, row := range rows {
		out[i] = executionJobPayload(row)
	}
	return c.JSON(fiber.Map{"execution_jobs": out})
}

// GET /api/v1/execution/jobs/:id
func (h *Handler) executionJob(c *fiber.Ctx) error {
	if !h.requireStore(c) {
		return nil
	}
	id, err := parseIDParam(c, "id")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	job, err := h.store.ExecutionJobByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return fiber.NewError(fiber.StatusNotFound, "execution job not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	targetCustody := h.executionService().CustodyAccount(job.TargetSymbol)
	payload := executionJobPayload(*job)
	payload["target_custody_account"] = targetCustody
	payload["can_submit_swap"] = job.Stage == "custody_staged"
	payload["can_settle"] = job.Stage == "custody_staged" || job.Stage == "swap_submitted" || job.Stage == "swap_confirmed" || job.Stage == "reconciled_in_custody"
	return c.JSON(payload)
}

// POST /api/v1/execution/jobs/:id/build-swap
// body: {"slippage_bps": 50, "amount": 0}
func (h *Handler) buildExecutionSwap(c *fiber.Ctx) error {
	if !h.requireStore(c) {
		return nil
	}
	id, err := parseIDParam(c, "id")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var req struct {
		SlippageBps int    `json:"slippage_bps"`
		Amount      uint64 `json:"amount"`
	}
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid body")
		}
	}
	if req.SlippageBps < 0 {
		return fiber.NewError(fiber.StatusBadRequest, "slippage_bps must be >= 0")
	}

	job, err := h.store.ExecutionJobByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return fiber.NewError(fiber.StatusNotFound, "execution job not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	execSvc := h.executionService()
	prepared, err := execSvc.PrepareSwap(c.Context(), job, req.SlippageBps, req.Amount)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"job_id":    job.ID,
			"stage":     job.Stage,
			"message":   err.Error(),
			"auto_swap": false,
		})
	}

	job.QuoteOutAmount = prepared.Quote.OutAmount
	job.MinOutAmount = prepared.Quote.OtherAmountThreshold
	job.PriceImpactPct = prepared.Quote.PriceImpactPct
	job.TargetCustodyAccount = prepared.TargetCustody
	job.Note = fmt.Sprintf("Built Jupiter swap transaction for %d units of %s into %s.", prepared.SwapAmount, job.SourceSymbol, job.TargetSymbol)
	if err := h.store.UpdateExecutionJob(*job); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("update execution job: %v", err))
	}

	payload := executionJobPayload(*job)
	payload["target_custody_account"] = prepared.TargetCustody
	payload["slippage_bps"] = prepared.Quote.SlippageBps
	payload["swap_amount"] = prepared.SwapAmount
	payload["route_hops"] = len(prepared.Quote.RoutePlan)
	payload["swap_transaction"] = prepared.SwapTx.SwapTransaction
	payload["last_valid_block_height"] = prepared.SwapTx.LastValidBlockHeight
	payload["prioritization_fee_lamports"] = prepared.SwapTx.PrioritizationFeeLamports
	payload["message"] = job.Note
	return c.JSON(payload)
}

// POST /api/v1/execution/jobs/:id/execute-swap
// body: {"slippage_bps": 50, "amount": 0}
func (h *Handler) executeExecutionSwap(c *fiber.Ctx) error {
	if !h.requireStore(c) {
		return nil
	}
	id, err := parseIDParam(c, "id")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var req struct {
		SlippageBps int    `json:"slippage_bps"`
		Amount      uint64 `json:"amount"`
	}
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid body")
		}
	}
	if req.SlippageBps < 0 {
		return fiber.NewError(fiber.StatusBadRequest, "slippage_bps must be >= 0")
	}

	job, err := h.store.ExecutionJobByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return fiber.NewError(fiber.StatusNotFound, "execution job not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	execSvc := h.executionService()
	cluster := config.ExplorerClusterParam(h.cfg.SolanaRPCURL)

	prepared, jupiterErr := execSvc.PrepareSwap(c.Context(), job, req.SlippageBps, req.Amount)
	if jupiterErr != nil {
		// Jupiter failed — try devnet mock swap if EXECUTION_DEVNET_MODE=true
		if h.cfg != nil && h.cfg.ExecutionDevnetMode {
			targetCustody := execSvc.CustodyAccount(job.TargetSymbol)
			targetMint := h.cfg.DevnetMintBySymbol(job.TargetSymbol)
			if targetMint == "" {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{
					"job_id":        job.ID,
					"stage":         job.Stage,
					"message":       fmt.Sprintf("Jupiter unavailable and devnet mint not configured for %s. Set MINT_%s in .env", job.TargetSymbol, job.TargetSymbol),
					"jupiter_error": jupiterErr.Error(),
				})
			}
			mockResult, mockErr := execSvc.DevnetMockSwap(c.Context(), job, targetCustody, targetMint)
			if mockErr != nil {
				return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("devnet mock swap: %v", mockErr))
			}
			payload := executionJobPayload(*job)
			payload["target_custody_account"] = mockResult.TargetCustody
			payload["swap_amount"] = mockResult.OutAmount
			payload["target_mint"] = mockResult.TargetMint
			payload["message"] = job.Note
			payload["swap_mode"] = "devnet_mint_fallback"
			payload["explorer"] = fmt.Sprintf("https://explorer.solana.com/tx/%s%s", mockResult.MintSig, cluster)
			payload["note"] = "Jupiter unavailable on devnet — minted target tokens directly (wallet is mint authority). All token movements are real on-chain transactions."
			return c.Status(fiber.StatusAccepted).JSON(payload)
		}
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"job_id":    job.ID,
			"stage":     job.Stage,
			"message":   jupiterErr.Error(),
			"auto_swap": false,
		})
	}

	job.QuoteOutAmount = prepared.Quote.OutAmount
	job.MinOutAmount = prepared.Quote.OtherAmountThreshold
	job.PriceImpactPct = prepared.Quote.PriceImpactPct
	job.TargetCustodyAccount = prepared.TargetCustody

	submission, err := execSvc.SubmitAndReconcile(c.Context(), job, prepared.TargetCustody, prepared.SwapTx.SwapTransaction, prepared.Quote.OtherAmountThreshold)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("execute jupiter swap: %v", err))
	}
	job = submission.Job

	payload := executionJobPayload(*job)
	payload["target_custody_account"] = prepared.TargetCustody
	payload["slippage_bps"] = prepared.Quote.SlippageBps
	payload["swap_amount"] = prepared.SwapAmount
	payload["route_hops"] = len(prepared.Quote.RoutePlan)
	payload["simulation_units"] = submission.SimulationUnits
	payload["source_delta"] = submission.SourceDelta
	payload["target_delta"] = submission.TargetDelta
	payload["last_valid_block_height"] = prepared.SwapTx.LastValidBlockHeight
	payload["prioritization_fee_lamports"] = prepared.SwapTx.PrioritizationFeeLamports
	payload["message"] = job.Note
	payload["explorer"] = fmt.Sprintf("https://explorer.solana.com/tx/%s%s", submission.SwapSig, cluster)
	return c.Status(fiber.StatusAccepted).JSON(payload)
}

// POST /api/v1/execution/jobs/:id/submit-swap
// body: {"swap_transaction":"<base64 versioned tx>"}
func (h *Handler) submitExecutionSwap(c *fiber.Ctx) error {
	if !h.requireStore(c) {
		return nil
	}
	id, err := parseIDParam(c, "id")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var req struct {
		SwapTransaction string `json:"swap_transaction"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if req.SwapTransaction == "" {
		return fiber.NewError(fiber.StatusBadRequest, "swap_transaction is required")
	}

	job, err := h.store.ExecutionJobByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return fiber.NewError(fiber.StatusNotFound, "execution job not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if job.Stage != "custody_staged" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error":   "execution job is not awaiting swap submission",
			"job_id":  job.ID,
			"stage":   job.Stage,
			"message": "Only custody_staged jobs can accept an external swap transaction.",
		})
	}

	execSvc := h.executionService()
	targetCustody := execSvc.CustodyAccount(job.TargetSymbol)
	if targetCustody == "" {
		return fiber.NewError(fiber.StatusConflict, "target execution custody account is not configured")
	}

	submission, err := execSvc.SubmitAndReconcile(c.Context(), job, targetCustody, req.SwapTransaction, job.MinOutAmount)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("submit external swap: %v", err))
	}
	job = submission.Job

	payload := executionJobPayload(*job)
	payload["target_custody_account"] = targetCustody
	payload["simulation_units"] = submission.SimulationUnits
	payload["source_delta"] = submission.SourceDelta
	payload["target_delta"] = submission.TargetDelta
	payload["message"] = job.Note
	payload["explorer"] = fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", submission.SwapSig)
	return c.Status(fiber.StatusAccepted).JSON(payload)
}

// POST /api/v1/execution/jobs/:id/settle
// body: {"amount": 0} -> settles the full balance currently present in the target custody account
func (h *Handler) settleExecutionJob(c *fiber.Ctx) error {
	if !h.requireStore(c) {
		return nil
	}
	id, err := parseIDParam(c, "id")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var req struct {
		Amount uint64 `json:"amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}

	job, err := h.store.ExecutionJobByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return fiber.NewError(fiber.StatusNotFound, "execution job not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if job.Stage != "custody_staged" && job.Stage != "swap_submitted" && job.Stage != "swap_confirmed" && job.Stage != "reconciled_in_custody" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error":   "execution job is not settleable",
			"job_id":  job.ID,
			"stage":   job.Stage,
			"message": "Only custody_staged, swap_submitted, swap_confirmed, or reconciled_in_custody jobs can be settled back into treasury.",
		})
	}

	targetCustody := h.executionService().CustodyAccount(job.TargetSymbol)
	if targetCustody == "" {
		return fiber.NewError(fiber.StatusConflict, "target execution custody account is not configured")
	}

	available, err := h.executor.TokenAccountBalance(c.Context(), targetCustody)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("target custody balance: %v", err))
	}

	reconciledAmount := execution.ReconciledAmount(*job)
	settleAmount := req.Amount
	if settleAmount == 0 {
		settleAmount = reconciledAmount
		if settleAmount == 0 {
			settleAmount = available
		}
	}
	if settleAmount == 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"job_id":            job.ID,
			"stage":             job.Stage,
			"available_amount":  available,
			"reconciled_amount": reconciledAmount,
			"message":           "No target token balance is available in execution custody for settlement.",
		})
	}
	if settleAmount > available {
		return fiber.NewError(fiber.StatusBadRequest, "requested amount exceeds target custody balance")
	}

	settlementSig, err := h.executor.SendDeposit(c.Context(), settleAmount, targetCustody, uint8(job.ToIndex))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("settle execution job: %v", err))
	}

	job.Stage = "settled_back_to_treasury"
	job.SettlementSig = settlementSig
	job.SettledAmount = settleAmount
	job.Note = fmt.Sprintf("Settled %d units of %s from execution custody back into treasury.", settleAmount, job.TargetSymbol)
	if err := h.store.UpdateExecutionJob(*job); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("update execution job: %v", err))
	}
	_ = h.store.SaveRebalance(job.FromIndex, job.ToIndex, settleAmount, settlementSig, 0)

	// Record the settlement on-chain: links vault accounting to the external swap TX.
	// This creates the full audit trail: funding TX → swap TX → settlement receipt.
	swapRefSig := job.SwapSig
	if swapRefSig == "" {
		swapRefSig = job.FundingSig // fall back to funding sig if no swap was submitted
	}
	var settlementReceiptSig string
	if swapRefSig != "" {
		inputAmt := job.Amount
		receiptCtx, receiptCancel := context.WithTimeout(c.Context(), 15*time.Second)
		if rSig, rErr := h.executor.SendRecordSwapResult(receiptCtx, uint8(job.FromIndex), uint8(job.ToIndex), inputAmt, settleAmount, swapRefSig); rErr == nil {
			settlementReceiptSig = rSig
		}
		receiptCancel()
	}

	cluster := config.ExplorerClusterParam(h.cfg.SolanaRPCURL)

	payload := executionJobPayload(*job)
	payload["available_amount_before_settlement"] = available
	payload["reconciled_amount"] = reconciledAmount
	payload["message"] = job.Note
	payload["explorer"] = fmt.Sprintf("https://explorer.solana.com/tx/%s%s", settlementSig, cluster)
	if settlementReceiptSig != "" {
		payload["settlement_receipt_sig"] = settlementReceiptSig
		payload["settlement_receipt_explorer"] = fmt.Sprintf("https://explorer.solana.com/tx/%s%s", settlementReceiptSig, cluster)
		payload["audit_trail"] = []string{
			fmt.Sprintf("Funding: %s", job.FundingSig),
			fmt.Sprintf("Swap: %s", job.SwapSig),
			fmt.Sprintf("Settlement: %s", settlementSig),
			fmt.Sprintf("On-chain receipt: %s", settlementReceiptSig),
		}
	}
	if h.alerter != nil {
		h.alerter.Send("manual_execution_settled", alerts.LevelInfo,
			fmt.Sprintf("💸 Execution settled back into treasury\nRoute: *%s → %s*\nSettled amount: *%d* base units\nSwap tx: `%s`\nSettlement tx: `%s`",
				job.SourceSymbol, job.TargetSymbol, settleAmount, swapRefSig, settlementSig))
	}
	return c.JSON(payload)
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
	executionReadiness := h.cfg.ExecutionReadiness()
	growthReadiness := h.cfg.GrowthReadiness()
	persisted := false
	var persistedUpdatedAt string
	if h.store != nil {
		if settings, err := h.store.OperatorSettings(); err == nil {
			persisted = true
			persistedUpdatedAt = settings.UpdatedAt.UTC().Format(time.RFC3339)
		} else if err != sql.ErrNoRows {
			persistedUpdatedAt = "error"
		}
	}
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
		"ai_agent_model": func() string {
			if h.cfg == nil {
				return "claude-haiku-4-5"
			}
			return h.cfg.AIAgentModel
		}(),
		"ai_decision_profile": func() string {
			if h.cfg == nil {
				return "balanced"
			}
			return h.cfg.AIDecisionProfile
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
		"execution_mode":                     executionReadiness.Mode,
		"custody_model":                      "external_execution_custody_with_onchain_audit",
		"execution_note":                     executionReadiness.Note,
		"execution_mainnet_rpc":              executionReadiness.MainnetRPC,
		"execution_ready_for_staging":        executionReadiness.ReadyForStaging,
		"execution_ready_for_auto_swap":      executionReadiness.ReadyForAutoSwap,
		"execution_approval_mode":            executionReadiness.ApprovalMode,
		"execution_auto_enabled":             executionReadiness.AutoExecutionEnabled,
		"execution_missing_custody_accounts": executionReadiness.MissingCustodyAccounts,
		"growth_sleeve_enabled":              h.cfg != nil && h.cfg.GrowthSleeveEnabled,
		"growth_sleeve_mode":                 growthReadiness.Mode,
		"growth_sleeve_budget_pct":           growthReadiness.BudgetPct,
		"growth_sleeve_max_asset_pct":        growthReadiness.MaxAssetPct,
		"growth_sleeve_allowed_assets":       growthReadiness.AllowedAssets,
		"growth_sleeve_live_execution":       growthReadiness.LiveExecution,
		"growth_sleeve_ready_for_live":       growthReadiness.ReadyForLive,
		"growth_sleeve_note":                 growthReadiness.Note,
		"mode_readiness": fiber.Map{
			"MANUAL":    modeReadiness(h.cfg, "MANUAL"),
			"GUARDED":   modeReadiness(h.cfg, "GUARDED"),
			"BALANCED":  modeReadiness(h.cfg, "BALANCED"),
			"YIELD_MAX": modeReadiness(h.cfg, "YIELD_MAX"),
		},
		"settings_persisted":  persisted,
		"settings_updated_at": persistedUpdatedAt,
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

	applyRuntimeModeProfile(h.cfg, profile)

	sig, err := h.executor.SendSetStrategy(c.Context(), onChainStrategyMode(profile.StrategyMode))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("set_strategy: %v", err))
	}
	thresholdSig, err := h.executor.SendUpdateThreshold(c.Context(), profile.RiskThreshold)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("update_threshold: %v", err))
	}
	if err := h.persistOperatorSettings(); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("persist operator settings: %v", err))
	}

	return c.JSON(fiber.Map{
		"ok":                   true,
		"control_mode":         profile.Name,
		"strategy_mode":        profile.StrategyMode,
		"strategy_name":        strategyName(profile.StrategyMode),
		"auto_execute":         profile.AutoExecute,
		"yield_enabled":        profile.YieldEnabled,
		"execution_approval":   profile.ExecutionApproval,
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
	if !h.cfg.AutoExecute {
		h.cfg.ExecutionApprovalMode = "manual"
	} else if h.cfg.ExecutionApprovalMode == "" {
		h.cfg.ExecutionApprovalMode = "auto"
	}
	if !h.cfg.YieldEnabled {
		h.cfg.GrowthSleeveLiveExecution = false
	}

	sig, err := h.executor.SendSetStrategy(c.Context(), onChainStrategyMode(uint8(req.StrategyMode)))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("set_strategy: %v", err))
	}
	thresholdSig, err := h.executor.SendUpdateThreshold(c.Context(), req.RiskThreshold)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("update_threshold: %v", err))
	}
	if err := h.persistOperatorSettings(); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("persist operator settings: %v", err))
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

// POST /api/v1/settings/growth-sleeve
func (h *Handler) applyGrowthSleeve(c *fiber.Ctx) error {
	if h.cfg == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "runtime config not attached")
	}

	var req struct {
		Enabled       *bool    `json:"enabled,omitempty"`
		BudgetPct     *float64 `json:"budget_pct,omitempty"`
		MaxAssetPct   *float64 `json:"max_asset_pct,omitempty"`
		AllowedAssets []string `json:"allowed_assets,omitempty"`
		LiveExecution *bool    `json:"live_execution,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}

	if req.Enabled != nil {
		h.cfg.GrowthSleeveEnabled = *req.Enabled
	}
	if req.BudgetPct != nil {
		if *req.BudgetPct < 0 || *req.BudgetPct > 100 {
			return fiber.NewError(fiber.StatusBadRequest, "budget_pct must be between 0 and 100")
		}
		h.cfg.GrowthSleeveBudgetPct = *req.BudgetPct
	}
	if req.MaxAssetPct != nil {
		if *req.MaxAssetPct < 0 || *req.MaxAssetPct > 100 {
			return fiber.NewError(fiber.StatusBadRequest, "max_asset_pct must be between 0 and 100")
		}
		h.cfg.GrowthSleeveMaxAssetPct = *req.MaxAssetPct
	}
	if req.AllowedAssets != nil {
		h.cfg.GrowthSleeveAllowedAssets = sanitizeAssetSymbols(req.AllowedAssets)
	}
	if req.LiveExecution != nil {
		h.cfg.GrowthSleeveLiveExecution = *req.LiveExecution
	}

	if h.cfg.GrowthSleeveMaxAssetPct > h.cfg.GrowthSleeveBudgetPct && h.cfg.GrowthSleeveBudgetPct > 0 {
		return fiber.NewError(fiber.StatusBadRequest, "max_asset_pct cannot exceed budget_pct")
	}
	if h.cfg.GrowthSleeveEnabled && h.cfg.GrowthSleeveBudgetPct == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "growth sleeve requires budget_pct > 0 when enabled")
	}
	if h.cfg.GrowthSleeveEnabled && len(h.cfg.GrowthSleeveAllowedAssets) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "growth sleeve requires at least one allowed asset when enabled")
	}

	if err := h.persistOperatorSettings(); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("persist operator settings: %v", err))
	}

	readiness := h.cfg.GrowthReadiness()
	return c.JSON(fiber.Map{
		"ok":                           true,
		"growth_sleeve_enabled":        h.cfg.GrowthSleeveEnabled,
		"growth_sleeve_mode":           readiness.Mode,
		"growth_sleeve_budget_pct":     readiness.BudgetPct,
		"growth_sleeve_max_asset_pct":  readiness.MaxAssetPct,
		"growth_sleeve_allowed_assets": readiness.AllowedAssets,
		"growth_sleeve_live_execution": readiness.LiveExecution,
		"growth_sleeve_ready_for_live": readiness.ReadyForLive,
		"growth_sleeve_note":           readiness.Note,
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
	if err := h.persistOperatorSettings(); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("persist operator settings: %v", err))
	}
	return c.JSON(fiber.Map{"ok": true, "message": "Telegram credentials updated"})
}

// POST /api/v1/notifications/telegram/register
// body: {"telegram_handle":"@name","phone":"+7701..."}
func (h *Handler) registerTelegramNotification(c *fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "store not available")
	}
	if h.alerter == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "alerter not initialized")
	}
	snapshot := h.alerter.Snapshot()
	if strings.TrimSpace(snapshot.TelegramToken) == "" {
		return fiber.NewError(fiber.StatusConflict, "telegram bot is not configured on the backend")
	}

	authType, userKey, err := authSubject(c)
	if err != nil {
		return err
	}

	var req struct {
		TelegramHandle string `json:"telegram_handle"`
		Phone          string `json:"phone"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	req.TelegramHandle = strings.TrimSpace(strings.TrimPrefix(req.TelegramHandle, "@"))
	req.Phone = strings.TrimSpace(req.Phone)
	if req.TelegramHandle == "" && req.Phone == "" {
		return fiber.NewError(fiber.StatusBadRequest, "telegram_handle or phone is required")
	}

	token, err := generateTelegramLinkToken()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create telegram link token")
	}
	if err := h.store.UpsertNotificationSubscription(store.NotificationSubscription{
		AuthType:       authType,
		UserKey:        userKey,
		Channel:        "telegram",
		TelegramHandle: req.TelegramHandle,
		Phone:          req.Phone,
		IsConfirmed:    false,
	}); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("save telegram subscription: %v", err))
	}
	if err := h.store.SaveTelegramLinkToken(token, authType, userKey, req.TelegramHandle, req.Phone, time.Now().Add(24*time.Hour)); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("save telegram link token: %v", err))
	}

	botUsername := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_USERNAME"))
	deepLink := ""
	if botUsername != "" {
		deepLink = fmt.Sprintf("https://t.me/%s?start=stableguard_%s", botUsername, token)
	}

	return c.JSON(fiber.Map{
		"ok":              true,
		"auth_type":       authType,
		"user_key":        userKey,
		"telegram_handle": req.TelegramHandle,
		"phone":           req.Phone,
		"link_token":      token,
		"deep_link":       deepLink,
		"message":         "Telegram contact saved. Start the StableGuard bot once to confirm this chat for alerts.",
	})
}

// GET /api/v1/notifications/telegram/link
func (h *Handler) telegramNotificationLink(c *fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "store not available")
	}
	authType, userKey, err := authSubject(c)
	if err != nil {
		return err
	}
	sub, err := h.store.NotificationSubscription(authType, userKey, "telegram")
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(fiber.Map{"configured": false})
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	token, err := generateTelegramLinkToken()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create telegram link token")
	}
	if err := h.store.SaveTelegramLinkToken(token, authType, userKey, sub.TelegramHandle, sub.Phone, time.Now().Add(24*time.Hour)); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("save telegram link token: %v", err))
	}
	botUsername := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_USERNAME"))
	deepLink := ""
	if botUsername != "" {
		deepLink = fmt.Sprintf("https://t.me/%s?start=stableguard_%s", botUsername, token)
	}
	return c.JSON(fiber.Map{
		"configured":      true,
		"confirmed":       sub.IsConfirmed,
		"telegram_handle": sub.TelegramHandle,
		"phone":           sub.Phone,
		"deep_link":       deepLink,
		"link_token":      token,
	})
}

// GET /api/v1/notifications/telegram/status
func (h *Handler) telegramNotificationStatus(c *fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "store not available")
	}
	authType, userKey, err := authSubject(c)
	if err != nil {
		return err
	}
	sub, err := h.store.NotificationSubscription(authType, userKey, "telegram")
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(fiber.Map{"configured": false, "confirmed": false})
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{
		"configured":      true,
		"confirmed":       sub.IsConfirmed,
		"telegram_handle": sub.TelegramHandle,
		"phone":           sub.Phone,
		"chat_linked":     strings.TrimSpace(sub.TelegramChatID) != "",
	})
}

// POST /api/v1/notifications/test-alert
func (h *Handler) userNotificationTestAlert(c *fiber.Ctx) error {
	if h.store == nil || h.alerter == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "notifications are not ready")
	}
	authType, userKey, err := authSubject(c)
	if err != nil {
		return err
	}
	sub, err := h.store.NotificationSubscription(authType, userKey, "telegram")
	if err != nil {
		if err == sql.ErrNoRows {
			return fiber.NewError(fiber.StatusBadRequest, "telegram notifications are not configured for this user")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if !sub.IsConfirmed || strings.TrimSpace(sub.TelegramChatID) == "" {
		return fiber.NewError(fiber.StatusConflict, "telegram chat is not linked yet; start the bot first")
	}
	h.alerter.SendTelegramDirect(sub.TelegramChatID, alerts.LevelInfo, fmt.Sprintf("🔔 Personal StableGuard test alert\nWallet/account: `%s`\nTelegram relay is linked and ready.", userKey))
	return c.JSON(fiber.Map{"ok": true, "message": "Test alert sent"})
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
	if err := h.persistOperatorSettings(); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("persist operator settings: %v", err))
	}
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

// ── Demo / Hackathon endpoints ────────────────────────────────────────────────

// POST /api/v1/demo/simulate-event
// Unified event simulation endpoint for reserve shocks and crypto crashes.
// Body examples:
//
//	{"kind":"depeg","magnitude_pct":2.0}
//	{"kind":"crash","asset":"SOL","magnitude_pct":30.0}
func (h *Handler) simulateEvent(c *fiber.Ctx) error {
	if h.pipe == nil {
		return c.Status(503).JSON(fiber.Map{
			"error": "pipeline not running — start backend with ANTHROPIC_API_KEY set",
		})
	}

	var body struct {
		Kind         string  `json:"kind"`
		Asset        string  `json:"asset"`
		MagnitudePct float64 `json:"magnitude_pct"`
		DepegPct     float64 `json:"depeg_pct"`
		CrashPct     float64 `json:"crash_pct"`
	}
	body.Kind = strings.ToLower(strings.TrimSpace(c.Query("kind", "depeg")))
	if err := c.BodyParser(&body); err != nil && len(c.Body()) > 0 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if body.Kind == "" {
		body.Kind = "depeg"
	}

	switch body.Kind {
	case "depeg":
		magnitude := body.MagnitudePct
		if magnitude <= 0 {
			magnitude = body.DepegPct
		}
		if magnitude <= 0 {
			magnitude = 2.0
		}
		result := h.pipe.SimulateDepeg(c.Context(), magnitude)
		return c.JSON(fiber.Map{
			"kind":          "depeg",
			"asset":         "USDT",
			"magnitude_pct": magnitude,
			"depeg_pct":     result.DepegPct,
			"prices":        result.Prices,
			"score":         result.Score,
			"decision":      result.Decision,
			"on_chain_sig":  result.OnChainSig,
			"explorer_url":  result.ExplorerURL,
			"error":         result.Error,
		})
	case "crash":
		magnitude := body.MagnitudePct
		if magnitude <= 0 {
			magnitude = body.CrashPct
		}
		if magnitude <= 0 {
			magnitude = 15.0
		}
		asset := strings.ToUpper(strings.TrimSpace(body.Asset))
		if asset == "" {
			asset = "BTC"
		}
		result := h.pipe.SimulateCrash(c.Context(), asset, magnitude)
		return c.JSON(fiber.Map{
			"kind":          "crash",
			"asset":         result.Asset,
			"magnitude_pct": magnitude,
			"crash_pct":     result.CrashPct,
			"price_before":  result.PriceBefore,
			"price_after":   result.PriceAfter,
			"prices":        result.Prices,
			"score":         result.Score,
			"decision":      result.Decision,
			"on_chain_sig":  result.OnChainSig,
			"explorer_url":  result.ExplorerURL,
			"error":         result.Error,
		})
	default:
		return fiber.NewError(fiber.StatusBadRequest, "kind must be one of: depeg, crash")
	}
}

// POST /api/v1/demo/simulate-depeg
// Inject a fake USDT depeg, run AI agents, record decision on-chain, return proof.
// Body: {"depeg_pct": 2.0}   (default 2.0 if omitted)
func (h *Handler) simulateDepeg(c *fiber.Ctx) error {
	if h.pipe == nil {
		return c.Status(503).JSON(fiber.Map{
			"error": "pipeline not running — start backend with ANTHROPIC_API_KEY set",
		})
	}

	var body struct {
		DepegPct float64 `json:"depeg_pct"`
	}
	body.DepegPct = 2.0
	_ = c.BodyParser(&body)
	if body.DepegPct <= 0 {
		body.DepegPct = 2.0
	}

	result := h.pipe.SimulateDepeg(c.Context(), body.DepegPct)
	return c.JSON(result)
}

// POST /api/v1/demo/simulate-crash
// Inject a crypto crash scenario for BTC/ETH/SOL, run AI agents, record on-chain.
// Body: {"asset": "BTC", "crash_pct": 15.0}
func (h *Handler) simulateCrash(c *fiber.Ctx) error {
	if h.pipe == nil {
		return c.Status(503).JSON(fiber.Map{
			"error": "pipeline not running — start backend with ANTHROPIC_API_KEY set",
		})
	}

	var body struct {
		Asset    string  `json:"asset"`
		CrashPct float64 `json:"crash_pct"`
	}
	body.Asset = "BTC"
	body.CrashPct = 15.0
	_ = c.BodyParser(&body)

	result := h.pipe.SimulateCrash(c.Context(), body.Asset, body.CrashPct)
	return c.JSON(result)
}

// POST /api/v1/demo/init-vault — initialize vault PDA on-chain (one-time setup)
func (h *Handler) demoInitVault(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 60*time.Second)
	defer cancel()

	sig, err := h.executor.SendInitialize(ctx, 10, 1_000_000_000_000)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": err.Error(),
			"note":  "Vault may already be initialized — try simulate-depeg directly",
		})
	}

	// Wait for the vault account to be confirmed before returning.
	// Without this, subsequent register-token / deposit calls fail with AccountNotInitialized.
	if _, waitErr := h.executor.WaitForSignatureConfirmation(ctx, sig, 30*time.Second, rpc.ConfirmationStatusConfirmed); waitErr != nil {
		// Non-fatal: return success with a warning so the caller knows to retry operations.
		cluster := config.ExplorerClusterParam(h.cfg.SolanaRPCURL)
		return c.JSON(fiber.Map{
			"sig":          sig,
			"explorer_url": "https://explorer.solana.com/tx/" + sig + cluster,
			"warning":      fmt.Sprintf("TX submitted but confirmation timed out (%v). Wait a few seconds before calling register-token.", waitErr),
			"note":         "Vault initialization submitted. Poll /api/v1/vault to check when vault appears.",
		})
	}

	cluster := config.ExplorerClusterParam(h.cfg.SolanaRPCURL)
	return c.JSON(fiber.Map{
		"sig":          sig,
		"explorer_url": "https://explorer.solana.com/tx/" + sig + cluster,
		"note":         "Vault confirmed on-chain! Now call /demo/register-token for each token, then /demo/full-rebalance.",
	})
}

// GET /api/v1/demo/latest-proof
// Return the latest AI decision + Solana Explorer URL for the on-chain record_decision PDA.
func (h *Handler) latestOnChainProof(c *fiber.Ctx) error {
	type proofResp struct {
		Decision    interface{} `json:"decision"`
		ExplorerURL string      `json:"explorer_url"`
		OnChainSig  string      `json:"on_chain_sig"`
		Note        string      `json:"note"`
	}

	var dec interface{}
	var execSig string
	if h.pipe != nil {
		score, snap, d := h.pipe.LastState()
		_ = score
		_ = snap
		dec = d
	}
	if h.store != nil {
		if rows, err := h.store.RecentDecisions(1); err == nil && len(rows) > 0 {
			execSig = rows[0].ExecSig
		}
	}

	explorerURL := ""
	if execSig != "" {
		cluster := config.ExplorerClusterParam(h.cfg.SolanaRPCURL)
		explorerURL = "https://explorer.solana.com/tx/" + execSig + cluster
	}

	return c.JSON(proofResp{
		Decision:    dec,
		ExplorerURL: explorerURL,
		OnChainSig:  execSig,
		Note:        "Each AI decision writes a record_decision PDA to Solana. All decisions are verifiable on-chain.",
	})
}

// POST /api/v1/vault/deposit
// Real SPL token deposit into vault. Requires vault token account to be registered.
// body: {"token_index": 0, "amount": 1000000000, "authority_token_account": "<base58>"}
func (h *Handler) vaultDeposit(c *fiber.Ctx) error {
	var req struct {
		TokenIndex            uint8  `json:"token_index"`
		Amount                uint64 `json:"amount"`
		AuthorityTokenAccount string `json:"authority_token_account"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body: expected {token_index, amount, authority_token_account}")
	}
	if req.Amount == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "amount must be > 0")
	}
	if req.AuthorityTokenAccount == "" {
		return fiber.NewError(fiber.StatusBadRequest, "authority_token_account is required")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	sig, err := h.executor.SendDeposit(ctx, req.Amount, req.AuthorityTokenAccount, req.TokenIndex)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("vault deposit: %v", err))
	}

	cluster := config.ExplorerClusterParam(h.cfg.SolanaRPCURL)

	if h.store != nil {
		_ = h.store.SaveRebalance(int(req.TokenIndex), int(req.TokenIndex), req.Amount, sig, 0)
	}

	return c.JSON(fiber.Map{
		"sig":          sig,
		"token_index":  req.TokenIndex,
		"amount":       req.Amount,
		"explorer_url": "https://explorer.solana.com/tx/" + sig + cluster,
		"message":      fmt.Sprintf("Real SPL transfer: deposited %d base units into vault slot %d", req.Amount, req.TokenIndex),
	})
}

// POST /api/v1/vault/withdraw
// Real SPL token withdrawal from vault.
// body: {"token_index": 0, "amount": 1000000000, "authority_token_account": "<base58>"}
func (h *Handler) vaultWithdraw(c *fiber.Ctx) error {
	var req struct {
		TokenIndex            uint8  `json:"token_index"`
		Amount                uint64 `json:"amount"`
		AuthorityTokenAccount string `json:"authority_token_account"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body: expected {token_index, amount, authority_token_account}")
	}
	if req.Amount == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "amount must be > 0")
	}
	if req.AuthorityTokenAccount == "" {
		return fiber.NewError(fiber.StatusBadRequest, "authority_token_account is required")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	// Use send_payment to transfer from vault token account to authority token account
	sig, err := h.executor.SendPayment(ctx, req.Amount, req.AuthorityTokenAccount, req.TokenIndex)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("vault withdraw: %v", err))
	}

	cluster := config.ExplorerClusterParam(h.cfg.SolanaRPCURL)

	return c.JSON(fiber.Map{
		"sig":          sig,
		"token_index":  req.TokenIndex,
		"amount":       req.Amount,
		"explorer_url": "https://explorer.solana.com/tx/" + sig + cluster,
		"message":      fmt.Sprintf("Real SPL transfer: withdrew %d base units from vault slot %d", req.Amount, req.TokenIndex),
	})
}

// POST /api/v1/demo/full-rebalance
// Full on-chain rebalance flow: execute_rebalance (intent) + record_decision (AI) + record_swap_result (receipt).
// Produces 3 verifiable on-chain transactions even on devnet.
// body: {"from_index":2,"to_index":0,"amount":500000000,"rationale":"Reduce ETH exposure into USDC","confidence":85}
func (h *Handler) demoFullRebalance(c *fiber.Ctx) error {
	var req struct {
		FromIndex  uint8  `json:"from_index"`
		ToIndex    uint8  `json:"to_index"`
		Amount     uint64 `json:"amount"`
		Action     string `json:"action"`     // "PROTECT" | "OPTIMIZE" | "HOLD"
		Rationale  string `json:"rationale"`
		Confidence uint8  `json:"confidence"`
	}
	req.Amount = 500_000_000 // default demo amount in base units
	req.FromIndex = 2
	req.ToIndex = 0
	req.Action = "PROTECT"
	req.Rationale = "AI reduced treasury risk by rotating a volatile sleeve into a stable reserve asset."
	req.Confidence = 85
	_ = c.BodyParser(&req)

	// Map AI action string to on-chain action code
	// 0=HOLD, 1=PROTECT, 2=OPTIMIZE
	var actionCode uint8
	switch strings.ToUpper(req.Action) {
	case "PROTECT":
		actionCode = 1
	case "OPTIMIZE":
		actionCode = 2
	default:
		actionCode = 0
	}

	if req.FromIndex == req.ToIndex {
		return fiber.NewError(fiber.StatusBadRequest, "from_index and to_index must differ")
	}
	if req.Amount == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "amount must be > 0")
	}

	cluster := config.ExplorerClusterParam(h.cfg.SolanaRPCURL)
	explorerBase := "https://explorer.solana.com/tx/"

	ctx, cancel := context.WithTimeout(c.Context(), 60*time.Second)
	defer cancel()

	result := fiber.Map{
		"from_index": req.FromIndex,
		"to_index":   req.ToIndex,
		"amount":     req.Amount,
		"steps":      []fiber.Map{},
	}
	var steps []fiber.Map

	fromSym, okFrom := tokenSymbolByIndex(req.FromIndex)
	toSym, okTo := tokenSymbolByIndex(req.ToIndex)
	if !okFrom || !okTo {
		return fiber.NewError(fiber.StatusBadRequest, "token index is not mapped to an active feed")
	}

	// Step 1: execute_rebalance — record intent on-chain
	rebalanceSig, err := h.executor.ExecuteRebalance(ctx, req.FromIndex, req.ToIndex, req.Amount)
	if err != nil {
		result["error"] = fmt.Sprintf("execute_rebalance failed: %v", err)
		result["note"] = "Vault may not be initialized or have insufficient balance. Call /demo/init-vault first."
		return c.Status(fiber.StatusBadRequest).JSON(result)
	}
	steps = append(steps, fiber.Map{
		"step":        1,
		"name":        "execute_rebalance",
		"description": "Records the rebalance intent on-chain. No balances move until real SPL transfers settle.",
		"sig":         rebalanceSig,
		"explorer":    explorerBase + rebalanceSig + cluster,
	})

	// Step 2: record_decision — record AI decision on-chain (action from AI agent)
	decisionSig, err := h.executor.SendRecordDecision(ctx, actionCode, req.Rationale, req.Confidence)
	if err != nil {
		result["rebalance_sig"] = rebalanceSig
		result["error"] = fmt.Sprintf("record_decision failed: %v", err)
		result["steps"] = steps
		return c.Status(fiber.StatusPartialContent).JSON(result)
	}
	steps = append(steps, fiber.Map{
		"step":        2,
		"name":        "record_decision",
		"description": fmt.Sprintf("Records the AI decision as an on-chain PDA. Immutable audit trail of the %s → %s rotation intent.", fromSym, toSym),
		"sig":         decisionSig,
		"explorer":    explorerBase + decisionSig + cluster,
	})

	// Step 3: Try real Jupiter swap (mainnet) or fall back gracefully (devnet/localnet).
	// Jupiter needs the wallet's ATA as source — works when wallet holds the token.
	var jupiterSig string
	var jupiterOutAmount uint64
	var jupiterInfo fiber.Map

	fromMint := pyth.MainnetMintBySlot(int(req.FromIndex))
	toMint := pyth.MainnetMintBySlot(int(req.ToIndex))

	if !config.IsLocalnetRPC(h.cfg.SolanaRPCURL) && fromMint != "" && toMint != "" {
		swapResp, quote, jupErr := jupiter.PrepareSwapTx(jupiter.QuoteRequest{
			InputMint:   fromMint,
			OutputMint:  toMint,
			Amount:      req.Amount,
			SlippageBps: 50,
		}, h.executor.WalletAddress().String(), true) // asLegacyTx=true for compatibility

		if jupErr == nil && swapResp != nil {
			jupSig, sendErr := h.executor.SendExternalTransaction(ctx, swapResp.SwapTransaction)
			if sendErr == nil {
				jupiterSig = jupSig
				if outAmt, parseErr := strconv.ParseUint(quote.OutAmount, 10, 64); parseErr == nil {
					jupiterOutAmount = outAmt
				}
				jupiterInfo = fiber.Map{
					"available":    true,
					"sig":          jupiterSig,
					"explorer":     explorerBase + jupiterSig + cluster,
					"in_amount":    quote.InAmount,
					"out_amount":   quote.OutAmount,
					"price_impact": quote.PriceImpactPct,
					"routes":       jupiter.RouteSummary(quote),
					"slippage_bps": quote.SlippageBps,
				}
				steps = append(steps, fiber.Map{
					"step":        3,
					"name":        "jupiter_swap",
					"description": fmt.Sprintf("Real Jupiter V6 swap: %s → %s. Route: %s.", fromSym, toSym, strings.Join(jupiter.RouteSummary(quote), " → ")),
					"sig":         jupiterSig,
					"explorer":    explorerBase + jupiterSig + cluster,
					"input":       req.Amount,
					"output":      jupiterOutAmount,
				})
			} else {
				jupiterInfo = fiber.Map{"available": false, "error": fmt.Sprintf("swap send failed: %v", sendErr)}
			}
		} else if jupErr != nil {
			jupiterInfo = fiber.Map{"available": false, "error": fmt.Sprintf("no route (devnet has no liquidity): %v", jupErr)}
		}
	} else {
		jupiterInfo = fiber.Map{"available": false, "note": "Jupiter requires mainnet liquidity. On devnet, this demo records intent and settlement while external swap execution remains simulated."}
	}

	// Step 4: record_swap_result — settlement receipt on-chain.
	// References the real Jupiter TX if available, otherwise uses the rebalance TX as anchor.
	outputAmount := jupiterOutAmount
	if outputAmount == 0 {
		outputAmount = uint64(float64(req.Amount) * 0.9995) // synthetic 0.05% fee estimate
	}
	if outputAmount == 0 {
		outputAmount = req.Amount
	}
	swapRef := rebalanceSig
	if jupiterSig != "" {
		swapRef = jupiterSig
	}

	settleSig, err := h.executor.SendRecordSwapResult(ctx, req.FromIndex, req.ToIndex, req.Amount, outputAmount, swapRef)
	if err != nil {
		result["error"] = fmt.Sprintf("record_swap_result failed: %v", err)
		result["steps"] = steps
		result["note"] = "execute_rebalance and record_decision succeeded. Settlement record failed — vault may have insufficient balance."
		return c.Status(fiber.StatusPartialContent).JSON(result)
	}

	settleDesc := "On-chain settlement receipt anchored to the rebalance TX."
	if jupiterSig != "" {
		settleDesc = fmt.Sprintf("On-chain settlement receipt. References real Jupiter swap TX %s.", jupiterSig[:8]+"…")
	}
	steps = append(steps, fiber.Map{
		"step":        4,
		"name":        "record_swap_result",
		"description": settleDesc,
		"sig":         settleSig,
		"explorer":    explorerBase + settleSig + cluster,
		"input":       req.Amount,
		"output":      outputAmount,
		"swap_ref":    swapRef,
	})

	if h.store != nil {
		_ = h.store.SaveRebalance(int(req.FromIndex), int(req.ToIndex), req.Amount, rebalanceSig, 0)
	}

	result["steps"] = steps
	result["jupiter"] = jupiterInfo
	txCount := len(steps)
	note := "Devnet: on-chain intent and settlement receipt are real, while the market swap leg is simulated when no Jupiter liquidity is available."
	if jupiterSig != "" {
		note = "Real Jupiter V6 swap executed on-chain with intent + decision + settlement receipt audit trail."
	}
	result["message"] = fmt.Sprintf(
		"Full rebalance completed: %d on-chain transactions. Moved %d units from slot %d (%s) to slot %d (%s).",
		txCount, req.Amount, req.FromIndex, fromSym, req.ToIndex, toSym,
	)
	result["note"] = note

	// 🔔 Telegram alert — notify operator that rebalance completed on-chain
	if h.alerter != nil {
		amountHuman := float64(req.Amount) / 1_000_000.0 // assuming 6 decimals
		actionEmoji := "🛡️"
		if strings.ToUpper(req.Action) == "OPTIMIZE" {
			actionEmoji = "📈"
		}
		msg := fmt.Sprintf(
			"🤖 *StableGuard AI Rebalance*\n\n"+
				"✅ %d on-chain TXs confirmed on Solana\n\n"+
				"📊 *Decision:* %s %s\n"+
				"🔄 *Rotation:* %s → %s\n"+
				"💰 *Amount:* %.2f tokens\n"+
				"🎯 *Confidence:* %d%%\n\n"+
				"📝 *Rationale:*\n_%s_\n\n"+
				"🔗 [View on Solana Explorer](%s)",
			txCount, actionEmoji, strings.ToUpper(req.Action), fromSym, toSym, amountHuman, req.Confidence,
			req.Rationale,
			explorerBase+rebalanceSig+cluster,
		)
		h.alerter.SendForce(alerts.LevelWarning, msg)
	}

	return c.JSON(result)
}

// GET /api/v1/demo/token — generate operator JWT for the server wallet (demo/localnet use only).
// Bypasses signature verification. Only works when RPC is localhost or devnet.
func (h *Handler) demoOperatorToken(c *fiber.Ctx) error {
	if config.IsMainnetRPC(h.cfg.SolanaRPCURL) {
		return fiber.NewError(fiber.StatusForbidden, "demo token endpoint is disabled on mainnet")
	}
	token, err := auth.GenerateWalletToken(h.executor.WalletAddress().String())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "generate token: "+err.Error())
	}
	return c.JSON(fiber.Map{
		"token":  token,
		"wallet": h.executor.WalletAddress().String(),
		"note":   "Use as: Authorization: Bearer <token>",
	})
}

// POST /api/v1/demo/register-token — register a token mint (demo, no auth required on localnet).
func (h *Handler) demoRegisterToken(c *fiber.Ctx) error {
	if config.IsMainnetRPC(h.cfg.SolanaRPCURL) {
		return fiber.NewError(fiber.StatusForbidden, "demo endpoints are disabled on mainnet")
	}
	var req struct {
		Mint       string `json:"mint"`
		TokenIndex uint8  `json:"token_index"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body: expected {mint, token_index}")
	}
	if req.Mint == "" {
		return fiber.NewError(fiber.StatusBadRequest, "mint is required")
	}
	ctx, cancel := context.WithTimeout(c.Context(), 60*time.Second)
	defer cancel()
	sig, err := h.executor.SendRegisterToken(ctx, req.Mint, req.TokenIndex)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("register_token: %v", err))
	}
	// Wait for confirmation so the next register-token / deposit call finds the account ready.
	_, _ = h.executor.WaitForSignatureConfirmation(ctx, sig, 30*time.Second, rpc.ConfirmationStatusConfirmed)
	return c.JSON(fiber.Map{
		"sig":         sig,
		"mint":        req.Mint,
		"token_index": req.TokenIndex,
		"confirmed":   true,
	})
}

// POST /api/v1/demo/deposit — deposit tokens into vault (demo, no auth required on localnet).
// body: {"token_index":0,"amount":1000000000,"authority_token_account":"<base58>"}
func (h *Handler) demoDeposit(c *fiber.Ctx) error {
	if config.IsMainnetRPC(h.cfg.SolanaRPCURL) {
		return fiber.NewError(fiber.StatusForbidden, "demo endpoints are disabled on mainnet")
	}
	var req struct {
		TokenIndex            uint8  `json:"token_index"`
		Amount                uint64 `json:"amount"`
		AuthorityTokenAccount string `json:"authority_token_account"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if req.Amount == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "amount must be > 0")
	}
	if req.AuthorityTokenAccount == "" {
		return fiber.NewError(fiber.StatusBadRequest, "authority_token_account required")
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	sig, err := h.executor.SendDeposit(ctx, req.Amount, req.AuthorityTokenAccount, req.TokenIndex)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("deposit: %v", err))
	}
	return c.JSON(fiber.Map{
		"sig":         sig,
		"token_index": req.TokenIndex,
		"amount":      req.Amount,
		"message":     fmt.Sprintf("Real SPL deposit: %d base units into vault slot %d", req.Amount, req.TokenIndex),
	})
}

// POST /api/v1/demo/set-balances
// Writes accounting balances directly into the vault on devnet via set_demo_balances instruction.
// No real SPL tokens required — purely for demo/testing purposes.
// Body: {"balances": [1000000000, 1000000000, 500000000, 500000000, 250000000, 0, 0, 0]}
func (h *Handler) demoSetBalances(c *fiber.Ctx) error {
	if config.IsMainnetRPC(h.cfg.SolanaRPCURL) {
		return fiber.NewError(fiber.StatusForbidden, "demo endpoints are disabled on mainnet")
	}

	var req struct {
		Balances       []uint64 `json:"balances"`
		TotalDeposited uint64   `json:"total_deposited"`
	}
	if err := c.BodyParser(&req); err != nil || len(req.Balances) == 0 {
		// Default: realistic demo treasury — USDC 40%, USDT 40%, ETH 10%, SOL 5%, BTC 5%
		// Values in 6-decimal units (USDC/USDT) or 8-decimal (BTC) — using 1e6 base for all demo
		req.Balances = []uint64{
			40_000_000_000, // slot 0 USDC  — $40,000
			40_000_000_000, // slot 1 USDT  — $40,000
			10_000_000_000, // slot 2 ETH   — $10,000
			 5_000_000_000, // slot 3 SOL   — $5,000
			 5_000_000_000, // slot 4 BTC   — $5,000
			             0,
			             0,
			             0,
		}
		req.TotalDeposited = 100_000_000_000
	}
	if req.TotalDeposited == 0 {
		for _, b := range req.Balances {
			req.TotalDeposited += b
		}
	}

	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	sig, err := h.executor.SendSetDemoBalances(ctx, req.Balances, req.TotalDeposited)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("set_demo_balances: %v", err))
	}

	cluster := config.ExplorerClusterParam(h.cfg.SolanaRPCURL)
	return c.JSON(fiber.Map{
		"sig":             sig,
		"explorer_url":    "https://explorer.solana.com/tx/" + sig + cluster,
		"balances_set":    req.Balances,
		"total_deposited": req.TotalDeposited,
		"message":         "Demo balances written on-chain. Vault is ready for AI rebalance demos.",
	})
}

// POST /api/v1/demo/reset-count
// Resets vault.decision_count on-chain so the next record_decision can find a free PDA slot.
// Call this once after deploying a new program version or if the counter is out of sync.
// Body: {"new_count": 0}  (optional — defaults to 0)
func (h *Handler) demoResetDecisionCount(c *fiber.Ctx) error {
	if config.IsMainnetRPC(h.cfg.SolanaRPCURL) {
		return fiber.NewError(fiber.StatusForbidden, "demo endpoints are disabled on mainnet")
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	var req struct {
		NewCount uint64 `json:"new_count"`
	}
	_ = c.BodyParser(&req)
	// Default to 0 — let retry loop scan for next available PDA
	// The loop handles "already in use" by incrementing automatically.

	sig, err := h.executor.SendAdminResetDecisionCount(ctx, req.NewCount)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": fmt.Sprintf("admin_reset_decision_count failed: %v", err),
			"note":  "Vault may not be initialized. Call /demo/init-vault first.",
		})
	}
	cluster := config.ExplorerClusterParam(h.cfg.SolanaRPCURL)
	return c.JSON(fiber.Map{
		"sig":           sig,
		"new_count":     req.NewCount,
		"explorer_url":  "https://explorer.solana.com/tx/" + sig + cluster,
		"message":       fmt.Sprintf("decision_count reset to %d. Next record_decision will create PDA[%d].", req.NewCount, req.NewCount),
		"next_step":     "Run the demo — record_decision will now find an available PDA slot.",
	})
}
