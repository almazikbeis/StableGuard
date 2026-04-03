package policy

import (
	"testing"

	"stableguard-backend/ai"
	"stableguard-backend/config"
)

func TestEvaluateProtectInManualRequiresApproval(t *testing.T) {
	cfg := &config.Config{StrategyMode: 0, AutoExecute: false, YieldEnabled: false}
	eval := Evaluate(cfg, &ai.FinalDecision{Action: ai.ActionProtect})
	if eval.Verdict != VerdictRequiresApproval {
		t.Fatalf("expected requires_approval, got %s", eval.Verdict)
	}
}

func TestEvaluateYieldInGuardedBlocked(t *testing.T) {
	cfg := &config.Config{StrategyMode: 0, AutoExecute: true, YieldEnabled: false}
	eval := Evaluate(cfg, &ai.FinalDecision{Action: ai.ActionOptimize})
	if eval.Verdict != VerdictBlocked {
		t.Fatalf("expected blocked, got %s", eval.Verdict)
	}
}

func TestEvaluateYieldInYieldMaxAllowed(t *testing.T) {
	cfg := &config.Config{StrategyMode: 2, AutoExecute: true, YieldEnabled: true}
	eval := Evaluate(cfg, &ai.FinalDecision{Action: ai.ActionOptimize})
	if eval.Verdict != VerdictAllowed {
		t.Fatalf("expected allowed, got %s", eval.Verdict)
	}
}

func TestEvaluateYieldInBalancedRequiresApproval(t *testing.T) {
	cfg := &config.Config{StrategyMode: 1, AutoExecute: true, YieldEnabled: true}
	eval := Evaluate(cfg, &ai.FinalDecision{Action: ai.ActionOptimize})
	if eval.Verdict != VerdictRequiresApproval {
		t.Fatalf("expected requires_approval, got %s", eval.Verdict)
	}
}
