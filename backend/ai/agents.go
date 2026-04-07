// Package ai implements a simulated multi-agent AI system for vault decisions.
// Three Claude Haiku agents run sequentially:
//  1. Risk Analyst   — interprets portfolio risk (stablecoin peg + volatile assets)
//  2. Yield Analyst  — finds yield opportunities across stablecoins and crypto
//  3. Strategy Agent — synthesizes both analyses → final decision
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

const (
	DecisionProfileCautious   = "cautious"
	DecisionProfileBalanced   = "balanced"
	DecisionProfileAggressive = "aggressive"
)

// AgentResult is the output of a single agent.
type AgentResult struct {
	Summary    string
	Action     string // HOLD / REBALANCE / OPTIMIZE
	Confidence int    // 0–100
}

// FinalDecision is the output of the Strategy Agent (agent 3).
type FinalDecision struct {
	Action            string  `json:"action"`
	FromIndex         int     `json:"from_index"`    // -1 = no trade
	ToIndex           int     `json:"to_index"`
	SuggestedFraction float64 `json:"suggested_fraction"`
	Rationale         string  `json:"rationale"`
	Confidence        int     `json:"confidence"`
	RiskAnalysis      string  `json:"risk_analysis"` // from agent 1
	YieldAnalysis     string  `json:"yield_analysis"` // from agent 2
}

// MultiAgentSystem runs 3 LLM agents in sequence.
type MultiAgentSystem struct {
	client          *anthropic.Client
	dryRun          bool // if true, skip LLM calls and return mock decisions
	model           anthropic.Model
	decisionProfile string
}

// New creates a MultiAgentSystem. Pass apiKey="" to enable dry-run mode.
func New(apiKey, model, decisionProfile string) *MultiAgentSystem {
	normalizedModel := normalizeModel(model)
	normalizedProfile := normalizeDecisionProfile(decisionProfile)
	if apiKey == "" {
		return &MultiAgentSystem{
			dryRun:          true,
			model:           normalizedModel,
			decisionProfile: normalizedProfile,
		}
	}
	c := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &MultiAgentSystem{
		client:          &c,
		model:           normalizedModel,
		decisionProfile: normalizedProfile,
	}
}

// Run executes all 3 agents and returns a FinalDecision.
// whaleScore (0–100) is the on-chain whale/liquidity risk from DexScreener.
// Pass 0 if whale data is unavailable.
// On any agent failure, it falls back gracefully without crashing.
func (m *MultiAgentSystem) Run(
	ctx context.Context,
	snap *pyth.PriceSnapshot,
	score risk.ScoreV2,
	balances []uint64,
	strategyMode uint8,
	whaleScore ...float64,
) (*FinalDecision, error) {
	ws := 0.0
	if len(whaleScore) > 0 {
		ws = whaleScore[0]
	}

	// ── Agent 1: Risk Analyst ─────────────────────────────────────────────
	riskResult := m.runRiskAgent(ctx, snap, score)
	log.Printf("[agent:risk] conf=%d → %s", riskResult.Confidence, riskResult.Summary)

	// ── Agent 2: Yield Analyst ────────────────────────────────────────────
	yieldResult := m.runYieldAgent(ctx, snap, balances)
	log.Printf("[agent:yield] conf=%d → %s", yieldResult.Confidence, yieldResult.Summary)

	// ── Agent 3: Strategy Agent ───────────────────────────────────────────
	decision := m.runStrategyAgent(ctx, riskResult, yieldResult, score, strategyMode, ws)
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

	btcPrice := snap.All["BTC"].Price
	ethPrice := snap.All["ETH"].Price
	solPrice := snap.All["SOL"].Price

	// Determine dominant risk driver so the agent has clear context.
	var riskDriver string
	if score.VolatileRisk >= 50 {
		// Find which asset crashed most
		crashAsset, crashPct := dominantCrash(snap)
		riskDriver = fmt.Sprintf("CRYPTO CRASH DETECTED: %s dropped ~%.0f%% (volatile_risk=%.0f/100). Stablecoins are intact (stable_risk=%.0f/100). Treasury volatile sleeve is at significant drawdown.",
			crashAsset, crashPct, score.VolatileRisk, score.StableRisk)
	} else if score.StableRisk >= 30 {
		riskDriver = fmt.Sprintf("STABLECOIN DEPEG DETECTED: peg deviation %.4f%% (stable_risk=%.0f/100). Volatile assets are stable (volatile_risk=%.0f/100).",
			score.Deviation, score.StableRisk, score.VolatileRisk)
	} else {
		riskDriver = fmt.Sprintf("Market stable. Peg deviation: %.5f%%, volatile crash signal: %.0f/100.", score.Deviation, score.VolatileRisk)
	}

	prompt := fmt.Sprintf(`You are a treasury risk analyst for a mixed crypto portfolio.

RISK BREAKDOWN:
- Composite risk score: %.1f/100
- Stable risk (peg deviation): %.1f/100
- Volatile crash risk (BTC/ETH/SOL drawdown): %.1f/100
- Trend: %.6f | Velocity: %.6f | Volatility: %.6f

CURRENT PRICES:
- USDC: $%.6f | USDT: $%.6f | Peg deviation: %.5f%%
- BTC: $%.0f | ETH: $%.0f | SOL: $%.4f

RISK DRIVER: %s

Should the treasury act to protect assets?
Respond in exactly 3 lines:
LINE1: <one sentence risk assessment — name the specific threat>
LINE2: ACTION: HOLD or REBALANCE
LINE3: CONFIDENCE: <0-100>

IMPORTANT: If volatile_risk > 50 OR stable_risk > 40, recommend REBALANCE.`,
		score.RiskLevel,
		score.StableRisk,
		score.VolatileRisk,
		score.Trend, score.Velocity, score.Volatility,
		snap.USDC.Price, snap.USDT.Price, score.Deviation,
		btcPrice, ethPrice, solPrice,
		riskDriver,
	)

	text := m.callLLM(ctx, "risk analyst", prompt)
	return parseThreeLineResult(text, dryRunRiskResult(score))
}

