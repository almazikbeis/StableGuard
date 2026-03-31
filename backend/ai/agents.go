// Package ai implements a simulated multi-agent AI system for vault decisions.
// Three Claude Haiku agents run sequentially:
//   1. Risk Analyst   — interprets the risk score
//   2. Yield Analyst  — finds yield opportunities in stablecoin spreads
//   3. Strategy Agent — synthesizes both analyses → final decision
package ai

import (
	"context"
	"fmt"
	"log"
	"strings"

	"stableguard-backend/pyth"
	"stableguard-backend/risk"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Action constants returned by the Strategy Agent.
const (
	ActionHold     = "HOLD"
	ActionProtect  = "PROTECT"  // risk-driven rebalance
	ActionOptimize = "OPTIMIZE" // yield-driven rebalance
)

// AgentResult is the output of a single agent.
type AgentResult struct {
	Summary    string
	Action     string // HOLD / REBALANCE / OPTIMIZE
	Confidence int    // 0–100
}

// FinalDecision is the output of the Strategy Agent (agent 3).
type FinalDecision struct {
	Action            string // HOLD | PROTECT | OPTIMIZE
	FromIndex         int    // -1 = no trade
	ToIndex           int
	SuggestedFraction float64
	Rationale         string
	Confidence        int
	RiskAnalysis      string // from agent 1
	YieldAnalysis     string // from agent 2
}

// MultiAgentSystem runs 3 LLM agents in sequence.
type MultiAgentSystem struct {
	client *anthropic.Client
	dryRun bool // if true, skip LLM calls and return mock decisions
}

// New creates a MultiAgentSystem. Pass apiKey="" to enable dry-run mode.
func New(apiKey string) *MultiAgentSystem {
	if apiKey == "" {
		return &MultiAgentSystem{dryRun: true}
	}
	c := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &MultiAgentSystem{client: &c}
}

// Run executes all 3 agents and returns a FinalDecision.
// On any agent failure, it falls back gracefully without crashing.
func (m *MultiAgentSystem) Run(
	ctx context.Context,
	snap *pyth.PriceSnapshot,
	score risk.ScoreV2,
	balances []uint64,
	strategyMode uint8,
) (*FinalDecision, error) {
	// ── Agent 1: Risk Analyst ─────────────────────────────────────────────
	riskResult := m.runRiskAgent(ctx, snap, score)
	log.Printf("[agent:risk] conf=%d → %s", riskResult.Confidence, riskResult.Summary)

	// ── Agent 2: Yield Analyst ────────────────────────────────────────────
	yieldResult := m.runYieldAgent(ctx, snap, balances)
	log.Printf("[agent:yield] conf=%d → %s", yieldResult.Confidence, yieldResult.Summary)

	// ── Agent 3: Strategy Agent ───────────────────────────────────────────
	decision := m.runStrategyAgent(ctx, riskResult, yieldResult, score, strategyMode)
	log.Printf("[agent:strategy] action=%s from=%d to=%d frac=%.2f conf=%d",
		decision.Action, decision.FromIndex, decision.ToIndex,
		decision.SuggestedFraction, decision.Confidence)

	return decision, nil
}

// ── Agent 1: Risk Analyst ─────────────────────────────────────────────────────

func (m *MultiAgentSystem) runRiskAgent(ctx context.Context, snap *pyth.PriceSnapshot, score risk.ScoreV2) AgentResult {
	if m.dryRun {
		return dryRunRiskResult(score)
	}

	prompt := fmt.Sprintf(`Stablecoin risk snapshot:
USDC price: $%.6f  USDT price: $%.6f
Deviation: %.5f%%  Risk score: %.1f/100
Trend (slope): %.6f  Velocity: %.6f  Volatility: %.6f
Window: %d observations

Is this a real depeg event or noise? Respond in 2 lines:
LINE1: <one sentence analysis>
LINE2: ACTION: HOLD or REBALANCE
LINE3: CONFIDENCE: <0-100>`,
		snap.USDC.Price, snap.USDT.Price,
		score.Deviation, score.RiskLevel,
		score.Trend, score.Velocity, score.Volatility,
		score.WindowSize,
	)

	text := m.callLLM(ctx, "risk analyst", prompt)
	return parseThreeLineResult(text, dryRunRiskResult(score))
}

// ── Agent 2: Yield Analyst ────────────────────────────────────────────────────

func (m *MultiAgentSystem) runYieldAgent(ctx context.Context, snap *pyth.PriceSnapshot, balances []uint64) AgentResult {
	if m.dryRun {
		return dryRunYieldResult(snap)
	}

	var totalBal uint64
	for _, b := range balances {
		totalBal += b
	}

	spread := snap.USDC.Price - snap.USDT.Price

	prompt := fmt.Sprintf(`Stablecoin yield opportunity:
USDC price: $%.6f  USDT price: $%.6f
Spread: $%+.6f (positive = USDC expensive)
Total vault balance: %d base units

Is there a yield-arbitrage opportunity? Respond in 3 lines:
LINE1: <one sentence about yield opportunity>
LINE2: ACTION: HOLD or OPTIMIZE
LINE3: CONFIDENCE: <0-100>`,
		snap.USDC.Price, snap.USDT.Price, spread, totalBal,
	)

	text := m.callLLM(ctx, "yield analyst", prompt)
	return parseThreeLineResult(text, dryRunYieldResult(snap))
}

// ── Agent 3: Strategy Agent ───────────────────────────────────────────────────

func (m *MultiAgentSystem) runStrategyAgent(
	ctx context.Context,
	riskRes, yieldRes AgentResult,
	score risk.ScoreV2,
	strategyMode uint8,
) *FinalDecision {
	modeName := strategyModeName(strategyMode)

	if m.dryRun {
		return buildDecision(score, riskRes, yieldRes, strategyMode)
	}

	prompt := fmt.Sprintf(`You are the final strategy agent for a stablecoin vault.

Strategy mode: %s
Risk analyst says: %s (action=%s, confidence=%d)
Yield analyst says: %s (action=%s, confidence=%d)
Current risk level: %.1f/100
Suggested rebalance: slot %d → slot %d (fraction %.2f)

Choose ONE final action. Respond in exactly 3 lines:
LINE1: ACTION: HOLD or PROTECT or OPTIMIZE
LINE2: RATIONALE: <one sentence>
LINE3: CONFIDENCE: <0-100>

Rules:
- PROTECT = rebalance to reduce depeg risk (risk-driven)
- OPTIMIZE = rebalance for yield arbitrage (spread-driven)
- HOLD = do nothing`,
		modeName,
		riskRes.Summary, riskRes.Action, riskRes.Confidence,
		yieldRes.Summary, yieldRes.Action, yieldRes.Confidence,
		score.RiskLevel,
		score.FromIndex, score.ToIndex, score.SuggestedFraction,
	)

	text := m.callLLM(ctx, "strategy agent", prompt)
	d := buildDecision(score, riskRes, yieldRes, strategyMode)

	// Override action/rationale/confidence from LLM if parse succeeds
	lines := splitLines(text)
	for _, line := range lines {
		lu := strings.ToUpper(line)
		if strings.HasPrefix(lu, "ACTION:") {
			raw := strings.TrimSpace(line[7:])
			switch strings.ToUpper(raw) {
			case ActionHold:
				d.Action = ActionHold
			case ActionProtect:
				d.Action = ActionProtect
			case ActionOptimize:
				d.Action = ActionOptimize
			}
		} else if strings.HasPrefix(strings.ToUpper(line), "RATIONALE:") {
			d.Rationale = strings.TrimSpace(line[10:])
		} else if strings.HasPrefix(strings.ToUpper(line), "CONFIDENCE:") {
			var c int
			if _, err := fmt.Sscanf(strings.TrimSpace(line[11:]), "%d", &c); err == nil {
				if c >= 0 && c <= 100 {
					d.Confidence = c
				}
			}
		}
	}

	// If LLM said HOLD, clear trade indices
	if d.Action == ActionHold {
		d.FromIndex = -1
		d.ToIndex = -1
		d.SuggestedFraction = 0
	}

	return d
}

// ── LLM call ──────────────────────────────────────────────────────────────────

func (m *MultiAgentSystem) callLLM(ctx context.Context, agentName, prompt string) string {
	if m.client == nil {
		return ""
	}
	msg, err := m.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
		MaxTokens: 200,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		log.Printf("[agent:%s] LLM error: %v", agentName, err)
		return ""
	}
	if len(msg.Content) == 0 {
		return ""
	}
	return msg.Content[0].Text
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func parseThreeLineResult(text string, fallback AgentResult) AgentResult {
	if text == "" {
		return fallback
	}
	res := fallback
	for _, line := range splitLines(text) {
		lu := strings.ToUpper(strings.TrimSpace(line))
		if strings.HasPrefix(lu, "LINE1:") || (!strings.HasPrefix(lu, "LINE") && !strings.HasPrefix(lu, "ACTION:") && !strings.HasPrefix(lu, "CONFIDENCE:")) {
			// treat as summary if not a directive line
			if !strings.HasPrefix(lu, "ACTION:") && !strings.HasPrefix(lu, "CONFIDENCE:") {
				trimmed := strings.TrimSpace(line)
				if len(trimmed) > 5 {
					res.Summary = trimmed
				}
			}
		}
		if strings.HasPrefix(lu, "ACTION:") {
			raw := strings.TrimSpace(strings.TrimPrefix(lu, "ACTION:"))
			if strings.Contains(raw, "REBALANCE") || strings.Contains(raw, "OPTIMIZE") {
				res.Action = "REBALANCE"
			} else {
				res.Action = "HOLD"
			}
		}
		if strings.HasPrefix(lu, "CONFIDENCE:") {
			var c int
			if _, err := fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(lu, "CONFIDENCE:")), "%d", &c); err == nil {
				if c >= 0 && c <= 100 {
					res.Confidence = c
				}
			}
		}
	}
	return res
}

