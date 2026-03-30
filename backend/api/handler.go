// Package api exposes the StableGuard REST API.
package api

import (
	"context"
	"fmt"
	"stableguard-backend/llm"
	"stableguard-backend/pyth"
	"stableguard-backend/risk"
	solanaexec "stableguard-backend/solana"

	"github.com/gofiber/fiber/v2"
)

// Handler holds all service dependencies.
type Handler struct {
	pyth     *pyth.Monitor
	llm      *llm.Client
	executor *solanaexec.Executor
}

// New creates a new API handler.
func New(p *pyth.Monitor, l *llm.Client, e *solanaexec.Executor) *Handler {
	return &Handler{pyth: p, llm: l, executor: e}
}

// Register mounts all routes on the given Fiber app.
func (h *Handler) Register(app *fiber.App) {
	v1 := app.Group("/api/v1")

	v1.Get("/health", h.health)
	v1.Get("/prices", h.prices)
	v1.Get("/risk", h.riskScore)
	v1.Post("/decide", h.decide)
	v1.Get("/vault", h.vaultState)

	// Core on-chain actions
	v1.Post("/rebalance", h.rebalance)
	v1.Post("/strategy", h.setStrategy)
	v1.Post("/send", h.sendPayment)
	v1.Post("/threshold", h.updateThreshold)
	v1.Post("/emergency", h.emergencyWithdraw)
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

	var balanceA, balanceB uint64
	var strategyMode uint8
	vs, err := h.fetchVault(c.Context())
	if err == nil {
		balanceA = vs.BalanceA
		balanceB = vs.BalanceB
		strategyMode = vs.StrategyMode
	}

	score := risk.Compute(snap, balanceA, balanceB, strategyMode)
	return c.JSON(fiber.Map{
		"risk_level":          score.RiskLevel,
		"deviation_pct":       score.Deviation,
		"suggested_direction": score.SuggestedDirection,
		"suggested_fraction":  score.SuggestedFraction,
		"action":              score.Action,
		"summary":             score.Summary,
		"strategy_mode":       strategyMode,
	})
}

// POST /api/v1/decide — fetch prices, score risk, ask Claude for a decision
func (h *Handler) decide(c *fiber.Ctx) error {
	snap, err := h.pyth.FetchSnapshot()
	if err != nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, fmt.Sprintf("pyth: %v", err))
	}

	var balanceA, balanceB uint64
	var strategyMode uint8
	vs, err := h.fetchVault(c.Context())
	if err == nil {
		balanceA = vs.BalanceA
		balanceB = vs.BalanceB
		strategyMode = vs.StrategyMode
	}

	score := risk.Compute(snap, balanceA, balanceB, strategyMode)
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
	authority := h.executor.WalletAddress()
	return c.JSON(fiber.Map{
		"authority":           authority.String(),
		"total_deposited":     vs.TotalDeposited,
		"balance_a":           vs.BalanceA,
		"balance_b":           vs.BalanceB,
		"rebalance_threshold": vs.RebalanceThreshold,
		"max_deposit":         vs.MaxDeposit,
		"decision_count":      vs.DecisionCount,
		"total_rebalances":    vs.TotalRebalances,
		"is_paused":           vs.IsPaused,
		"strategy_mode":       vs.StrategyMode,
	})
}

// POST /api/v1/rebalance — execute virtual rebalance on-chain
// body: {"direction": 0, "amount": 50000000}
func (h *Handler) rebalance(c *fiber.Ctx) error {
	var req struct {
		Direction uint8  `json:"direction"`
		Amount    uint64 `json:"amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if req.Direction > 1 {
		return fiber.NewError(fiber.StatusBadRequest, "direction must be 0 or 1")
	}
	if req.Amount == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "amount must be > 0")
	}

	sig, err := h.executor.ExecuteRebalance(c.Context(), req.Direction, req.Amount)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("execute_rebalance: %v", err))
	}
	return c.JSON(fiber.Map{
		"signature": sig,
		"direction": req.Direction,
		"amount":    req.Amount,
		"explorer":  fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", sig),
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
// body: {"amount": 1000000, "recipient": "<token_account_pubkey>", "is_token_a": true}
func (h *Handler) sendPayment(c *fiber.Ctx) error {
	var req struct {
		Amount     uint64 `json:"amount"`
		Recipient  string `json:"recipient"`
		IsTokenA   bool   `json:"is_token_a"`
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

	sig, err := h.executor.SendPayment(c.Context(), req.Amount, req.Recipient, req.IsTokenA)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("send_payment: %v", err))
	}

	token := "token_b"
	if req.IsTokenA {
		token = "token_a"
	}
	return c.JSON(fiber.Map{
		"signature": sig,
		"amount":    req.Amount,
		"token":     token,
		"recipient": req.Recipient,
		"explorer":  fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", sig),
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

	// Read current threshold before update
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
// body: {"authority_token_a": "<pubkey>", "authority_token_b": "<pubkey>"}
func (h *Handler) emergencyWithdraw(c *fiber.Ctx) error {
	var req struct {
		AuthorityTokenA string `json:"authority_token_a"`
		AuthorityTokenB string `json:"authority_token_b"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if req.AuthorityTokenA == "" || req.AuthorityTokenB == "" {
		return fiber.NewError(fiber.StatusBadRequest, "authority_token_a and authority_token_b required")
	}

	// Snapshot balances before draining
	var amountA, amountB uint64
	vs, err := h.fetchVault(c.Context())
	if err == nil {
		amountA = vs.BalanceA
		amountB = vs.BalanceB
	}

	sig, err := h.executor.SendEmergencyWithdraw(c.Context(), req.AuthorityTokenA, req.AuthorityTokenB)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("emergency_withdraw: %v", err))
	}

	return c.JSON(fiber.Map{
		"amount_a":  amountA,
		"amount_b":  amountB,
		"signature": sig,
		"explorer":  fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", sig),
	})
}

func (h *Handler) fetchVault(ctx context.Context) (*solanaexec.VaultState, error) {
	authority := h.executor.WalletAddress()
	vaultPDA, _, err := h.executor.DeriveVaultPDA(authority)
	if err != nil {
		return nil, err
	}
	return h.executor.FetchVaultState(ctx, vaultPDA)
}
