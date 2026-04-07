package pipeline

import (
	"testing"

	"stableguard-backend/config"
)

func TestLiveYieldModeRequiresYieldMaxControlMode(t *testing.T) {
	engine := &Engine{
		cfg: &config.Config{
			SolanaRPCURL:              "https://api.mainnet-beta.solana.com",
			StrategyMode:              1,
			AutoExecute:               true,
			YieldEnabled:              true,
			YieldStrategyUSDCAccount:  "strategy-usdc",
			YieldStrategyUSDTAccount:  "strategy-usdt",
			YieldStrategyDAIAccount:   "strategy-dai",
			YieldStrategyPYUSDAccount: "strategy-pyusd",
			YieldKaminoUSDCVault:      "kamino-usdc",
			YieldKaminoUSDTVault:      "kamino-usdt",
			YieldKaminoDAIVault:       "kamino-dai",
			YieldKaminoPYUSDVault:     "kamino-pyusd",
		},
	}

	ok, reason := engine.liveYieldMode()
	if ok {
		t.Fatal("expected live yield mode to be blocked outside YIELD_MAX")
	}
	if reason == "" {
		t.Fatal("expected non-empty reason when live yield mode is blocked")
	}
}

func TestLiveYieldModeReadyOnlyWhenYieldMaxAndYieldReadinessGreen(t *testing.T) {
	engine := &Engine{
		cfg: &config.Config{
			SolanaRPCURL:              "https://api.mainnet-beta.solana.com",
			StrategyMode:              2,
			AutoExecute:               true,
			YieldEnabled:              true,
			YieldStrategyUSDCAccount:  "strategy-usdc",
			YieldStrategyUSDTAccount:  "strategy-usdt",
			YieldStrategyDAIAccount:   "strategy-dai",
			YieldStrategyPYUSDAccount: "strategy-pyusd",
			YieldKaminoUSDCVault:      "kamino-usdc",
			YieldKaminoUSDTVault:      "kamino-usdt",
			YieldKaminoDAIVault:       "kamino-dai",
			YieldKaminoPYUSDVault:     "kamino-pyusd",
		},
	}

	ok, reason := engine.liveYieldMode()
	if !ok {
		t.Fatalf("expected live yield mode to be ready, got reason: %s", reason)
	}
	if reason != "" {
		t.Fatalf("expected empty reason when live yield mode is ready, got %s", reason)
	}
}
