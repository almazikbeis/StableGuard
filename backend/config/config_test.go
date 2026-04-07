package config

import "testing"

func TestExecutionReadinessDefaultsToRecordOnly(t *testing.T) {
	cfg := &Config{}
	readiness := cfg.ExecutionReadiness()

	if readiness.Mode != "record_only" {
		t.Fatalf("expected record_only, got %s", readiness.Mode)
	}
	if readiness.ReadyForStaging {
		t.Fatal("expected staging to be disabled by default")
	}
	if readiness.ReadyForAutoSwap {
		t.Fatal("expected auto swap to be disabled by default")
	}
	if readiness.ApprovalMode != "manual" {
		t.Fatalf("expected manual approval mode, got %s", readiness.ApprovalMode)
	}
	if len(readiness.MissingCustodyAccounts) != 7 {
		t.Fatalf("expected 7 missing custody accounts, got %d", len(readiness.MissingCustodyAccounts))
	}
}

func TestExecutionReadinessEnablesCustodyScaffoldWhenFullyConfigured(t *testing.T) {
	cfg := &Config{
		SolanaRPCURL:                 "https://api.mainnet-beta.solana.com",
		ExecutionCustodyUSDCAccount:  "usdc-custody",
		ExecutionCustodyUSDTAccount:  "usdt-custody",
		ExecutionCustodyETHAccount:   "eth-custody",
		ExecutionCustodySOLAccount:   "sol-custody",
		ExecutionCustodyBTCAccount:   "btc-custody",
		ExecutionCustodyDAIAccount:   "dai-custody",
		ExecutionCustodyPYUSDAccount: "pyusd-custody",
		ExecutionApprovalMode:        "auto",
	}
	readiness := cfg.ExecutionReadiness()

	if readiness.Mode != "custody_scaffold" {
		t.Fatalf("expected custody_scaffold, got %s", readiness.Mode)
	}
	if !readiness.ReadyForStaging {
		t.Fatal("expected staging to be enabled")
	}
	if !readiness.ReadyForAutoSwap {
		t.Fatal("expected auto swap to be enabled on mainnet-ready config")
	}
	if !readiness.AutoExecutionEnabled {
		t.Fatal("expected auto execution to be enabled in auto mode")
	}
	if len(readiness.MissingCustodyAccounts) != 0 {
		t.Fatalf("expected no missing custody accounts, got %d", len(readiness.MissingCustodyAccounts))
	}
}

func TestGetAIDecisionProfileDefaultsToBalanced(t *testing.T) {
	if got := getAIDecisionProfile("MISSING_KEY_FOR_TEST", "balanced"); got != "balanced" {
		t.Fatalf("expected balanced, got %s", got)
	}
}

func TestGrowthReadinessDefaultsToDisabled(t *testing.T) {
	cfg := &Config{}
	readiness := cfg.GrowthReadiness()

	if readiness.Mode != "disabled" {
		t.Fatalf("expected disabled, got %s", readiness.Mode)
	}
	if readiness.ReadyForLive {
		t.Fatal("growth sleeve should not be live by default")
	}
}

func TestGrowthReadinessRequiresExplicitLiveOptIn(t *testing.T) {
	cfg := &Config{
		SolanaRPCURL:                 "https://api.mainnet-beta.solana.com",
		ExecutionCustodyUSDCAccount:  "usdc-custody",
		ExecutionCustodyUSDTAccount:  "usdt-custody",
		ExecutionCustodyETHAccount:   "eth-custody",
		ExecutionCustodySOLAccount:   "sol-custody",
		ExecutionCustodyBTCAccount:   "btc-custody",
		ExecutionCustodyDAIAccount:   "dai-custody",
		ExecutionCustodyPYUSDAccount: "pyusd-custody",
		ExecutionApprovalMode:        "auto",
		GrowthSleeveEnabled:          true,
		GrowthSleeveBudgetPct:        15,
		GrowthSleeveMaxAssetPct:      5,
		GrowthSleeveAllowedAssets:    []string{"SOL", "WBTC"},
	}
	readiness := cfg.GrowthReadiness()
	if readiness.Mode != "paper" {
		t.Fatalf("expected paper mode before explicit live opt-in, got %s", readiness.Mode)
	}

	cfg.GrowthSleeveLiveExecution = true
	readiness = cfg.GrowthReadiness()
	if readiness.Mode != "live" {
		t.Fatalf("expected live mode after explicit opt-in, got %s", readiness.Mode)
	}
	if !readiness.ReadyForLive {
		t.Fatal("expected growth sleeve to be live-ready")
	}
}
