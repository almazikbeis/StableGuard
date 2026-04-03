// Package pipeline wires the real-time price stream → risk engine → AI agents → on-chain execution.
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"stableguard-backend/ai"
	"stableguard-backend/alerts"
	"stableguard-backend/config"
	"stableguard-backend/hub"
	"stableguard-backend/pyth"
	"stableguard-backend/risk"
	solanaexec "stableguard-backend/solana"
	"stableguard-backend/store"
	"stableguard-backend/yield"
)

// Engine is the real-time decision pipeline.
type Engine struct {
	streamer *pyth.Streamer
	scorer   *risk.WindowedScorer
	agents   *ai.MultiAgentSystem
	executor *solanaexec.Executor
	cfg      *config.Config
	store    *store.DB         // optional
	alerter  *alerts.Client    // optional
	hub      *hub.Hub          // optional — SSE broadcast
	yieldAgg *yield.Aggregator // optional — yield APY data

	mu                    sync.RWMutex
	lastDecision          *ai.FinalDecision
	lastScore             risk.ScoreV2
	lastExecSig           string
	lastExecStatus        string
	lastExecNote          string
	lastSnap              *pyth.PriceSnapshot
	circuitTripped        bool  // vault has been auto-paused
	activeYieldPositionID int64 // 0 = no active position
}

// New creates a pipeline Engine.
func New(
	streamer *pyth.Streamer,
	scorer *risk.WindowedScorer,
	agents *ai.MultiAgentSystem,
	executor *solanaexec.Executor,
	cfg *config.Config,
) *Engine {
	return &Engine{
		streamer: streamer,
		scorer:   scorer,
		agents:   agents,
		executor: executor,
		cfg:      cfg,
	}
}

func (e *Engine) WithStore(s *store.DB) *Engine           { e.store = s; return e }
func (e *Engine) WithAlerter(a *alerts.Client) *Engine    { e.alerter = a; return e }
func (e *Engine) WithHub(h *hub.Hub) *Engine              { e.hub = h; return e }
func (e *Engine) WithYield(agg *yield.Aggregator) *Engine { e.yieldAgg = agg; return e }

// LastState returns the most recent risk score, price snapshot, and AI decision.
func (e *Engine) LastState() (risk.ScoreV2, *pyth.PriceSnapshot, *ai.FinalDecision) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastScore, e.lastSnap, e.lastDecision
}

// Agents returns the multi-agent system (may be nil if not running).
func (e *Engine) Agents() *ai.MultiAgentSystem {
	return e.agents
}