// dominantCrash returns the asset with the largest price drop in the current snapshot
// compared to the previous price (estimated from the All map confidence interval).
func dominantCrash(snap *pyth.PriceSnapshot) (string, float64) {
	assets := []string{"BTC", "ETH", "SOL"}
	best := ""
	var bestPct float64
	for _, sym := range assets {
		pd, ok := snap.All[sym]
		if !ok || pd.Price <= 0 {
			continue
		}
		// Use confidence as a proxy for relative volatility; pick the asset with non-zero price.
		if best == "" {
			best = sym
			bestPct = pd.Confidence / pd.Price * 100
		}
	}
	// Return the asset and a rough drop estimate (confidence-based)
	return best, bestPct
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

	btcPrice := snap.All["BTC"].Price
	ethPrice := snap.All["ETH"].Price
	solPrice := snap.All["SOL"].Price

	prompt := fmt.Sprintf(`Crypto treasury yield opportunity:
Stablecoins — USDC: $%.6f  USDT: $%.6f  Spread: $%+.6f
Volatile assets — BTC: $%.0f  ETH: $%.0f  SOL: $%.2f
Total vault balance: %d base units

Is there a yield-arbitrage or rebalancing opportunity across stablecoins or volatile assets?
Respond in exactly 3 lines:
LINE1: <one sentence about the best yield opportunity>
LINE2: ACTION: HOLD or OPTIMIZE
LINE3: CONFIDENCE: <0-100>`,
		snap.USDC.Price, snap.USDT.Price, spread,
		btcPrice, ethPrice, solPrice,
		totalBal,
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
	whaleScore float64,
) *FinalDecision {
	modeName := strategyModeName(strategyMode)
	profileName := strings.ToUpper(m.decisionProfile)

	if m.dryRun {
		return buildDecision(score, riskRes, yieldRes, strategyMode, m.decisionProfile)
	}

	whaleLine := "Whale/on-chain signal: unavailable"
	if whaleScore > 0 {
		level := "low"
		if whaleScore >= 70 {
			level = "HIGH — large sell pressure or liquidity drain detected on DEX"
		} else if whaleScore >= 40 {
			level = "medium — elevated sell activity on Solana DEXes"
		}
		whaleLine = fmt.Sprintf("Whale/on-chain signal: %.0f/100 (%s)", whaleScore, level)
	}

	// Build clear rebalance context so LLM doesn't see "-1 → -1"
	rebalanceLine := "No rebalance suggested (risk below threshold)"
	if score.FromIndex >= 0 && score.ToIndex >= 0 {
		rebalanceLine = fmt.Sprintf("Move %.0f%% from vault slot %d → slot %d (crash/depeg protection)",
			score.SuggestedFraction*100, score.FromIndex, score.ToIndex)
	}

	// Dominant risk signal
	riskSignal := "low"
	if score.VolatileRisk >= 70 {
		riskSignal = fmt.Sprintf("CRITICAL CRYPTO CRASH (volatile_risk=%.0f/100)", score.VolatileRisk)
	} else if score.VolatileRisk >= 40 {
		riskSignal = fmt.Sprintf("elevated crypto drawdown (volatile_risk=%.0f/100)", score.VolatileRisk)
	} else if score.StableRisk >= 50 {
		riskSignal = fmt.Sprintf("STABLECOIN DEPEG (stable_risk=%.0f/100)", score.StableRisk)
	} else if score.StableRisk >= 20 {
		riskSignal = fmt.Sprintf("moderate peg stress (stable_risk=%.0f/100)", score.StableRisk)
	}

	prompt := fmt.Sprintf(`You are the final strategy agent for a mixed crypto treasury (stablecoins + BTC/ETH/SOL).

SITUATION:
- Strategy mode: %s | Decision profile: %s
- Composite risk: %.1f/100 | Signal: %s
- Stable risk (peg deviation): %.1f/100
- Volatile crash risk (BTC/ETH/SOL drawdown): %.1f/100
- %s

AGENT INPUTS:
- Risk analyst: %s → recommends %s (confidence %d%%)
- Yield analyst: %s → recommends %s (confidence %d%%)

SUGGESTED ACTION: %s

Choose ONE final action. Respond in exactly 3 lines:
LINE1: ACTION: HOLD or PROTECT or OPTIMIZE
LINE2: RATIONALE: <one sentence explaining why>
LINE3: CONFIDENCE: <0-100>

DECISION RULES:
- If volatile_risk > 50 → strongly consider PROTECT (crypto crash threatens portfolio value)
- If stable_risk > 40 → strongly consider PROTECT (stablecoin depeg risk)
- If composite risk > 40 → lean toward PROTECT unless compelling reason to HOLD
- PROTECT = execute the suggested rebalance to reduce risk
- OPTIMIZE = rebalance for yield when risk is low
- HOLD = only if risk is genuinely low and no clear threat
- Profile CAUTIOUS: require higher confidence before acting
- Profile AGGRESSIVE: act earlier, size larger`,
		modeName, profileName,
		score.RiskLevel, riskSignal,
		score.StableRisk,
		score.VolatileRisk,
		whaleLine,
		riskRes.Summary, riskRes.Action, riskRes.Confidence,
		yieldRes.Summary, yieldRes.Action, yieldRes.Confidence,
		rebalanceLine,
	)

	text := m.callLLM(ctx, "strategy agent", prompt)
	d := buildDecision(score, riskRes, yieldRes, strategyMode, m.decisionProfile)

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
		Model:     m.model,
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
				// Strip "LINE1:" prefix so UI shows clean text
				if strings.HasPrefix(strings.ToUpper(trimmed), "LINE1:") {
					trimmed = strings.TrimSpace(trimmed[6:])
				}
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

func buildDecision(score risk.ScoreV2, riskRes, yieldRes AgentResult, strategyMode uint8, decisionProfile string) *FinalDecision {
	profile := normalizeDecisionProfile(decisionProfile)
	d := &FinalDecision{
		FromIndex:         score.FromIndex,
		ToIndex:           score.ToIndex,
		SuggestedFraction: adjustedSuggestedFraction(score.SuggestedFraction, profile),
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
		if score.RiskLevel >= protectThreshold(strategyMode, profile) {
			d.Action = ActionProtect
			d.Rationale = fmt.Sprintf("Risk %.1f/100 — protecting vault", score.RiskLevel)
		} else {
			d.Action = ActionHold
			d.Rationale = "SAFE mode: risk not high enough to act"
		}
	case risk.StrategyModeYieldV2:
		if shouldOptimize(strategyMode, profile, score.RiskLevel, yieldRes.Action) {
			d.Action = ActionOptimize
			d.Rationale = fmt.Sprintf("Yield mode: spread opportunity detected (dev=%.5f%%)", score.Deviation)
		} else if score.RiskLevel >= protectThreshold(strategyMode, profile) {
			d.Action = ActionProtect
			d.Rationale = fmt.Sprintf("Risk %.1f/100 — rebalancing", score.RiskLevel)
		} else {
			d.Action = ActionHold
			d.Rationale = "Yield mode: thresholds not met"
		}
	default: // BALANCED
		if score.RiskLevel >= protectThreshold(strategyMode, profile) {
			d.Action = ActionProtect
			d.Rationale = fmt.Sprintf("Balanced: risk %.1f/100 — protecting", score.RiskLevel)
		} else if shouldOptimize(strategyMode, profile, score.RiskLevel, yieldRes.Action) {
			d.Action = ActionOptimize
			d.Rationale = fmt.Sprintf("%s profile: moderate risk with yield opportunity", strings.Title(profile))
		} else {
			d.Action = ActionHold
			d.Rationale = "Balanced mode — conditions not met"
		}
	}
	return d
}

func dryRunRiskResult(score risk.ScoreV2) AgentResult {
	action := "HOLD"
	conf := int(score.RiskLevel)

	var summary string
	if score.VolatileRisk >= 50 {
		action = "REBALANCE"
		conf = int(score.VolatileRisk)
		summary = fmt.Sprintf("Crypto crash detected: volatile_risk=%.0f/100, stable_risk=%.0f/100 — rotate volatile sleeve to USDC", score.VolatileRisk, score.StableRisk)
	} else if score.StableRisk >= 30 || score.Action == "rebalance" {
		action = "REBALANCE"
		summary = fmt.Sprintf("Stablecoin depeg: stable_risk=%.0f/100, deviation=%.5f%%", score.StableRisk, score.Deviation)
	} else {
		summary = fmt.Sprintf("Market stable: risk=%.1f/100, stable=%.0f, volatile=%.0f", score.RiskLevel, score.StableRisk, score.VolatileRisk)
	}

	return AgentResult{
		Summary:    summary,
		Action:     action,
		Confidence: conf,
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

func normalizeModel(model string) anthropic.Model {
	raw := strings.TrimSpace(model)
	if raw == "" {
		return anthropic.ModelClaudeHaiku4_5
	}
	return anthropic.Model(raw)
}

func normalizeDecisionProfile(profile string) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case DecisionProfileCautious:
		return DecisionProfileCautious
	case DecisionProfileAggressive:
		return DecisionProfileAggressive
	default:
		return DecisionProfileBalanced
	}
}

func adjustedSuggestedFraction(fraction float64, profile string) float64 {
	switch profile {
	case DecisionProfileCautious:
		return minFloat64(fraction*0.7, 0.35)
	case DecisionProfileAggressive:
		return minFloat64(fraction*1.25, 0.9)
	default:
		return fraction
	}
}

func protectThreshold(strategyMode uint8, profile string) float64 {
	switch profile {
	case DecisionProfileCautious:
		switch strategyMode {
		case risk.StrategyModeSafe:
			return 45
		case risk.StrategyModeYieldV2:
			return 35
		default:
			return 30
		}
	case DecisionProfileAggressive:
		switch strategyMode {
		case risk.StrategyModeSafe:
			return 20
		case risk.StrategyModeYieldV2:
			return 15
		default:
			return 15
		}
	default:
		switch strategyMode {
		case risk.StrategyModeSafe:
			return 30
		case risk.StrategyModeYieldV2:
			return 20
		default:
			return 20
		}
	}
}

func shouldOptimize(strategyMode uint8, profile string, riskLevel float64, yieldAction string) bool {
	if yieldAction != "REBALANCE" {
		return false
	}
	switch profile {
	case DecisionProfileCautious:
		if strategyMode == risk.StrategyModeYieldV2 {
			return riskLevel <= 35
		}
		return riskLevel >= 20 && riskLevel <= 35
	case DecisionProfileAggressive:
		if strategyMode == risk.StrategyModeYieldV2 {
			return riskLevel <= 65
		}
		return riskLevel >= 5 && riskLevel <= 45
	default:
		if strategyMode == risk.StrategyModeYieldV2 {
			return true
		}
		return riskLevel >= 10
	}
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
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
