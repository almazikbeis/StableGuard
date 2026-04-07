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
	"stableguard-backend/execution"
	"stableguard-backend/hub"
	"stableguard-backend/jupiter"
	"stableguard-backend/onchain"
	"stableguard-backend/policy"
	"stableguard-backend/pyth"
	"stableguard-backend/risk"
	solanaexec "stableguard-backend/solana"
	"stableguard-backend/store"
	"stableguard-backend/yield"
)

const maxUint64Decimal = "18446744073709551615"

// Engine is the real-time decision pipeline.
type Engine struct {
	streamer *pyth.Streamer
	scorer   *risk.WindowedScorer
	agents   *ai.MultiAgentSystem
	executor *solanaexec.Executor
	cfg      *config.Config
	store    *store.DB           // optional
	alerter  *alerts.Client      // optional
	hub      *hub.Hub            // optional — SSE broadcast
	yieldAgg *yield.Aggregator   // optional — yield APY data
	whaleAgg *onchain.Aggregator // optional — DexScreener whale signals

	mu                    sync.RWMutex
	lastDecision          *ai.FinalDecision
	lastPolicyEval        policy.Evaluation
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

func (e *Engine) WithStore(s *store.DB) *Engine              { e.store = s; return e }
func (e *Engine) WithAlerter(a *alerts.Client) *Engine       { e.alerter = a; return e }
func (e *Engine) WithHub(h *hub.Hub) *Engine                 { e.hub = h; return e }
func (e *Engine) WithYield(agg *yield.Aggregator) *Engine    { e.yieldAgg = agg; return e }
func (e *Engine) WithWhales(agg *onchain.Aggregator) *Engine { e.whaleAgg = agg; return e }

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
			// Use stablecoin max deviation for depeg detection.
			// For volatile crash events, elevate maxDev if risk score is critical.
			maxDev := snap.MaxDeviation()
			if e.cfg.CircuitBreakerEnabled {
				effectiveDev := maxDev
				if score.RiskLevel >= 80 && effectiveDev < e.cfg.CircuitBreakerPausePct {
					// Crypto crash ≥80 risk overrides depeg threshold
					effectiveDev = e.cfg.CircuitBreakerPausePct
				}
				e.checkCircuitBreaker(ctx, effectiveDev, score)
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

			// ── Step 8: AI agents (with live whale score if available) ────
			var whaleScore float64
			if e.whaleAgg != nil {
				whaleScore = e.whaleAgg.Last().Score
			}
			decision, err := e.agents.Run(ctx, snap, score, balances, e.cfg.StrategyMode, whaleScore)
			if err != nil {
				log.Printf("[pipeline] AI error: %v", err)
				continue
			}

			e.mu.Lock()
			e.lastDecision = decision
			e.lastPolicyEval = policy.Evaluate(e.cfg, decision)
			e.mu.Unlock()

			log.Printf("[pipeline] decision: action=%s from=%d to=%d frac=%.2f conf=%d | %s",
				decision.Action, decision.FromIndex, decision.ToIndex,
				decision.SuggestedFraction, decision.Confidence, decision.Rationale)

			// Record non-HOLD decisions on-chain and persist with sig
			var onChainSig string
			if decision.Action != ai.ActionHold {
				if sig, err := e.recordDecisionTrace(ctx, decision, score); err != nil {
					log.Printf("[pipeline] record_decision failed: %v", err)
				} else {
					onChainSig = sig
					log.Printf("[pipeline] record_decision tx: %s", sig)
				}
			}
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
					ExecSig:           onChainSig,
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

			// ── Step 9: Execute on-chain ──────────────────────────────────
			policyEval := policy.Evaluate(e.cfg, decision)
			// Always attempt the on-chain vault rebalance when AI says act.
			// This is separate from Jupiter swap which needs mainnet custody.
			if policyEval.Verdict == policy.VerdictAllowed && e.cfg.AutoExecute && decision.Action != ai.ActionHold {
				go e.autoRebalanceOnChain(ctx, decision, balances)
				e.executeDecision(ctx, decision, score, balances)
			} else {
				e.mu.Lock()
				e.lastExecSig = ""
				switch policyEval.Verdict {
				case policy.VerdictBlocked:
					e.lastExecStatus = "blocked_by_policy"
					e.lastExecNote = policyEval.Reason
				case policy.VerdictRequiresApproval:
					e.lastExecStatus = "approval_required"
					e.lastExecNote = policyEval.Reason
				default:
					e.lastExecStatus = "standby"
					e.lastExecNote = "No execution requested by the current AI decision."
				}
				e.mu.Unlock()
			}

			// ── Step 10: Yield optimizer (Variant B — tracked position) ───
			if e.cfg.YieldEnabled && e.store != nil && e.yieldAgg != nil {
				e.handleYield(ctx, score, decision, policyEval)
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
	Policy       policy.Evaluation  `json:"policy"`
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
	pol := e.lastPolicyEval
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
		Policy:       pol,
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

	if e.cfg == nil || e.cfg.ExecutionApprovalMode != "auto" {
		note := "Execution approval mode is manual. AI created an actionable decision, but autonomous Jupiter execution is disabled until EXECUTION_APPROVAL_MODE=auto."
		e.mu.Lock()
		e.lastExecSig = ""
		e.lastExecStatus = "approval_required"
		e.lastExecNote = note
		e.mu.Unlock()
		return
	}

	if e.store == nil {
		note := "Execution store is unavailable, so autonomous swap lifecycle tracking is disabled."
		e.mu.Lock()
		e.lastExecSig = ""
		e.lastExecStatus = "blocked_by_policy"
		e.lastExecNote = note
		e.mu.Unlock()
		return
	}

	readiness := e.cfg.ExecutionReadiness()
	if !readiness.ReadyForAutoSwap {
		e.mu.Lock()
		e.lastExecSig = ""
		e.lastExecStatus = "signal_only"
		e.lastExecNote = readiness.Note
		e.mu.Unlock()
		return
	}

	hasActive, err := e.store.HasActiveExecutionJob()
	if err != nil {
		e.mu.Lock()
		e.lastExecSig = ""
		e.lastExecStatus = "failed"
		e.lastExecNote = "Could not determine whether another execution job is already active."
		e.mu.Unlock()
		return
	}
	if hasActive {
		e.mu.Lock()
		e.lastExecSig = ""
		e.lastExecStatus = "blocked_by_policy"
		e.lastExecNote = "Another execution job is still active. Autonomous swap execution is serialized and fail-closed."
		e.mu.Unlock()
		return
	}

	if err := e.executeAutonomousSwap(ctx, d, score, amount); err != nil {
		log.Printf("[pipeline] autonomous execution failed: %v", err)
		e.mu.Lock()
		e.lastExecSig = ""
		e.lastExecStatus = "failed"
		e.lastExecNote = err.Error()
		e.mu.Unlock()
	}
}

func (e *Engine) executeAutonomousSwap(ctx context.Context, d *ai.FinalDecision, score risk.ScoreV2, amount uint64) error {
	execSvc := execution.New(e.executor, e.cfg, e.store)
	fromSymbol, ok := tokenSymbolByIndex(uint8(d.FromIndex))
	if !ok {
		return fmt.Errorf("execution aborted: source token slot %d is not mapped to an active feed", d.FromIndex)
	}
	toSymbol, ok := tokenSymbolByIndex(uint8(d.ToIndex))
	if !ok {
		return fmt.Errorf("execution aborted: target token slot %d is not mapped to an active feed", d.ToIndex)
	}

	sourceCustody := execSvc.CustodyAccount(fromSymbol)
	targetCustody := execSvc.CustodyAccount(toSymbol)
	if sourceCustody == "" || targetCustody == "" {
		return fmt.Errorf("execution aborted: trusted execution custody accounts are missing for %s -> %s", fromSymbol, toSymbol)
	}

	fundingSig, err := e.executor.SendPayment(ctx, amount, sourceCustody, uint8(d.FromIndex))
	if err != nil {
		return fmt.Errorf("execution aborted while staging source funds into custody: %w", err)
	}

	job := store.ExecutionJobRow{
		FromIndex:            d.FromIndex,
		ToIndex:              d.ToIndex,
		Amount:               amount,
		Stage:                "custody_staged",
		FundingSig:           fundingSig,
		SourceSymbol:         fromSymbol,
		TargetSymbol:         toSymbol,
		CustodyAccount:       sourceCustody,
		TargetCustodyAccount: targetCustody,
		Note:                 fmt.Sprintf("AI staged %s into trusted execution custody for autonomous swap into %s.", fromSymbol, toSymbol),
	}
	jobID, err := e.store.SaveExecutionJob(job)
	if err != nil {
		return fmt.Errorf("execution aborted while saving execution job: %w", err)
	}
	job.ID = jobID

	sourceFeed, ok := tokenFeedBySymbol(fromSymbol)
	if !ok {
		return e.failExecutionJob(jobID, fmt.Sprintf("execution job %d staged, but source token feed mapping is missing", jobID))
	}
	targetFeed, ok := tokenFeedBySymbol(toSymbol)
	if !ok {
		return e.failExecutionJob(jobID, fmt.Sprintf("execution job %d staged, but target token feed mapping is missing", jobID))
	}

	// ── Try Jupiter swap; fall back to devnet mock when unavailable ──────
	var swapSig string
	var settleAmount uint64

	quote, jupiterErr := jupiter.GetQuote(jupiter.QuoteRequest{
		InputMint:   sourceFeed.MainnetMint,
		OutputMint:  targetFeed.MainnetMint,
		Amount:      amount,
		SlippageBps: execSvc.SlippageBps(0),
	})

	if jupiterErr != nil && e.cfg.ExecutionDevnetMode {
		// Devnet fallback: mint target tokens directly (wallet must be mint authority)
		targetMint := e.cfg.DevnetMintBySymbol(toSymbol)
		if targetMint == "" {
			return e.failExecutionJob(jobID, fmt.Sprintf("execution job %d: Jupiter unavailable, devnet mint not configured for %s", jobID, toSymbol))
		}
		mockResult, mockErr := execSvc.DevnetMockSwap(ctx, &job, targetCustody, targetMint)
		if mockErr != nil {
			return e.failExecutionJob(jobID, fmt.Sprintf("execution job %d devnet mock swap failed: %v", jobID, mockErr))
		}
		swapSig = mockResult.MintSig
		settleAmount = mockResult.OutAmount
		log.Printf("[pipeline] devnet mock swap: minted %d %s to custody. sig=%s", settleAmount, toSymbol, swapSig)
	} else if jupiterErr != nil {
		return e.failExecutionJob(jobID, fmt.Sprintf("execution job %d failed while fetching a Jupiter quote: %v", jobID, jupiterErr))
	} else {
		// Jupiter quote succeeded — build and submit the real swap
		if err := execSvc.ValidateQuote(quote); err != nil {
			return e.failExecutionJob(jobID, fmt.Sprintf("execution job %d blocked by quote guardrails: %v", jobID, err))
		}
		swapTx, err := jupiter.BuildSwapTransaction(jupiter.SwapRequest{
			QuoteResponse:           *quote,
			UserPublicKey:           e.executor.WalletAddress().String(),
			WrapAndUnwrapSOL:        false,
			UseSharedAccounts:       true,
			DynamicComputeUnitLimit: true,
			AsLegacyTransaction:     false,
			SourceTokenAccount:      sourceCustody,
			DestinationTokenAccount: targetCustody,
			PrioritizationFee:       "auto",
		})
		if err != nil {
			return e.failExecutionJob(jobID, fmt.Sprintf("execution job %d failed while building the Jupiter swap transaction: %v", jobID, err))
		}
		job.QuoteOutAmount = quote.OutAmount
		job.MinOutAmount = quote.OtherAmountThreshold
		job.PriceImpactPct = quote.PriceImpactPct
		submission, err := execSvc.SubmitAndReconcile(ctx, &job, targetCustody, swapTx.SwapTransaction, quote.OtherAmountThreshold)
		if err != nil {
			return e.failExecutionJob(jobID, fmt.Sprintf("execution job %d failed during autonomous swap lifecycle: %v", jobID, err))
		}
		swapSig = submission.SwapSig
		settleAmount = submission.TargetDelta
	}

	e.mu.Lock()
	e.lastExecSig = swapSig
	e.lastExecStatus = "executed"
	e.lastExecNote = fmt.Sprintf("AI confirmed and reconciled autonomous execution for %s -> %s.", fromSymbol, toSymbol)
	e.mu.Unlock()

	if !e.cfg.ExecutionAutoSettle {
		return nil
	}
	settlementSig, err := e.executor.SendDeposit(ctx, settleAmount, targetCustody, uint8(d.ToIndex))
	if err != nil {
		return e.failExecutionJob(jobID, fmt.Sprintf("execution job %d could not settle target custody funds back into treasury: %v", jobID, err))
	}

	jobRow, err := e.store.ExecutionJobByID(jobID)
	if err != nil {
		return fmt.Errorf("execution job %d settled, but could not be reloaded for persistence: %w", jobID, err)
	}
	jobRow.Stage = "settled_back_to_treasury"
	jobRow.SettlementSig = settlementSig
	jobRow.SettledAmount = settleAmount
	jobRow.Note = fmt.Sprintf("AI completed autonomous execution and settled %s back into treasury.", toSymbol)
	if err := e.store.UpdateExecutionJob(*jobRow); err != nil {
		return fmt.Errorf("execution job %d settled, but persistence update failed: %w", jobID, err)
	}
	if err := e.store.SaveRebalance(d.FromIndex, d.ToIndex, settleAmount, settlementSig, score.RiskLevel); err != nil {
		log.Printf("[pipeline] save rebalance after settlement failed: %v", err)
	}

	e.mu.Lock()
	e.lastExecSig = settlementSig
	e.lastExecStatus = "executed"
	e.lastExecNote = fmt.Sprintf("AI completed autonomous execution for %s -> %s and settled the output back into treasury.", fromSymbol, toSymbol)
	e.mu.Unlock()
	if e.alerter != nil {
		e.alerter.Send("execution_settled", alerts.LevelInfo,
			fmt.Sprintf("💸 Autonomous execution settled\nRoute: *%s → %s*\nAmount: *%d* base units\nSettlement tx: `%s`",
				fromSymbol, toSymbol, settleAmount, settlementSig))
	}
	return nil
}

func (e *Engine) failExecutionJob(jobID int64, note string) error {
	if err := execution.New(e.executor, e.cfg, e.store).MarkFailed(jobID, note); err != nil && e.store != nil {
		log.Printf("[pipeline] failed to persist execution job %d failure note: %v", jobID, err)
	}
	return fmt.Errorf("%s", note)
}

func (e *Engine) recordDecisionTrace(ctx context.Context, d *ai.FinalDecision, score risk.ScoreV2) (string, error) {
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
	return e.executor.SendRecordDecision(ctx, actionCode, rationale, confidence)
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

func (e *Engine) LastPolicyEval() policy.Evaluation {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastPolicyEval
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
func (e *Engine) handleYield(ctx context.Context, score risk.ScoreV2, decision *ai.FinalDecision, eval policy.Evaluation) {
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

		if strategyTokenAccount := e.strategyTokenAccount(pos.Token); strategyTokenAccount != "" {
			tokenIndex, ok := tokenIndexBySymbol(pos.Token)
			if !ok {
				log.Printf("[yield] EXIT blocked: token %s is not registered in ActiveFeeds", pos.Token)
				e.mu.Lock()
				e.lastExecSig = ""
				e.lastExecStatus = "failed"
				e.lastExecNote = "Yield exit failed: token is not mapped to a vault slot."
				e.mu.Unlock()
				return
			}
			liveYieldEnabled, liveYieldReason := e.liveYieldMode()
			if kvault := e.kaminoVault(pos.Token); kvault != "" && pos.Protocol == string(yield.ProtocolKamino) && liveYieldEnabled {
				txB64, err := yield.BuildKaminoWithdrawTx(ctx, e.executor.WalletAddress().String(), kvault, maxUint64Decimal)
				if err != nil {
					log.Printf("[yield] EXIT Kamino withdraw build failed: %v", err)
					e.mu.Lock()
					e.lastExecSig = ""
					e.lastExecStatus = "failed"
					e.lastExecNote = "Yield exit failed while building the Kamino withdraw transaction."
					e.mu.Unlock()
					return
				}
				sig, err := e.executor.SendExternalTransaction(ctx, txB64)
				if err != nil {
					log.Printf("[yield] EXIT Kamino withdraw failed: %v", err)
					e.mu.Lock()
					e.lastExecSig = ""
					e.lastExecStatus = "failed"
					e.lastExecNote = "Yield exit failed while withdrawing from Kamino."
					e.mu.Unlock()
					return
				}
				log.Printf("[yield] EXIT Kamino withdraw tx: %s", sig)
			} else if kvault != "" && pos.Protocol == string(yield.ProtocolKamino) && !liveYieldEnabled {
				log.Printf("[yield] EXIT live Kamino withdraw skipped: %s", liveYieldReason)
			}

			strategyBalance, err := e.executor.TokenAccountBalance(ctx, strategyTokenAccount)
			if err != nil {
				log.Printf("[yield] EXIT strategy balance fetch failed: %v", err)
				e.mu.Lock()
				e.lastExecSig = ""
				e.lastExecStatus = "failed"
				e.lastExecNote = "Yield exit failed while reading the trusted strategy account balance."
				e.mu.Unlock()
				return
			}
			if strategyBalance == 0 {
				log.Printf("[yield] EXIT skipped: strategy account balance is zero")
			} else {
				sig, err := e.executor.SendDeposit(ctx, strategyBalance, strategyTokenAccount, tokenIndex)
				if err != nil {
					log.Printf("[yield] EXIT deposit-back failed: %v", err)
					e.mu.Lock()
					e.lastExecSig = ""
					e.lastExecStatus = "failed"
					e.lastExecNote = "Yield exit failed while returning funds from the strategy account back into the vault."
					e.mu.Unlock()
					return
				}
				e.mu.Lock()
				e.lastExecSig = sig
				e.lastExecStatus = "executed"
				e.lastExecNote = fmt.Sprintf("Yield exit returned %s from the trusted strategy account back into the vault.", pos.Token)
				e.mu.Unlock()
				log.Printf("[yield] EXIT return tx: %s", sig)
			}
		}

		withdrawSig := "simulated-exit"
		if e.lastExecStatus == "executed" {
			withdrawSig = e.lastExecSig
		}
		if err := e.store.CloseYieldPosition(pos.ID, earned, withdrawSig); err != nil {
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
	if eval.ActionClass != policy.ActionClassYieldAllocate {
		return
	}
	if eval.Verdict != policy.VerdictAllowed {
		e.mu.Lock()
		e.lastExecSig = ""
		if eval.Verdict == policy.VerdictRequiresApproval {
			e.lastExecStatus = "approval_required"
		} else {
			e.lastExecStatus = "blocked_by_policy"
		}
		e.lastExecNote = eval.Reason
		e.mu.Unlock()
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
	liveYieldEnabled, liveYieldReason := e.liveYieldMode()
	if !liveYieldEnabled {
		e.mu.Lock()
		e.lastExecSig = ""
		e.lastExecStatus = "signal_only"
		e.lastExecNote = fmt.Sprintf("YIELD_MAX found an opportunity in %s, but live yield execution is blocked: %s", best.DisplayName, liveYieldReason)
		e.mu.Unlock()
		return
	}

	amount := e.cfg.YieldDepositAmount
	amountBaseUnits := uint64(amount * 1_000_000)
	tokenIndex, ok := tokenIndexBySymbol(best.Token)
	if !ok {
		log.Printf("[yield] token %s is not mapped to a vault slot", best.Token)
		return
	}
	strategyTokenAccount := e.strategyTokenAccount(best.Token)
	if strategyTokenAccount == "" {
		log.Printf("[yield] no trusted strategy token account configured for %s", best.Token)
		e.mu.Lock()
		e.lastExecSig = ""
		e.lastExecStatus = "blocked_by_policy"
		e.lastExecNote = fmt.Sprintf("YIELD_MAX selected %s, but no trusted strategy account is configured for that token.", best.Token)
		e.mu.Unlock()
		return
	}

	depositSig, err := e.executor.SendPayment(ctx, amountBaseUnits, strategyTokenAccount, tokenIndex)
	if err != nil {
		log.Printf("[yield] send_payment failed: %v", err)
		e.mu.Lock()
		e.lastExecSig = ""
		e.lastExecStatus = "failed"
		e.lastExecNote = "Yield entry failed while transferring funds into the trusted strategy account."
		e.mu.Unlock()
		return
	}

	e.mu.Lock()
	e.lastExecSig = depositSig
	e.lastExecStatus = "executed"
	e.lastExecNote = fmt.Sprintf("Yield entry moved %s into the trusted strategy account for autonomous deployment.", best.Token)
	e.mu.Unlock()

	liveDepositSig := depositSig
	if kvault := e.kaminoVault(best.Token); kvault != "" && best.Protocol == yield.ProtocolKamino && liveYieldEnabled {
		txB64, err := yield.BuildKaminoDepositTx(ctx, e.executor.WalletAddress().String(), kvault, fmt.Sprintf("%.6f", amount))
		if err != nil {
			log.Printf("[yield] Kamino deposit build failed: %v", err)
			e.mu.Lock()
			e.lastExecStatus = "failed"
			e.lastExecNote = "Yield entry transferred funds, but Kamino deposit transaction build failed."
			e.mu.Unlock()
			return
		}
		kaminoSig, err := e.executor.SendExternalTransaction(ctx, txB64)
		if err != nil {
			log.Printf("[yield] Kamino deposit failed: %v", err)
			e.mu.Lock()
			e.lastExecStatus = "failed"
			e.lastExecNote = "Yield entry transferred funds, but live Kamino deposit failed."
			e.mu.Unlock()
			return
		}
		liveDepositSig = kaminoSig
		e.mu.Lock()
		e.lastExecSig = kaminoSig
		e.lastExecStatus = "executed"
		e.lastExecNote = fmt.Sprintf("Yield entry deposited %s into Kamino automatically.", best.Token)
		e.mu.Unlock()
		log.Printf("[yield] Kamino deposit tx: %s", kaminoSig)
	}

	posID, err := e.store.SaveYieldPosition(
		string(best.Protocol),
		best.Token,
		amount,
		best.SupplyAPY,
		liveDepositSig,
	)
	if err != nil {
		log.Printf("[yield] save position failed: %v", err)
		return
	}

	e.mu.Lock()
	e.activeYieldPositionID = posID
	e.mu.Unlock()

	log.Printf("[yield] ENTER: transferred %.0f %s into trusted strategy account for %s @ %.2f%% APY estimate (posID=%d sig=%s)",
		amount, best.Token, best.DisplayName, best.SupplyAPY, posID, depositSig)

	if e.alerter != nil {
		e.alerter.Send("yield_enter", alerts.LevelInfo,
			fmt.Sprintf("💰 Yield position opened\nProtocol: *%s* | Token: %s\nAmount: *$%.0f* @ *%.2f%%* APY\nRisk: %.0f (safe to enter)",
				best.DisplayName, best.Token, amount, best.SupplyAPY, score.RiskLevel))
	}
}

func (e *Engine) strategyTokenAccount(token string) string {
	if e.cfg == nil {
		return ""
	}
	switch token {
	case "USDC":
		return e.cfg.YieldStrategyUSDCAccount
	case "USDT":
		return e.cfg.YieldStrategyUSDTAccount
	case "DAI":
		return e.cfg.YieldStrategyDAIAccount
	case "PYUSD":
		return e.cfg.YieldStrategyPYUSDAccount
	default:
		return ""
	}
}

func (e *Engine) kaminoVault(token string) string {
	if e.cfg == nil {
		return ""
	}
	switch token {
	case "USDC":
		return e.cfg.YieldKaminoUSDCVault
	case "USDT":
		return e.cfg.YieldKaminoUSDTVault
	case "DAI":
		return e.cfg.YieldKaminoDAIVault
	case "PYUSD":
		return e.cfg.YieldKaminoPYUSDVault
	default:
		return ""
	}
}

func (e *Engine) liveYieldMode() (bool, string) {
	if e.cfg == nil {
		return false, "runtime config is not attached"
	}
	if currentControlMode(e.cfg) != "YIELD_MAX" {
		return false, "live yield execution is only enabled in YIELD_MAX control mode"
	}
	if !e.cfg.YieldEnabled {
		return false, "yield mode is disabled"
	}
	readiness := e.cfg.YieldExecutionReadiness()
	if !readiness.ReadyForLive {
		return false, readiness.Note
	}
	return true, ""
}

func currentControlMode(cfg *config.Config) string {
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

func tokenIndexBySymbol(symbol string) (uint8, bool) {
	for _, feed := range pyth.ActiveFeeds {
		if feed.Symbol == symbol && feed.VaultSlot >= 0 {
			return uint8(feed.VaultSlot), true
		}
	}
	return 0, false
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

// autoRebalanceOnChain submits execute_rebalance on-chain whenever the AI
// decides PROTECT or OPTIMIZE. This is purely vault accounting — no Jupiter
// swap required — so it works on devnet and mainnet alike.
func (e *Engine) autoRebalanceOnChain(ctx context.Context, d *ai.FinalDecision, balances []uint64) {
	if d.FromIndex < 0 || d.ToIndex < 0 {
		return
	}
	var fromBal uint64
	if d.FromIndex < len(balances) {
		fromBal = balances[d.FromIndex]
	}
	if fromBal == 0 {
		log.Printf("[pipeline:rebalance] skipped — from_balance[%d] is zero", d.FromIndex)
		return
	}
	fraction := d.SuggestedFraction
	if fraction <= 0 {
		fraction = 0.10 // default 10% if AI didn't specify
	}
	amount := uint64(float64(fromBal) * fraction)
	if amount == 0 {
		return
	}

	ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	sig, err := e.executor.ExecuteRebalance(ctx2, uint8(d.FromIndex), uint8(d.ToIndex), amount)
	if err != nil {
		log.Printf("[pipeline:rebalance] execute_rebalance failed: %v", err)
		return
	}
	log.Printf("[pipeline:rebalance] execute_rebalance OK: from=%d to=%d amount=%d sig=%s",
		d.FromIndex, d.ToIndex, amount, sig)

	e.mu.Lock()
	e.lastExecSig = sig
	e.lastExecStatus = "executed"
	e.lastExecNote = fmt.Sprintf("AI %s: rebalanced %d units from slot %d → slot %d", d.Action, amount, d.FromIndex, d.ToIndex)
	e.mu.Unlock()

	// Save to history store if available
	if e.store != nil {
		_ = e.store.SaveRebalance(d.FromIndex, d.ToIndex, amount, sig, 0)
	}
}