// Run starts the pipeline loop. Blocks until ctx is cancelled.
func (e *Engine) Run(ctx context.Context) {
	prices := make(chan *pyth.PriceSnapshot, 20)
	go e.streamer.Start(ctx, prices)

	log.Printf("[pipeline] started | strategy=%s auto_execute=%v ai_interval=%ds circuit_breaker=%v",
		strategyName(e.cfg.StrategyMode), e.cfg.AutoExecute, e.cfg.AIIntervalSec,
		e.cfg.CircuitBreakerEnabled)

	var (
		lastAITime    time.Time
		lastRiskLevel float64
	)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[pipeline] stopped")
			return

		case snap, ok := <-prices:
			if !ok {
				return
			}

			// ── Step 0: Hot path — push price on-chain immediately ────────
			if e.cfg.HotPathEnabled {
				priceU64 := minPriceU64(snap)
				go func(p uint64) {
					ctx2, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					sig, err := e.executor.SendUpdatePriceAndCheck(ctx2, p)
					if err != nil {
						log.Printf("[hot-path] tick failed: %v", err)
					} else {
						log.Printf("[hot-path] tick price=%d sig=%s", p, sig)
					}
				}(priceU64)
			}

			// ── Step 1: push to window ────────────────────────────────────
			e.scorer.Push(snap)

			// ── Step 2: compute v2 risk ───────────────────────────────────
			balances := e.fetchBalances(ctx)
			score := e.scorer.Compute(snap, balances, e.cfg.StrategyMode)

			e.mu.Lock()
			e.lastScore = score
			e.lastSnap = snap
			e.mu.Unlock()

			log.Printf("[pipeline] risk=%.1f | dev=%.5f%% vel=%.6f trend=%.6f vol=%.6f | %s",
				score.RiskLevel, score.Deviation,
				score.Velocity, score.Trend, score.Volatility,
				score.Summary,
			)

			// ── Step 3: Persist prices + risk events ──────────────────────
			if e.store != nil {
				for sym, pd := range snap.All {
					_ = e.store.SavePrice(sym, pd.FeedID, pd.Price, pd.Confidence)
				}
				if score.RiskLevel >= 30 {
					_ = e.store.SaveRiskEvent(score.RiskLevel, score.Deviation, score.Summary, string(score.Action))
				}
			}

			// ── Step 4: Circuit Breaker ────────────────────────────────────
			maxDev := snap.MaxDeviation()
			if e.cfg.CircuitBreakerEnabled {
				e.checkCircuitBreaker(ctx, maxDev, score)
			}

			// ── Step 5: Risk alerts ────────────────────────────────────────
			if e.alerter != nil {
				e.checkRiskAlerts(score, maxDev)
			}

			// ── Step 6: Broadcast to SSE clients ──────────────────────────
			if e.hub != nil {
				e.broadcastUpdate(snap, score)
			}

			// ── Step 7: Decide whether to run AI ──────────────────────────
			riskJump := math.Abs(score.RiskLevel-lastRiskLevel) >= 15
			timedOut := time.Since(lastAITime) >= time.Duration(e.cfg.AIIntervalSec)*time.Second

			if !riskJump && !timedOut {
				continue
			}
			lastRiskLevel = score.RiskLevel
			lastAITime = time.Now()

			// ── Step 8: AI agents ─────────────────────────────────────────
			decision, err := e.agents.Run(ctx, snap, score, balances, e.cfg.StrategyMode)
			if err != nil {
				log.Printf("[pipeline] AI error: %v", err)
				continue
			}

			e.mu.Lock()
			e.lastDecision = decision
			e.mu.Unlock()

			log.Printf("[pipeline] decision: action=%s from=%d to=%d frac=%.2f conf=%d | %s",
				decision.Action, decision.FromIndex, decision.ToIndex,
				decision.SuggestedFraction, decision.Confidence, decision.Rationale)

			// Persist AI decision
			if e.store != nil {
				_ = e.store.SaveDecision(store.DecisionRow{
					Action:            string(decision.Action),
					FromIndex:         decision.FromIndex,
					ToIndex:           decision.ToIndex,
					SuggestedFraction: decision.SuggestedFraction,
					Confidence:        decision.Confidence,
					Rationale:         decision.Rationale,
					RiskAnalysis:      decision.RiskAnalysis,
					YieldAnalysis:     decision.YieldAnalysis,
					RiskLevel:         score.RiskLevel,
				})
			}

			// Alert on significant AI decisions
			if e.alerter != nil && decision.Action != ai.ActionHold {
				e.alerter.Send(
					"ai_decision_"+string(decision.Action),
					alerts.LevelWarning,
					fmt.Sprintf("AI Decision: *%s*\nRisk: %.0f | Confidence: %d%%\n_%s_",
						decision.Action, score.RiskLevel, decision.Confidence, decision.Rationale),
				)
			}

			// ── Step 9: Execute on-chain (optional) ───────────────────────
			if e.cfg.AutoExecute && decision.Action != ai.ActionHold {
				e.executeDecision(ctx, decision, score, balances)
			}

			// ── Step 10: Yield optimizer (Variant B — tracked position) ───
			if e.cfg.YieldEnabled && e.store != nil && e.yieldAgg != nil {
				e.handleYield(ctx, score, decision)
			}

			// Re-broadcast with decision included (includes yield position)
			if e.hub != nil {
				e.broadcastUpdate(snap, score)
			}
		}
	}
}

// ── Circuit Breaker ────────────────────────────────────────────────────────

