package store

import (
	"path/filepath"
	"testing"
)

func TestExecutionJobLifecycle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stableguard-test.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	jobID, err := s.SaveExecutionJob(ExecutionJobRow{
		FromIndex:            0,
		ToIndex:              1,
		Amount:               1_000_000,
		Stage:                "custody_staged",
		FundingSig:           "funding-sig",
		SourceSymbol:         "USDC",
		TargetSymbol:         "USDT",
		CustodyAccount:       "source-custody",
		TargetCustodyAccount: "target-custody",
		QuoteOutAmount:       "999500",
		MinOutAmount:         "998000",
		PriceImpactPct:       "0.01",
		SourceBalanceBefore:  1_000_000,
		TargetBalanceBefore:  0,
		Note:                 "funds staged",
	})
	if err != nil {
		t.Fatalf("save execution job: %v", err)
	}
	if jobID == 0 {
		t.Fatal("expected non-zero execution job id")
	}

	active, err := s.HasActiveExecutionJob()
	if err != nil {
		t.Fatalf("has active execution job: %v", err)
	}
	if !active {
		t.Fatal("expected staged job to be active")
	}

	job, err := s.ExecutionJobByID(jobID)
	if err != nil {
		t.Fatalf("execution job by id: %v", err)
	}
	if job.Stage != "custody_staged" {
		t.Fatalf("expected custody_staged, got %s", job.Stage)
	}

	job.Stage = "swap_submitted"
	job.SwapSig = "swap-sig"
	job.Note = "swap submitted"
	if err := s.UpdateExecutionJob(*job); err != nil {
		t.Fatalf("update execution job to swap_submitted: %v", err)
	}

	job, err = s.ExecutionJobByID(jobID)
	if err != nil {
		t.Fatalf("execution job by id after swap: %v", err)
	}
	if job.SwapSig != "swap-sig" {
		t.Fatalf("expected swap sig to persist, got %s", job.SwapSig)
	}

	job.Stage = "settled_back_to_treasury"
	job.SettlementSig = "settlement-sig"
	job.SettledAmount = 998_000
	job.Note = "settled"
	if err := s.UpdateExecutionJob(*job); err != nil {
		t.Fatalf("update execution job to settled_back_to_treasury: %v", err)
	}

	active, err = s.HasActiveExecutionJob()
	if err != nil {
		t.Fatalf("has active execution job after settlement: %v", err)
	}
	if active {
		t.Fatal("expected no active execution jobs after settlement")
	}

	rows, err := s.RecentExecutionJobs(10)
	if err != nil {
		t.Fatalf("recent execution jobs: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 execution job, got %d", len(rows))
	}
	if rows[0].SettlementSig != "settlement-sig" {
		t.Fatalf("expected settlement sig to persist, got %s", rows[0].SettlementSig)
	}
	if rows[0].SettledAmount != 998_000 {
		t.Fatalf("expected settled amount to persist, got %d", rows[0].SettledAmount)
	}
}

func TestOperatorSettingsRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stableguard-settings.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	input := OperatorSettings{
		StrategyMode:              2,
		AutoExecute:               true,
		YieldEnabled:              true,
		YieldEntryRisk:            42,
		YieldExitRisk:             63,
		CircuitBreakerPausePct:    1.25,
		AlertRiskThreshold:        55,
		ExecutionApprovalMode:     "auto",
		AIAgentModel:              "claude-sonnet-4-5",
		AIDecisionProfile:         "aggressive",
		GrowthSleeveEnabled:       true,
		GrowthSleeveBudgetPct:     12.5,
		GrowthSleeveMaxAssetPct:   5,
		GrowthSleeveAllowedAssets: "SOL,WBTC,JITOSOL",
		GrowthSleeveLiveExecution: true,
		TelegramBotToken:          "tg-token",
		TelegramChatID:            "tg-chat",
		DiscordWebhookURL:         "https://discord.example/webhook",
	}
	if err := s.SaveOperatorSettings(input); err != nil {
		t.Fatalf("save operator settings: %v", err)
	}

	got, err := s.OperatorSettings()
	if err != nil {
		t.Fatalf("load operator settings: %v", err)
	}
	if got.StrategyMode != input.StrategyMode {
		t.Fatalf("expected strategy mode %d, got %d", input.StrategyMode, got.StrategyMode)
	}
	if got.ExecutionApprovalMode != "auto" {
		t.Fatalf("expected auto approval mode, got %s", got.ExecutionApprovalMode)
	}
	if !got.GrowthSleeveEnabled || !got.GrowthSleeveLiveExecution {
		t.Fatalf("growth sleeve flags did not persist")
	}
	if got.GrowthSleeveAllowedAssets != input.GrowthSleeveAllowedAssets {
		t.Fatalf("expected growth asset list %q, got %q", input.GrowthSleeveAllowedAssets, got.GrowthSleeveAllowedAssets)
	}
	if got.TelegramBotToken != input.TelegramBotToken || got.TelegramChatID != input.TelegramChatID {
		t.Fatalf("telegram credentials did not persist")
	}
	if got.DiscordWebhookURL != input.DiscordWebhookURL {
		t.Fatalf("discord webhook did not persist")
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("expected updated_at to be populated")
	}
}
