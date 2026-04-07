package execution

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"stableguard-backend/config"
	"stableguard-backend/store"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type fakeExecutor struct {
	wallet            solana.PublicKey
	balanceReads      map[string][]uint64
	simulateResult    *rpc.SimulateTransactionResult
	simulateErr       error
	sendSig           string
	sendErr           error
	confirmErr        error
	mintSig           string
	mintErr           error
	simulatedTxBase64 string
	submittedTxBase64 string
}

func (f *fakeExecutor) WalletAddress() solana.PublicKey { return f.wallet }

func (f *fakeExecutor) TokenAccountBalance(_ context.Context, account string) (uint64, error) {
	values := f.balanceReads[account]
	if len(values) == 0 {
		return 0, nil
	}
	value := values[0]
	if len(values) > 1 {
		f.balanceReads[account] = values[1:]
	}
	return value, nil
}

func (f *fakeExecutor) SimulateExternalTransaction(_ context.Context, txBase64 string) (*rpc.SimulateTransactionResult, error) {
	f.simulatedTxBase64 = txBase64
	return f.simulateResult, f.simulateErr
}

func (f *fakeExecutor) SendExternalTransaction(_ context.Context, txBase64 string) (string, error) {
	f.submittedTxBase64 = txBase64
	return f.sendSig, f.sendErr
}

func (f *fakeExecutor) WaitForSignatureConfirmation(_ context.Context, _ string, _ time.Duration, _ rpc.ConfirmationStatusType) (*rpc.SignatureStatusesResult, error) {
	return nil, f.confirmErr
}

func (f *fakeExecutor) MintTokensTo(_ context.Context, _ string, _ string, _ uint64) (string, error) {
	if f.mintSig != "" || f.mintErr != nil {
		return f.mintSig, f.mintErr
	}
	return "mint-signature", nil
}

func TestSubmitAndReconcilePersistsLifecycle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "execution-service.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	jobID, err := db.SaveExecutionJob(store.ExecutionJobRow{
		FromIndex:      0,
		ToIndex:        1,
		Amount:         1_000_000,
		Stage:          "custody_staged",
		SourceSymbol:   "USDC",
		TargetSymbol:   "USDT",
		CustodyAccount: "source-custody",
	})
	if err != nil {
		t.Fatalf("save execution job: %v", err)
	}

	job, err := db.ExecutionJobByID(jobID)
	if err != nil {
		t.Fatalf("load execution job: %v", err)
	}

	units := uint64(123456)
	svc := New(&fakeExecutor{
		wallet: solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
		balanceReads: map[string][]uint64{
			"source-custody": {1_000_000, 150_000},
			"target-custody": {25_000, 970_000},
		},
		simulateResult: &rpc.SimulateTransactionResult{UnitsConsumed: &units},
		sendSig:        "swap-signature",
	}, &config.Config{}, db)

	result, err := svc.SubmitAndReconcile(context.Background(), job, "target-custody", "swap-transaction-b64", "900000")
	if err != nil {
		t.Fatalf("submit and reconcile: %v", err)
	}
	if result.SwapSig != "swap-signature" {
		t.Fatalf("unexpected swap signature: %s", result.SwapSig)
	}
	if result.SourceDelta != 850_000 {
		t.Fatalf("unexpected source delta: %d", result.SourceDelta)
	}
	if result.TargetDelta != 945_000 {
		t.Fatalf("unexpected target delta: %d", result.TargetDelta)
	}
	if result.SimulationUnits != units {
		t.Fatalf("unexpected simulation units: %d", result.SimulationUnits)
	}
	if result.Job.Stage != "reconciled_in_custody" {
		t.Fatalf("expected reconciled_in_custody, got %s", result.Job.Stage)
	}

	reloaded, err := db.ExecutionJobByID(jobID)
	if err != nil {
		t.Fatalf("reload execution job: %v", err)
	}
	if reloaded.Stage != "reconciled_in_custody" {
		t.Fatalf("expected persisted reconciled stage, got %s", reloaded.Stage)
	}
	if reloaded.SourceBalanceBefore != 1_000_000 || reloaded.SourceBalanceAfter != 150_000 {
		t.Fatalf("unexpected persisted source balances: before=%d after=%d", reloaded.SourceBalanceBefore, reloaded.SourceBalanceAfter)
	}
	if reloaded.TargetBalanceBefore != 25_000 || reloaded.TargetBalanceAfter != 970_000 {
		t.Fatalf("unexpected persisted target balances: before=%d after=%d", reloaded.TargetBalanceBefore, reloaded.TargetBalanceAfter)
	}
}