func (e *Engine) checkCircuitBreaker(ctx context.Context, maxDev float64, score risk.ScoreV2) {
	if maxDev >= e.cfg.CircuitBreakerEmergencyPct {
		// Critical — send emergency alert (no auto-withdraw, too risky without ATA config)
		if e.alerter != nil {
			e.alerter.Send("circuit_emergency", alerts.LevelCritical,
				fmt.Sprintf("🚨 CRITICAL DEPEG DETECTED\nMax deviation: *%.4f%%* (threshold: %.1f%%)\nRisk level: %.0f\n\n⚡ Manual emergency withdrawal required!\nUse: POST /api/v1/emergency",
					maxDev, e.cfg.CircuitBreakerEmergencyPct, score.RiskLevel))
		}
		log.Printf("[circuit-breaker] EMERGENCY: max_dev=%.4f%% — alert sent, manual action required", maxDev)
		return
	}

	if maxDev >= e.cfg.CircuitBreakerPausePct {
		e.mu.Lock()
		alreadyTripped := e.circuitTripped
		e.mu.Unlock()

		if !alreadyTripped {
			log.Printf("[circuit-breaker] TRIPPED: max_dev=%.4f%% >= %.1f%% — pausing vault",
				maxDev, e.cfg.CircuitBreakerPausePct)

			sig, err := e.executor.SendTogglePause(ctx)
			if err != nil {
				log.Printf("[circuit-breaker] pause failed: %v", err)
			} else {
				e.mu.Lock()
				e.circuitTripped = true
				e.mu.Unlock()
				log.Printf("[circuit-breaker] vault paused: %s", sig)
			}

			if e.alerter != nil {
				e.alerter.Send("circuit_pause", alerts.LevelCritical,
					fmt.Sprintf("⚠️ VAULT PAUSED by circuit breaker\nMax deviation: *%.4f%%* (threshold: %.1f%%)\nRisk: %.0f | Summary: %s\n\nTx: %s",
						maxDev, e.cfg.CircuitBreakerPausePct, score.RiskLevel, score.Summary, sig))
			}
		}
	} else if e.circuitTripped {
		// Deviation recovered — reset trip state (don't auto-unpause, manual action)
		e.mu.Lock()
		e.circuitTripped = false
		e.mu.Unlock()
		log.Printf("[circuit-breaker] deviation recovered to %.4f%% — trip reset (vault still paused, manual unpause required)", maxDev)
		if e.alerter != nil {
			e.alerter.Send("circuit_recover", alerts.LevelInfo,
				fmt.Sprintf("✅ Deviation recovered to %.4f%%\nVault is still paused — manual unpause required via API.", maxDev))
		}
	}
}

// ── Risk Alerts ────────────────────────────────────────────────────────────

func (e *Engine) checkRiskAlerts(score risk.ScoreV2, maxDev float64) {
	// High risk alert
	if score.RiskLevel >= e.cfg.AlertRiskThreshold {
		e.alerter.Send("risk_high", alerts.LevelWarning,
			fmt.Sprintf("Risk level: *%.0f/100*\nDeviation: %.4f%%\nMax deviation: %.4f%%\n_%s_",
				score.RiskLevel, score.Deviation, maxDev, score.Summary))
	}

	// Depeg warning (lower threshold than circuit breaker, just an alert)
	if maxDev >= 0.5 {
		e.alerter.Send("depeg_warning", alerts.LevelWarning,
			fmt.Sprintf("Depeg warning: max deviation *%.4f%%*\nRisk: %.0f | %s",
				maxDev, score.RiskLevel, score.Summary))
	}
}

// ── SSE Broadcast ──────────────────────────────────────────────────────────

// HotPathMsg carries the last on-chain price and circuit breaker state.
type HotPathMsg struct {
	LastPrice uint64 `json:"last_price"`
	Tripped   bool   `json:"tripped"`
}

// YieldPositionMsg is the yield position embedded in SSE broadcasts.
type YieldPositionMsg struct {
	Protocol    string  `json:"protocol"`
	Token       string  `json:"token"`
	Amount      float64 `json:"amount"`
	EntryAPY    float64 `json:"entry_apy"`
	Earned      float64 `json:"earned"`
	DepositedAt int64   `json:"deposited_at"`
}

// FeedMessage is the JSON payload pushed to SSE clients.
type FeedMessage struct {
	Ts           int64              `json:"ts"`
	Risk         risk.ScoreV2       `json:"risk"`
	Prices       map[string]float64 `json:"prices"`
	MaxDeviation float64            `json:"max_deviation"`
	Decision     *ai.FinalDecision  `json:"decision,omitempty"`
	ExecSig      string             `json:"exec_sig,omitempty"`
	ExecStatus   string             `json:"exec_status,omitempty"`
	ExecNote     string             `json:"exec_note,omitempty"`
	YieldPos     *YieldPositionMsg  `json:"yield_position,omitempty"`
	HotPath      *HotPathMsg        `json:"hot_path,omitempty"`
}