func buildDecision(score risk.ScoreV2, riskRes, yieldRes AgentResult, strategyMode uint8) *FinalDecision {
	d := &FinalDecision{
		FromIndex:         score.FromIndex,
		ToIndex:           score.ToIndex,
		SuggestedFraction: score.SuggestedFraction,
		RiskAnalysis:      riskRes.Summary,
		YieldAnalysis:     yieldRes.Summary,
		Confidence:        (riskRes.Confidence + yieldRes.Confidence) / 2,
	}

	// Determine action from risk score + strategy
	if score.Action == "hold" || score.FromIndex < 0 {
		d.Action = ActionHold
		d.Rationale = "Risk below threshold — holding"
		return d
	}

	// Choose PROTECT vs OPTIMIZE by strategy mode
	switch strategyMode {
	case risk.StrategyModeSafe:
		if score.RiskLevel >= 30 {
			d.Action = ActionProtect
			d.Rationale = fmt.Sprintf("Risk %.1f/100 — protecting vault", score.RiskLevel)
		} else {
			d.Action = ActionHold
			d.Rationale = "SAFE mode: risk not high enough to act"
		}
	case risk.StrategyModeYieldV2:
		if yieldRes.Action == "REBALANCE" {
			d.Action = ActionOptimize
			d.Rationale = fmt.Sprintf("Yield mode: spread opportunity detected (dev=%.5f%%)", score.Deviation)
		} else {
			d.Action = ActionProtect
			d.Rationale = fmt.Sprintf("Risk %.1f/100 — rebalancing", score.RiskLevel)
		}
	default: // BALANCED
		if score.RiskLevel >= 20 {
			d.Action = ActionProtect
			d.Rationale = fmt.Sprintf("Balanced: risk %.1f/100 — protecting", score.RiskLevel)
		} else if yieldRes.Action == "REBALANCE" && score.RiskLevel >= 10 {
			d.Action = ActionOptimize
			d.Rationale = "Balanced: moderate risk with yield opportunity"
		} else {
			d.Action = ActionHold
			d.Rationale = "Balanced mode — conditions not met"
		}
	}
	return d
}

