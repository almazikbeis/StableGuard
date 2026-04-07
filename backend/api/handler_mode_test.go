package api

import (
	"testing"

	"stableguard-backend/config"
)

func TestApplyRuntimeModeProfileManualFailsClosed(t *testing.T) {
	cfg := &config.Config{
		ExecutionApprovalMode:     "auto",
		GrowthSleeveLiveExecution: true,
	}

	profile, ok := controlModeProfile("MANUAL")
	if !ok {
		t.Fatal("expected MANUAL profile")
	}
	applyRuntimeModeProfile(cfg, profile)

	if cfg.AutoExecute {
		t.Fatal("manual mode must disable auto execution")
	}
	if cfg.YieldEnabled {
		t.Fatal("manual mode must disable yield")
	}
	if cfg.ExecutionApprovalMode != "manual" {
		t.Fatalf("manual mode should force manual approval, got %s", cfg.ExecutionApprovalMode)
	}
	if cfg.GrowthSleeveLiveExecution {
		t.Fatal("manual mode must disable live growth sleeve execution")
	}
}

func TestApplyRuntimeModeProfileYieldMaxKeepsAutonomousPathsArmed(t *testing.T) {
	cfg := &config.Config{
		GrowthSleeveLiveExecution: true,
	}

	profile, ok := controlModeProfile("YIELD_MAX")
	if !ok {
		t.Fatal("expected YIELD_MAX profile")
	}
	applyRuntimeModeProfile(cfg, profile)

	if !cfg.AutoExecute {
		t.Fatal("yield max must keep auto execution enabled")
	}
	if !cfg.YieldEnabled {
		t.Fatal("yield max must keep yield enabled")
	}
	if cfg.ExecutionApprovalMode != "auto" {
		t.Fatalf("yield max should force auto approval, got %s", cfg.ExecutionApprovalMode)
	}
	if !cfg.GrowthSleeveLiveExecution {
		t.Fatal("yield max should not forcibly disable live growth sleeve execution")
	}
}

func TestModeReadinessYieldMaxBlockedWithoutLivePrerequisites(t *testing.T) {
	item := modeReadiness(&config.Config{
		SolanaRPCURL:          "https://api.devnet.solana.com",
		StrategyMode:          2,
		AutoExecute:           true,
		YieldEnabled:          true,
		ExecutionApprovalMode: "auto",
	}, "YIELD_MAX")

	ready, _ := item["ready"].(bool)
	if ready {
		t.Fatal("expected YIELD_MAX readiness to be blocked without live prerequisites")
	}
	blockers, _ := item["blockers"].([]string)
	if len(blockers) == 0 {
		t.Fatal("expected YIELD_MAX readiness to surface blockers")
	}
}
