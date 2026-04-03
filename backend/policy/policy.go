package policy

import (
	"stableguard-backend/ai"
	"stableguard-backend/config"
)

type ActionClass string

const (
	ActionClassObserve       ActionClass = "observe"
	ActionClassRecommend     ActionClass = "recommend"
	ActionClassProtect       ActionClass = "protect"
	ActionClassYieldAllocate ActionClass = "yield_allocate"
)

type Verdict string

const (
	VerdictAllowed          Verdict = "allowed"
	VerdictBlocked          Verdict = "blocked"
	VerdictRequiresApproval Verdict = "requires_approval"
)

type Evaluation struct {
	ActionClass     ActionClass `json:"action_class"`
	Verdict         Verdict     `json:"verdict"`
	Reason          string      `json:"reason"`
	ControlMode     string      `json:"control_mode"`
	AutoExecute     bool        `json:"auto_execute"`
	YieldEnabled    bool        `json:"yield_enabled"`
	ExecutionIntent string      `json:"execution_intent"`
}

type rule struct {
	verdict         Verdict
	reason          string
	executionIntent string
}

var modePolicies = map[string]map[ActionClass]rule{
	"UNKNOWN": {
		ActionClassObserve: {
			verdict:         VerdictAllowed,
			reason:          "No AI action proposed.",
			executionIntent: "none",
		},
		ActionClassRecommend: {
			verdict:         VerdictAllowed,
			reason:          "Policy allows recommendation-only operation.",
			executionIntent: "explain_only",
		},
		ActionClassProtect: {
			verdict:         VerdictRequiresApproval,
			reason:          "Unknown mode defaults to operator approval for protective actions.",
			executionIntent: "approval_gate",
		},
		ActionClassYieldAllocate: {
			verdict:         VerdictRequiresApproval,
			reason:          "Unknown mode defaults to operator approval for yield allocation.",
			executionIntent: "approval_gate",
		},
	},
	"MANUAL": {
		ActionClassObserve: {
			verdict:         VerdictAllowed,
			reason:          "No AI action proposed.",
			executionIntent: "none",
		},
		ActionClassRecommend: {
			verdict:         VerdictAllowed,
			reason:          "Manual mode allows observation and recommendation only.",
			executionIntent: "explain_only",
		},
		ActionClassProtect: {
			verdict:         VerdictRequiresApproval,
			reason:          "Manual mode requires operator approval for protective actions.",
			executionIntent: "approval_gate",
		},
		ActionClassYieldAllocate: {
			verdict:         VerdictBlocked,
			reason:          "Manual mode does not allow AI yield allocation.",
			executionIntent: "blocked",
		},
	},
	"GUARDED": {
		ActionClassObserve: {
			verdict:         VerdictAllowed,
			reason:          "No AI action proposed.",
			executionIntent: "none",
		},
		ActionClassRecommend: {
			verdict:         VerdictAllowed,
			reason:          "Guarded mode allows recommendation-only operation.",
			executionIntent: "explain_only",
		},
		ActionClassProtect: {
			verdict:         VerdictAllowed,
			reason:          "Guarded mode allows protective AI actions within configured limits.",
			executionIntent: "signal_only_under_current_custody",
		},
		ActionClassYieldAllocate: {
			verdict:         VerdictBlocked,
			reason:          "Guarded mode does not allow AI yield allocation.",
			executionIntent: "blocked",
		},
	},
	"BALANCED": {
		ActionClassObserve: {
			verdict:         VerdictAllowed,
			reason:          "No AI action proposed.",
			executionIntent: "none",
		},
		ActionClassRecommend: {
			verdict:         VerdictAllowed,
			reason:          "Balanced mode allows recommendation-only operation.",
			executionIntent: "explain_only",
		},
		ActionClassProtect: {
			verdict:         VerdictAllowed,
			reason:          "Balanced mode allows protective AI actions within configured limits.",
			executionIntent: "signal_only_under_current_custody",
		},
		ActionClassYieldAllocate: {
			verdict:         VerdictRequiresApproval,
			reason:          "Balanced mode keeps yield allocation behind operator approval.",
			executionIntent: "approval_gate",
		},
	},
	"YIELD_MAX": {
		ActionClassObserve: {
			verdict:         VerdictAllowed,
			reason:          "No AI action proposed.",
			executionIntent: "none",
		},
		ActionClassRecommend: {
			verdict:         VerdictAllowed,
			reason:          "Yield Max mode allows recommendation-only operation.",
			executionIntent: "explain_only",
		},
		ActionClassProtect: {
			verdict:         VerdictAllowed,
			reason:          "Yield Max mode allows protective AI actions within configured limits.",
			executionIntent: "signal_only_under_current_custody",
		},
		ActionClassYieldAllocate: {
			verdict:         VerdictAllowed,
			reason:          "Yield Max mode allows AI-managed yield allocation within configured limits.",
			executionIntent: "simulated_yield_or_signal_only",
		},
	},
}

func Evaluate(cfg *config.Config, d *ai.FinalDecision) Evaluation {
	controlMode := deriveControlMode(cfg)
	eval := Evaluation{
		ActionClass:  classifyDecision(d),
		ControlMode:  controlMode,
		AutoExecute:  cfg != nil && cfg.AutoExecute,
		YieldEnabled: cfg != nil && cfg.YieldEnabled,
	}

	modeRules, ok := modePolicies[controlMode]
	if !ok {
		modeRules = modePolicies["UNKNOWN"]
	}
	selected, ok := modeRules[eval.ActionClass]
	if !ok {
		selected = rule{
			verdict:         VerdictRequiresApproval,
			reason:          "Action class is not whitelisted for autonomous execution.",
			executionIntent: "approval_gate",
		}
	}

	if eval.ActionClass == ActionClassYieldAllocate && !eval.YieldEnabled {
		selected = rule{
			verdict:         VerdictBlocked,
			reason:          "Yield action proposed while the yield layer is disabled.",
			executionIntent: "blocked",
		}
	}

	if eval.ActionClass == ActionClassProtect && !eval.AutoExecute && selected.verdict == VerdictAllowed {
		selected.executionIntent = "recommend_only"
	}

	eval.Verdict = selected.verdict
	eval.Reason = selected.reason
	eval.ExecutionIntent = selected.executionIntent

	return eval
}

func classifyDecision(d *ai.FinalDecision) ActionClass {
	if d == nil {
		return ActionClassObserve
	}
	switch d.Action {
	case ai.ActionHold:
		return ActionClassRecommend
	case ai.ActionProtect:
		return ActionClassProtect
	case ai.ActionOptimize:
		return ActionClassYieldAllocate
	default:
		return ActionClassRecommend
	}
}

func deriveControlMode(cfg *config.Config) string {
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
