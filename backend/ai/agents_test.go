package ai

import (
	"testing"

	"stableguard-backend/risk"
)

func TestBuildDecisionProfilesAdjustActionAndSize(t *testing.T) {
	score := risk.ScoreV2{
		Action:            "rebalance",
		FromIndex:         0,
		ToIndex:           1,
		SuggestedFraction: 0.5,
		RiskLevel:         25,
		Deviation:         0.15,
	}
	riskRes := AgentResult{Summary: "risk elevated", Action: "REBALANCE", Confidence: 70}
	yieldRes := AgentResult{Summary: "spread open", Action: "REBALANCE", Confidence: 80}

	cautious := buildDecision(score, riskRes, yieldRes, risk.StrategyModeBalanced, DecisionProfileCautious)
	if cautious.Action != ActionOptimize {
		t.Fatalf("expected cautious profile to optimize in balanced mode at medium risk, got %s", cautious.Action)
	}
	if cautious.SuggestedFraction >= 0.5 {
		t.Fatalf("expected cautious fraction to be reduced, got %.2f", cautious.SuggestedFraction)
	}

	aggressive := buildDecision(score, riskRes, yieldRes, risk.StrategyModeBalanced, DecisionProfileAggressive)
	if aggressive.Action != ActionProtect {
		t.Fatalf("expected aggressive profile to protect earlier, got %s", aggressive.Action)
	}
	if aggressive.SuggestedFraction <= 0.5 {
		t.Fatalf("expected aggressive fraction to increase, got %.2f", aggressive.SuggestedFraction)
	}
}

func TestNormalizeDecisionProfileDefaultsToBalanced(t *testing.T) {
	if got := normalizeDecisionProfile("unexpected"); got != DecisionProfileBalanced {
		t.Fatalf("expected balanced fallback, got %s", got)
	}
}
