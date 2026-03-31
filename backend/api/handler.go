// Package api exposes the StableGuard REST API.
package api

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"time"

	"stableguard-backend/ai"
	"stableguard-backend/alerts"
	"stableguard-backend/hub"
	"stableguard-backend/llm"
	"stableguard-backend/pipeline"
	"stableguard-backend/pyth"
	"stableguard-backend/risk"
	solanaexec "stableguard-backend/solana"
	"stableguard-backend/store"
	"stableguard-backend/yield"

	"github.com/gofiber/fiber/v2"
)

// Handler holds all service dependencies.
type Handler struct {
	pyth     *pyth.Monitor
	llm      *llm.Client
	executor *solanaexec.Executor
	pipe     *pipeline.Engine    // optional
	store    *store.DB           // optional
	alerter  *alerts.Client      // optional
	feedHub  *hub.Hub            // optional — SSE broadcast hub
	yieldAgg *yield.Aggregator   // optional — yield APY aggregator
}

// New creates a new API handler.
func New(p *pyth.Monitor, l *llm.Client, e *solanaexec.Executor) *Handler {
	return &Handler{pyth: p, llm: l, executor: e}
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

// Register mounts all routes on the given Fiber app.
func (h *Handler) Register(app *fiber.App) {
	v1 := app.Group("/api/v1")

	v1.Get("/health", h.health)
	v1.Get("/prices", h.prices)
	v1.Get("/tokens", h.tokensList)        // all monitored tokens + live prices
	v1.Get("/risk", h.riskScore)           // v1: simple deviation scorer (unchanged)
	v1.Get("/risk/v2", h.riskScoreV2)      // windowed scorer with trend/velocity/volatility
	v1.Get("/pipeline/status", h.pipelineStatus) // last AI decision + score
	v1.Post("/decide", h.decide)
	v1.Get("/vault", h.vaultState)

	// Core on-chain actions
	v1.Post("/rebalance", h.rebalance)
	v1.Post("/strategy", h.setStrategy)
	v1.Post("/send", h.sendPayment)
	v1.Post("/threshold", h.updateThreshold)
	v1.Post("/emergency", h.emergencyWithdraw)
	v1.Post("/register-token", h.registerToken)

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
	v1.Get("/settings", h.getSettings)
	v1.Post("/settings/telegram", h.setTelegram)
	v1.Post("/settings/discord", h.setDiscord)
	v1.Post("/settings/test-alert", h.testAlert)
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

	sig, err := h.executor.ExecuteRebalance(c.Context(), req.FromIndex, req.ToIndex, req.Amount)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("execute_rebalance: %v", err))
	}
	return c.JSON(fiber.Map{
		"signature":  sig,
		"from_index": req.FromIndex,
		"to_index":   req.ToIndex,
		"amount":     req.Amount,
		"explorer":   fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", sig),
	})
}

// POST /api/v1/strategy — change vault strategy mode
// body: {"mode": 0}  → 0=safe, 1=yield
func (h *Handler) setStrategy(c *fiber.Ctx) error {
	var req struct {
		Mode uint8 `json:"mode"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body: expected {mode}")
	}
	if req.Mode > 1 {
		return fiber.NewError(fiber.StatusBadRequest, "mode must be 0 (safe) or 1 (yield)")
	}

	sig, err := h.executor.SendSetStrategy(c.Context(), req.Mode)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("set_strategy: %v", err))
	}

	modeName := "safe"
	if req.Mode == 1 {
		modeName = "yield"
	}
	return c.JSON(fiber.Map{
		"mode":      modeName,
		"mode_id":   req.Mode,
		"signature": sig,
		"explorer":  fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", sig),
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
		"last_exec_sig": execSig,
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
		Symbol      string  `json:"symbol"`
		Name        string  `json:"name"`
		VaultSlot   int     `json:"vault_slot"`
		MainnetMint string  `json:"mainnet_mint"`
		Price       float64 `json:"price"`
		Confidence  float64 `json:"confidence"`
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
		"tokens":       tokens,
		"fetched_at":   snap.FetchedAt,
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
		"total_decisions":  stats.TotalDecisions,
		"total_rebalances": stats.TotalRebalances,
		"total_risk_events": stats.TotalRiskEvents,
		"avg_risk_level":   stats.AvgRiskLevel,
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
	return c.JSON(fiber.Map{
		"alerts_enabled":               telegramEnabled,
		"circuit_breaker_enabled":      true,
		"pipeline_running":             h.pipe != nil,
		"hub_subscribers":              func() int {
			if h.feedHub == nil { return 0 }
			return h.feedHub.Subscribers()
		}(),
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
