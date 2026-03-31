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
)

// Engine is the real-time decision pipeline.
type Engine struct {
	streamer *pyth.Streamer
	scorer   *risk.WindowedScorer
	agents   *ai.MultiAgentSystem
	executor *solanaexec.Executor
	cfg      *config.Config
	store    *store.DB       // optional
	alerter  *alerts.Client  // optional
	hub      *hub.Hub        // optional — SSE broadcast

	mu              sync.RWMutex
	lastDecision    *ai.FinalDecision
	lastScore       risk.ScoreV2
	lastExecSig     string
	lastSnap        *pyth.PriceSnapshot
	circuitTripped  bool // vault has been auto-paused
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

func (e *Engine) WithStore(s *store.DB) *Engine   { e.store = s; return e }
func (e *Engine) WithAlerter(a *alerts.Client) *Engine { e.alerter = a; return e }
func (e *Engine) WithHub(h *hub.Hub) *Engine       { e.hub = h; return e }

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

			// Re-broadcast with decision included
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

// FeedMessage is the JSON payload pushed to SSE clients.
type FeedMessage struct {
	Ts          int64              `json:"ts"`
	Risk        risk.ScoreV2       `json:"risk"`
	Prices      map[string]float64 `json:"prices"`
	MaxDeviation float64           `json:"max_deviation"`
	Decision    *ai.FinalDecision  `json:"decision,omitempty"`
	ExecSig     string             `json:"exec_sig,omitempty"`
}

func (e *Engine) broadcastUpdate(snap *pyth.PriceSnapshot, score risk.ScoreV2) {
	prices := make(map[string]float64, len(snap.All))
	for sym, pd := range snap.All {
		prices[sym] = pd.Price
	}

	e.mu.RLock()
	dec := e.lastDecision
	sig := e.lastExecSig
	e.mu.RUnlock()

	msg := FeedMessage{
		Ts:           time.Now().Unix(),
		Risk:         score,
		Prices:       prices,
		MaxDeviation: snap.MaxDeviation(),
		Decision:     dec,
		ExecSig:      sig,
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

	log.Printf("[pipeline] executing rebalance: %d→%d amount=%d", d.FromIndex, d.ToIndex, amount)
	sig, err := e.executor.ExecuteRebalance(ctx, uint8(d.FromIndex), uint8(d.ToIndex), amount)
	if err != nil {
		log.Printf("[pipeline] execute_rebalance failed: %v", err)
		return
	}
	log.Printf("[pipeline] rebalance tx: %s", sig)

	if e.store != nil {
		_ = e.store.SaveRebalance(d.FromIndex, d.ToIndex, amount, sig, score.RiskLevel)
	}

	e.mu.Lock()
	e.lastExecSig = sig
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
