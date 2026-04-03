// Package ai — conversational chat with full portfolio context.
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"stableguard-backend/store"

	anthropic "github.com/anthropics/anthropic-sdk-go"
)

// ChatMessage is one turn in the conversation history.
type ChatMessage struct {
	Role    string `json:"role"`    // "user" | "assistant"
	Content string `json:"content"`
}

// ChatRequest is what the frontend sends.
type ChatRequest struct {
	Message string        `json:"message"`
	History []ChatMessage `json:"history,omitempty"`
}

// ChatAction is an optional executable action parsed from the AI reply.
type ChatAction struct {
	Type    string         `json:"type"`    // "set_strategy", "pause", "set_threshold", "send_payment"
	Params  map[string]any `json:"params"`
	Confirm bool           `json:"confirm"` // true = ask user before executing
	Label   string         `json:"label"`   // human-readable button label
}

// ChatResponse is what the API returns.
type ChatResponse struct {
	Reply   string      `json:"reply"`
	Action  *ChatAction `json:"action,omitempty"`
	TokensUsed int      `json:"tokens_used,omitempty"`
}

// IntentConfig is the result of parsing a natural-language autopilot goal.
type IntentConfig struct {
	StrategyMode    int     `json:"strategy_mode"`    // 0=SAFE 1=BALANCED 2=YIELD
	RiskThreshold   int     `json:"risk_threshold"`
	YieldEntryRisk  float64 `json:"yield_entry_risk"`
	YieldExitRisk   float64 `json:"yield_exit_risk"`
	CircuitBreaker  float64 `json:"circuit_breaker_pct"`
	Explanation     string  `json:"explanation"`
	StrategyName    string  `json:"strategy_name"`
}

// PortfolioContext bundles live state for the system prompt.
type PortfolioContext struct {
	RiskLevel     float64
	RiskSummary   string
	Action        string
	Prices        map[string]float64
	MaxDeviation  float64
	StrategyMode  uint8
	LastDecisions []store.DecisionRow
	YieldProtocol string
	YieldAPY      float64
	YieldEarned   float64
	TotalDecisions int
	TotalRebalances int
}

// Chat sends a conversational message to Claude with full portfolio context.
func (m *MultiAgentSystem) Chat(ctx context.Context, req ChatRequest, pc PortfolioContext) (*ChatResponse, error) {
	sys := buildChatSystemPrompt(pc)

	// Build message history
	msgs := make([]anthropic.MessageParam, 0, len(req.History)+1)
	for _, h := range req.History {
		if h.Role == "user" {
			msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(h.Content)))
		} else {
			msgs = append(msgs, anthropic.NewAssistantMessage(anthropic.NewTextBlock(h.Content)))
		}
	}
	msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(req.Message)))

	if m.dryRun || m.client == nil {
		return &ChatResponse{Reply: dryRunReply(req.Message, pc)}, nil
	}

	resp, err := m.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
		MaxTokens: 1024,
		System:    []anthropic.TextBlockParam{{Text: sys}},
		Messages:  msgs,
	})
	if err != nil {
		return &ChatResponse{Reply: dryRunReply(req.Message, pc)}, nil
	}

	if len(resp.Content) == 0 {
		return &ChatResponse{Reply: "I couldn't process that — please try again."}, nil
	}

	text := resp.Content[0].Text
	action := parseActionFromReply(text)

	return &ChatResponse{
		Reply:      cleanReply(text),
		Action:     action,
		TokensUsed: int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
	}, nil
}

// ParseIntent converts a natural-language goal into vault configuration.
func (m *MultiAgentSystem) ParseIntent(ctx context.Context, intent string) (*IntentConfig, error) {
	sys := `You are StableGuard's intent parser. Convert the user's natural language goal into exact vault configuration.

Always respond ONLY with this JSON — no prose before or after:
{
  "strategy_mode": <0=SAFE, 1=BALANCED, 2=YIELD>,
  "risk_threshold": <integer 5–80, how aggressively to rebalance>,
  "yield_entry_risk": <float, max risk score to enter yield positions, 20–45>,
  "yield_exit_risk": <float, risk score to exit yield, 40–70>,
  "circuit_breaker_pct": <float, depeg % to auto-pause, 0.3–3.0>,
  "strategy_name": "<one-word name: Safe/Balanced/Aggressive>",
  "explanation": "<2 sentences explaining what the vault will do>"
}

Guidelines:
- "max yield", "aggressive", "earn as much" → YIELD (2), low thresholds
- "protect", "safe", "sleep at night", "capital preservation" → SAFE (0)
- "balanced", "moderate", "medium" → BALANCED (1)
- If user mentions a specific depeg %, use that for circuit_breaker_pct
- If user mentions a specific risk level, use that for risk_threshold`

	if m.dryRun || m.client == nil {
		return defaultIntentConfig(), nil
	}

	resp, err := m.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
		MaxTokens: 512,
		System:    []anthropic.TextBlockParam{{Text: sys}},
		Messages:  []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(intent)),
		},
	})
	if err != nil {
		return defaultIntentConfig(), nil
	}
	if len(resp.Content) == 0 {
		return defaultIntentConfig(), nil
	}

	var cfg IntentConfig
	text := resp.Content[0].Text
	// Extract JSON from response
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(text[start:end+1]), &cfg); err == nil {
			return &cfg, nil
		}
	}
	return defaultIntentConfig(), nil
}