func dryRunRiskResult(score risk.ScoreV2) AgentResult {
	action := "HOLD"
	if score.Action == "rebalance" {
		action = "REBALANCE"
	}
	return AgentResult{
		Summary:    fmt.Sprintf("Deviation %.5f%% risk %.1f/100 trend=%.5f", score.Deviation, score.RiskLevel, score.Trend),
		Action:     action,
		Confidence: int(score.RiskLevel),
	}
}

func dryRunYieldResult(snap *pyth.PriceSnapshot) AgentResult {
	spread := snap.USDC.Price - snap.USDT.Price
	action := "HOLD"
	conf := 50
	summary := fmt.Sprintf("Spread $%+.6f — no significant yield opportunity", spread)
	if spread > 0.0001 || spread < -0.0001 {
		action = "REBALANCE"
		conf = 70
		summary = fmt.Sprintf("Spread $%+.6f — sell expensive token for %.4f%% gain", spread, (abs64(spread)/1.0)*100)
	}
	return AgentResult{Summary: summary, Action: action, Confidence: conf}
}

func strategyModeName(mode uint8) string {
	switch mode {
	case risk.StrategyModeSafe:
		return "SAFE"
	case risk.StrategyModeYieldV2:
		return "YIELD"
	default:
		return "BALANCED"
	}
}

func splitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

func abs64(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