func (e *Engine) broadcastUpdate(snap *pyth.PriceSnapshot, score risk.ScoreV2) {
	prices := make(map[string]float64, len(snap.All))
	for sym, pd := range snap.All {
		prices[sym] = pd.Price
	}

	e.mu.RLock()
	dec := e.lastDecision
	sig := e.lastExecSig
	status := e.lastExecStatus
	note := e.lastExecNote
	e.mu.RUnlock()

	msg := FeedMessage{
		Ts:           time.Now().Unix(),
		Risk:         score,
		Prices:       prices,
		MaxDeviation: snap.MaxDeviation(),
		Decision:     dec,
		ExecSig:      sig,
		ExecStatus:   status,
		ExecNote:     note,
	}

	// Attach hot path data if enabled
	if e.cfg.HotPathEnabled {
		msg.HotPath = &HotPathMsg{
			LastPrice: minPriceU64(snap),
			Tripped:   e.circuitTripped,
		}
	}

	// Attach active yield position with live earned calculation
	if e.store != nil {
		if pos, err := e.store.ActiveYieldPosition(); err == nil && pos != nil {
			elapsed := time.Since(pos.DepositedAt).Seconds()
			earned := pos.Amount * (pos.EntryAPY / 100) / (365.25 * 24 * 3600) * elapsed
			msg.YieldPos = &YieldPositionMsg{
				Protocol:    pos.Protocol,
				Token:       pos.Token,
				Amount:      pos.Amount,
				EntryAPY:    pos.EntryAPY,
				Earned:      earned,
				DepositedAt: pos.DepositedAt.Unix(),
			}
		}
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	e.hub.Publish(data)
}

// ── On-chain execution ─────────────────────────────────────────────────────

func (e *Engine) executeDecision(ctx context.Context, d *ai.FinalDecision, score risk.ScoreV2, balances []uint64) {
	if d.FromIndex < 0 || d.ToIndex < 0 {
		return
	}
	var fromBal uint64
	if d.FromIndex < len(balances) {
		fromBal = balances[d.FromIndex]
	}
	if fromBal == 0 {
		log.Printf("[pipeline] execute skipped — from_balance is zero")
		return
	}
	amount := uint64(float64(fromBal) * d.SuggestedFraction)
	if amount == 0 {
		return
	}

	note := "Signal only: market execution is unavailable under the current custody model because vault token accounts are program-owned. Add a dedicated swap/CPI path before enabling real execution."
	log.Printf("[pipeline] signal only: %d→%d amount=%d | %s", d.FromIndex, d.ToIndex, amount, note)
	e.mu.Lock()
	e.lastExecSig = ""
	e.lastExecStatus = "signal_only"
	e.lastExecNote = note
	e.mu.Unlock()

	var actionCode uint8
	switch d.Action {
	case ai.ActionProtect:
		actionCode = 1
	case ai.ActionOptimize:
		actionCode = 2
	}
	rationale := fmt.Sprintf("[%s] %s | risk=%.1f", d.Action, d.Rationale, score.RiskLevel)
	confidence := uint8(d.Confidence)
	if confidence > 100 {
		confidence = 100
	}
	decSig, err := e.executor.SendRecordDecision(ctx, actionCode, rationale, confidence)
	if err != nil {
		log.Printf("[pipeline] record_decision failed: %v", err)
		return
	}
	log.Printf("[pipeline] record_decision tx: %s", decSig)
}

func (e *Engine) fetchBalances(ctx context.Context) []uint64 {
	authority := e.executor.WalletAddress()
	vaultPDA, _, err := e.executor.DeriveVaultPDA(authority)
	if err != nil {
		return nil
	}
	vs, err := e.executor.FetchVaultState(ctx, vaultPDA)
	if err != nil {
		return nil
	}
	n := int(vs.NumTokens)
	balances := make([]uint64, n)
	for i := 0; i < n; i++ {
		balances[i] = vs.Balances[i]
	}
	return balances
}

// ── Status accessors ──────────────────────────────────────────────────────

func (e *Engine) LastDecision() *ai.FinalDecision {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastDecision
}

func (e *Engine) LastScore() risk.ScoreV2 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastScore
}

func (e *Engine) LastExecSig() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastExecSig
}

func (e *Engine) LastExecMeta() (string, string) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastExecStatus, e.lastExecNote
}

func (e *Engine) LastSnap() *pyth.PriceSnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastSnap
}

func (e *Engine) CircuitTripped() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.circuitTripped
}

// ── Yield Optimizer (Variant B — Tracked Position) ─────────────────────────