func TestMarkFailedUpdatesExecutionJob(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "execution-failed.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	jobID, err := db.SaveExecutionJob(store.ExecutionJobRow{
		FromIndex:      0,
		ToIndex:        1,
		Amount:         1_000,
		Stage:          "custody_staged",
		SourceSymbol:   "USDC",
		TargetSymbol:   "USDT",
		CustodyAccount: "source-custody",
	})
	if err != nil {
		t.Fatalf("save execution job: %v", err)
	}

	svc := New(nil, &config.Config{}, db)
	if err := svc.MarkFailed(jobID, "quote guardrail rejected route"); err == nil {
		t.Fatal("expected MarkFailed to return an error")
	}

	reloaded, err := db.ExecutionJobByID(jobID)
	if err != nil {
		t.Fatalf("reload execution job: %v", err)
	}
	if reloaded.Stage != "failed" {
		t.Fatalf("expected failed stage, got %s", reloaded.Stage)
	}
	if reloaded.Note != "quote guardrail rejected route" {
		t.Fatalf("unexpected failure note: %s", reloaded.Note)
	}
	if ReconciledAmount(*reloaded) != 0 {
		t.Fatalf("expected zero reconciled amount, got %d", ReconciledAmount(*reloaded))
	}
}

func TestCustodyAccountSupportsMixedAssetTreasury(t *testing.T) {
	svc := New(nil, &config.Config{
		ExecutionCustodyUSDCAccount:  "usdc-custody",
		ExecutionCustodyUSDTAccount:  "usdt-custody",
		ExecutionCustodyETHAccount:   "eth-custody",
		ExecutionCustodySOLAccount:   "sol-custody",
		ExecutionCustodyBTCAccount:   "btc-custody",
		ExecutionCustodyDAIAccount:   "dai-custody",
		ExecutionCustodyPYUSDAccount: "pyusd-custody",
	}, nil)

	cases := map[string]string{
		"USDC":  "usdc-custody",
		"USDT":  "usdt-custody",
		"ETH":   "eth-custody",
		"SOL":   "sol-custody",
		"BTC":   "btc-custody",
		"DAI":   "dai-custody",
		"PYUSD": "pyusd-custody",
	}

	for symbol, want := range cases {
		if got := svc.CustodyAccount(symbol); got != want {
			t.Fatalf("custody account for %s: got %s want %s", symbol, got, want)
		}
	}

	if got := svc.CustodyAccount(" sol "); got != "sol-custody" {
		t.Fatalf("custody account should normalize symbol casing and spacing, got %s", got)
	}
}

func TestPrepareSwapRejectsMismatchedConfiguredSourceCustody(t *testing.T) {
	svc := New(&fakeExecutor{
		wallet: solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
	}, &config.Config{
		SolanaRPCURL:                 "https://api.mainnet-beta.solana.com",
		ExecutionCustodyUSDCAccount:  "usdc-custody",
		ExecutionCustodyUSDTAccount:  "usdt-custody",
		ExecutionCustodyETHAccount:   "eth-custody",
		ExecutionCustodySOLAccount:   "sol-custody",
		ExecutionCustodyBTCAccount:   "btc-custody",
		ExecutionCustodyDAIAccount:   "dai-custody",
		ExecutionCustodyPYUSDAccount: "pyusd-custody",
		ExecutionApprovalMode:        "auto",
	}, nil)

	_, err := svc.PrepareSwap(context.Background(), &store.ExecutionJobRow{
		Stage:          "custody_staged",
		SourceSymbol:   "USDC",
		TargetSymbol:   "BTC",
		CustodyAccount: "wrong-custody",
	}, 50, 1_000_000)
	if err == nil {
		t.Fatal("expected custody mismatch to be rejected")
	}
	if !strings.Contains(err.Error(), "does not match configured source custody") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDevnetMockSwapRejectsMismatchedTargetCustody(t *testing.T) {
	svc := New(&fakeExecutor{
		wallet: solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
	}, &config.Config{
		ExecutionCustodyUSDCAccount:  "usdc-custody",
		ExecutionCustodyUSDTAccount:  "usdt-custody",
		ExecutionCustodyETHAccount:   "eth-custody",
		ExecutionCustodySOLAccount:   "sol-custody",
		ExecutionCustodyBTCAccount:   "btc-custody",
		ExecutionCustodyDAIAccount:   "dai-custody",
		ExecutionCustodyPYUSDAccount: "pyusd-custody",
	}, nil)

	_, err := svc.DevnetMockSwap(context.Background(), &store.ExecutionJobRow{
		Stage:          "custody_staged",
		SourceSymbol:   "ETH",
		TargetSymbol:   "USDC",
		CustodyAccount: "eth-custody",
		Amount:         1_000_000,
	}, "wrong-target-custody", "mint-usdc")
	if err == nil {
		t.Fatal("expected target custody mismatch to be rejected")
	}
	if !strings.Contains(err.Error(), "does not match configured custody") {
		t.Fatalf("unexpected error: %v", err)
	}
}