func defaultIntentConfig() *IntentConfig {
	return &IntentConfig{
		StrategyMode:   1,
		RiskThreshold:  10,
		YieldEntryRisk: 35,
		YieldExitRisk:  55,
		CircuitBreaker: 1.5,
		StrategyName:   "Balanced",
		Explanation:    "Balanced strategy: rebalance on moderate risk, enter yield positions when stable.",
	}
}

func buildChatSystemPrompt(pc PortfolioContext) string {
	stratNames := map[uint8]string{0: "SAFE", 1: "BALANCED", 2: "YIELD"}
	strat := stratNames[pc.StrategyMode]
	if strat == "" {
		strat = "BALANCED"
	}

	pricesStr := ""
	for sym, p := range pc.Prices {
		pricesStr += fmt.Sprintf("  %s: $%.6f\n", sym, p)
	}

	yieldStr := "No active yield position"
	if pc.YieldProtocol != "" {
		yieldStr = fmt.Sprintf("%s at %.2f%% APY, earned $%.4f", pc.YieldProtocol, pc.YieldAPY, pc.YieldEarned)
	}

	recentStr := ""
	for i, d := range pc.LastDecisions {
		if i >= 3 {
			break
		}
		recentStr += fmt.Sprintf("  [%s] %s (confidence %d%%, risk %.0f)\n",
			d.Ts.Format("15:04"), d.Action, d.Confidence, d.RiskLevel)
	}
	if recentStr == "" {
		recentStr = "  No recent decisions yet\n"
	}

	return fmt.Sprintf(`You are StableGuard AI — a friendly, expert DeFi financial advisor managing a stablecoin vault on Solana.

## Current Portfolio State
- Risk Level: %.1f/100 (%s)
- Strategy: %s
- Max Deviation: %.4f%%
- Active Yield: %s
- Total AI Decisions: %d | Rebalances: %d

## Live Prices
%s
## Recent Decisions
%s
## Your personality
- Speak concisely — max 3-4 sentences unless explanation is needed
- Be proactive: if risk is high, mention it unprompted
- Use specific numbers from the portfolio data above
- You CAN execute actions: if user asks to change strategy/threshold, include an action tag
- Format actions like this at the END of your reply ONLY if action is needed:
  [ACTION:set_strategy:{"mode":1}:Change to Balanced]
  [ACTION:pause:{}:Pause the vault]
  [ACTION:set_threshold:{"value":40}:Set threshold to 40]
- Never make up data — use only what's provided above
- If asked about something you don't know, say so honestly`,
		pc.RiskLevel, pc.RiskSummary,
		strat,
		pc.MaxDeviation,
		yieldStr,
		pc.TotalDecisions, pc.TotalRebalances,
		pricesStr,
		recentStr,
	)
}

func parseActionFromReply(text string) *ChatAction {
	// Look for [ACTION:type:params:label] at end of message
	const prefix = "[ACTION:"
	idx := strings.LastIndex(text, prefix)
	if idx == -1 {
		return nil
	}
	inner := strings.TrimPrefix(text[idx:], prefix)
	inner = strings.TrimSuffix(inner, "]")
	parts := strings.SplitN(inner, ":", 3)
	if len(parts) < 3 {
		return nil
	}

	actionType := strings.TrimSpace(parts[0])
	paramsStr  := strings.TrimSpace(parts[1])
	label      := strings.TrimSpace(parts[2])

	var params map[string]any
	_ = json.Unmarshal([]byte(paramsStr), &params)
	if params == nil {
		params = map[string]any{}
	}

	return &ChatAction{
		Type:    actionType,
		Params:  params,
		Label:   label,
		Confirm: true,
	}
}

func cleanReply(text string) string {
	// Remove the [ACTION:...] tag from display text
	const prefix = "[ACTION:"
	if idx := strings.LastIndex(text, prefix); idx != -1 {
		return strings.TrimSpace(text[:idx])
	}
	return strings.TrimSpace(text)
}

func dryRunReply(msg string, pc PortfolioContext) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "risk"):
		return fmt.Sprintf("Current risk level is %.0f/100. %s Strategy: %s.",
			pc.RiskLevel, pc.RiskSummary, map[uint8]string{0: "SAFE", 1: "BALANCED", 2: "YIELD"}[pc.StrategyMode])
	case strings.Contains(lower, "yield") || strings.Contains(lower, "earn"):
		if pc.YieldProtocol != "" {
			return fmt.Sprintf("Active yield position on %s at %.2f%% APY. Earned $%.4f so far.", pc.YieldProtocol, pc.YieldAPY, pc.YieldEarned)
		}
		return "No active yield position. Risk needs to be below 35 in OPTIMIZE mode to enter one."
	case strings.Contains(lower, "price") || strings.Contains(lower, "usdc") || strings.Contains(lower, "usdt"):
		return fmt.Sprintf("Max deviation: %.4f%%. All stablecoins are within normal range.", pc.MaxDeviation)
	default:
		return fmt.Sprintf("I'm monitoring your vault 24/7. Current risk: %.0f/100. %s", pc.RiskLevel, pc.RiskSummary)
	}
}