// handleYield manages simulated yield positions based on AI decisions and risk level.
// Entry: OPTIMIZE decision + risk < YieldEntryRisk + no open position.
// Exit:  risk > YieldExitRisk + open position exists.
func (e *Engine) handleYield(ctx context.Context, score risk.ScoreV2, decision *ai.FinalDecision) {
	e.mu.Lock()
	activeID := e.activeYieldPositionID
	e.mu.Unlock()

	// ── Exit: risk too high, withdraw ────────────────────────────────────────
	if score.RiskLevel > e.cfg.YieldExitRisk && activeID != 0 {
		pos, err := e.store.ActiveYieldPosition()
		if err != nil || pos == nil {
			e.mu.Lock()
			e.activeYieldPositionID = 0
			e.mu.Unlock()
			return
		}

		elapsed := time.Since(pos.DepositedAt).Seconds()
		earned := pos.Amount * (pos.EntryAPY / 100) / (365.25 * 24 * 3600) * elapsed

		if err := e.store.CloseYieldPosition(pos.ID, earned, "simulated-exit"); err != nil {
			log.Printf("[yield] close position failed: %v", err)
			return
		}
		e.mu.Lock()
		e.activeYieldPositionID = 0
		e.mu.Unlock()

		log.Printf("[yield] EXIT: risk=%.1f > %.1f — withdrew %.4f %s from %s | earned=%.4f",
			score.RiskLevel, e.cfg.YieldExitRisk, pos.Amount, pos.Token, pos.Protocol, earned)

		if e.alerter != nil {
			e.alerter.Send("yield_exit", alerts.LevelWarning,
				fmt.Sprintf("⚡ Yield position withdrawn\nRisk rose to *%.0f* (threshold %.0f)\nProtocol: %s | Token: %s\nEarned: *$%.4f* over %s",
					score.RiskLevel, e.cfg.YieldExitRisk,
					pos.Protocol, pos.Token, earned,
					formatDuration(time.Since(pos.DepositedAt))))
		}
		return
	}

	// ── Entry: OPTIMIZE + low risk + no active position ──────────────────────
	if activeID != 0 {
		return // already in a position
	}
	if decision == nil || decision.Action != ai.ActionOptimize {
		return
	}
	if score.RiskLevel > e.cfg.YieldEntryRisk {
		return
	}

	// Find best opportunity
	best := e.yieldAgg.BestFor(ctx, "USDC") // prefer USDC
	if best == nil {
		best = e.yieldAgg.BestFor(ctx, "USDT")
	}
	if best == nil || best.SupplyAPY < e.cfg.YieldMinAPY {
		log.Printf("[yield] no suitable opportunity (min APY %.1f%%)", e.cfg.YieldMinAPY)
		return
	}

	amount := e.cfg.YieldDepositAmount

	posID, err := e.store.SaveYieldPosition(
		string(best.Protocol),
		best.Token,
		amount,
		best.SupplyAPY,
		"simulated-deposit",
	)
	if err != nil {
		log.Printf("[yield] save position failed: %v", err)
		return
	}

	e.mu.Lock()
	e.activeYieldPositionID = posID
	e.mu.Unlock()

	log.Printf("[yield] ENTER: deposited %.0f %s into %s @ %.2f%% APY (simulated, posID=%d)",
		amount, best.Token, best.DisplayName, best.SupplyAPY, posID)

	if e.alerter != nil {
		e.alerter.Send("yield_enter", alerts.LevelInfo,
			fmt.Sprintf("💰 Yield position opened\nProtocol: *%s* | Token: %s\nAmount: *$%.0f* @ *%.2f%%* APY\nRisk: %.0f (safe to enter)",
				best.DisplayName, best.Token, amount, best.SupplyAPY, score.RiskLevel))
	}
}

// minPriceU64 returns the minimum price across all tokens as a uint64
// with 6 decimal places (e.g. 1.000000 = 1_000_000).
func minPriceU64(snap *pyth.PriceSnapshot) uint64 {
	var minPrice float64 = -1
	for _, pd := range snap.All {
		if minPrice < 0 || pd.Price < minPrice {
			minPrice = pd.Price
		}
	}
	if minPrice < 0 {
		return 0
	}
	return uint64(minPrice * 1_000_000)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

func strategyName(mode uint8) string {
	switch mode {
	case risk.StrategyModeSafe:
		return "SAFE"
	case risk.StrategyModeYieldV2:
		return "YIELD"
	default:
		return "BALANCED"
	}
}
