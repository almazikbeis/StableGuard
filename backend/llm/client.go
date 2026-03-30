// Package llm wraps the Anthropic Claude API for AI-driven vault decisions.
package llm

import (
	"context"
	"fmt"
	"stableguard-backend/pyth"
	"stableguard-backend/risk"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Decision is the structured output from the LLM.
type Decision struct {
	// Action: 0=hold, 1=rebalance A→B, 2=rebalance B→A
	Action     int
	Rationale  string
	Confidence int // 0–100
}

// Client wraps the Anthropic SDK client.
type Client struct {
	ac anthropic.Client
}

// New creates a new LLM client.
func New(apiKey string) *Client {
	ac := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &Client{ac: ac}
}

// Decide asks Claude to evaluate a price snapshot and risk score, returning a structured decision.
func (c *Client) Decide(ctx context.Context, snap *pyth.PriceSnapshot, score risk.Score, strategyMode uint8) (*Decision, error) {
	sysPrompt := buildSystemPrompt(strategyMode)
	userPrompt := buildUserPrompt(snap, score, strategyMode)

	msg, err := c.ac.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
		MaxTokens: 512,
		System: []anthropic.TextBlockParam{
			{Text: sysPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("llm request: %w", err)
	}

	if len(msg.Content) == 0 {
		return nil, fmt.Errorf("empty response from llm")
	}

	return parseDecision(msg.Content[0].Text, score)
}

func buildSystemPrompt(strategyMode uint8) string {
	if strategyMode == risk.StrategyModeYield {
		return `You are StableGuard AI in YIELD mode — an aggressive stablecoin arbitrage analyst.

Your goal is to find and exploit small price differences between USDC and USDT.
Even a 0.005% spread is actionable — buy the cheap one, sell the expensive one.
Buy the stablecoin below $1 (it will return to peg = profit).

Always respond in this exact JSON format:
{
  "action": <0=hold, 1=rebalance_A_to_B, 2=rebalance_B_to_A>,
  "rationale": "<one sentence: what spread, what trade, expected yield>",
  "confidence": <0-100>
}

Rules:
- action=1 means sell USDC, buy USDT (USDC is expensive or USDT is cheap)
- action=2 means sell USDT, buy USDC (USDT is expensive or USDC is cheap)
- confidence reflects data quality and spread size`
	}

	return `You are StableGuard AI in SAFE mode — a conservative stablecoin risk analyst.

Your goal is to protect the vault from depeg events.
Only recommend action if there is genuine depeg risk (deviation > 0.05%).
Prefer stability — do not churn the vault unnecessarily.

Always respond in this exact JSON format:
{
  "action": <0=hold, 1=rebalance_A_to_B, 2=rebalance_B_to_A>,
  "rationale": "<one sentence explanation>",
  "confidence": <0-100>
}

Rules:
- action=1 means move allocation from USDC to USDT
- action=2 means move allocation from USDT to USDC
- confidence reflects your certainty given data quality`
}

func buildUserPrompt(snap *pyth.PriceSnapshot, score risk.Score, strategyMode uint8) string {
	mode := "SAFE"
	if strategyMode == risk.StrategyModeYield {
		mode = "YIELD"
	}
	return fmt.Sprintf(`Strategy mode: %s
USDC price: $%.6f (±$%.6f)
USDT price: $%.6f (±$%.6f)
Deviation: %.5f%%
Risk score: %.1f/100
Pre-computed suggestion: %s

Respond in JSON only.`,
		mode,
		snap.USDC.Price, snap.USDC.Confidence,
		snap.USDT.Price, snap.USDT.Confidence,
		score.Deviation,
		score.RiskLevel,
		score.Summary,
	)
}

func parseDecision(text string, score risk.Score) (*Decision, error) {
	var action int
	var confidence int

	_, err := fmt.Sscanf(extractField(text, "action"), "%d", &action)
	if err != nil || action < 0 || action > 2 {
		action = 0
	}

	rationale := extractStringField(text, "rationale")
	if rationale == "" {
		rationale = score.Summary
	}

	_, err = fmt.Sscanf(extractField(text, "confidence"), "%d", &confidence)
	if err != nil || confidence < 0 || confidence > 100 {
		confidence = int(score.RiskLevel)
	}

	return &Decision{
		Action:     action,
		Rationale:  rationale,
		Confidence: confidence,
	}, nil
}

func extractField(s, field string) string {
	key := fmt.Sprintf(`"%s":`, field)
	idx := -1
	for i := 0; i < len(s)-len(key); i++ {
		if s[i:i+len(key)] == key {
			idx = i + len(key)
			break
		}
	}
	if idx == -1 {
		return ""
	}
	for idx < len(s) && (s[idx] == ' ' || s[idx] == '\t') {
		idx++
	}
	end := idx
	for end < len(s) && s[end] != ',' && s[end] != '\n' && s[end] != '}' {
		end++
	}
	return s[idx:end]
}

func extractStringField(s, field string) string {
	key := fmt.Sprintf(`"%s":`, field)
	idx := -1
	for i := 0; i < len(s)-len(key); i++ {
		if s[i:i+len(key)] == key {
			idx = i + len(key)
			break
		}
	}
	if idx == -1 {
		return ""
	}
	for idx < len(s) && s[idx] != '"' {
		idx++
	}
	if idx >= len(s) {
		return ""
	}
	idx++
	end := idx
	for end < len(s) && s[end] != '"' {
		end++
	}
	return s[idx:end]
}
