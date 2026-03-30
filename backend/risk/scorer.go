// Package risk computes a risk score from Pyth price data.
package risk

import (
	"math"
	"stableguard-backend/pyth"
)

// Score holds the computed risk metrics.
type Score struct {
	// 0–100, higher = more risky / actionable
	RiskLevel float64
	// Deviation between USDC and USDT price (percentage)
	Deviation float64
	// Suggested rebalance direction: 0 = A→B, 1 = B→A, -1 = no action
	SuggestedDirection int
	// Suggested amount as fraction of total vault (0–1)
	SuggestedFraction float64
	// Human-readable summary
	Summary string
	// Action string for the executor
	Action string
}

// Strategy modes
const (
	StrategyModeSafe  uint8 = 0
	StrategyModeYield uint8 = 1
)

// Safe mode: act only above this deviation threshold (bps)
const SafeDeviationThresholdBPS = 5.0 // 0.05%

// Yield mode: act above this (much lower) threshold
const YieldDeviationThresholdBPS = 0.5 // 0.005%

// MaxRiskDeviation: deviation at which risk score hits 100 (bps)
const MaxRiskDeviation = 50.0

// Compute calculates a risk score from a Pyth price snapshot.
// strategyMode: 0 = Safe (conservative), 1 = Yield (aggressive arbitrage)
// balanceA and balanceB are the current virtual balances in the vault.
func Compute(snap *pyth.PriceSnapshot, balanceA, balanceB uint64, strategyMode uint8) Score {
	devPct := snap.Deviation()
	devBPS := devPct * 100 // convert percentage to bps

	// Base risk score (0–100)
	baseScore := math.Min(100, (devBPS/MaxRiskDeviation)*100)

	var effectiveScore float64
	var threshold float64

	if strategyMode == StrategyModeYield {
		// Yield mode: 3× more sensitive — small deviations are opportunities
		effectiveScore = math.Min(100, baseScore*3)
		threshold = YieldDeviationThresholdBPS
	} else {
		// Safe mode: standard sensitivity
		effectiveScore = baseScore
		threshold = SafeDeviationThresholdBPS
	}

	s := Score{
		RiskLevel: math.Round(effectiveScore*100) / 100,
		Deviation: devPct,
	}

	if devBPS < threshold {
		s.SuggestedDirection = -1
		s.SuggestedFraction = 0
		s.Action = "hold"
		if strategyMode == StrategyModeYield {
			s.Summary = "Spread too tight for yield arbitrage, holding"
		} else {
			s.Summary = "Prices within normal range, no rebalance needed"
		}
		return s
	}

	total := balanceA + balanceB
	if total == 0 {
		s.SuggestedDirection = -1
		s.Action = "hold"
		s.Summary = "Vault empty"
		return s
	}

	if snap.USDC.Price > snap.USDT.Price {
		// USDC > USDT: sell expensive USDC, buy cheap USDT → direction 0 (A→B)
		s.SuggestedDirection = 0
		s.Action = "swap_usdc_to_usdt"
		if strategyMode == StrategyModeYield {
			s.SuggestedFraction = math.Min(0.5, devBPS/10)
			s.Summary = "USDC above peg — arbitrage: swap USDC→USDT for yield"
		} else {
			s.SuggestedFraction = math.Min(0.5, devBPS/100)
			s.Summary = "USDC trading above peg — shift allocation to USDT"
		}
	} else {
		// USDT > USDC: sell expensive USDT, buy cheap USDC → direction 1 (B→A)
		s.SuggestedDirection = 1
		s.Action = "swap_usdt_to_usdc"
		if strategyMode == StrategyModeYield {
			s.SuggestedFraction = math.Min(0.5, devBPS/10)
			s.Summary = "USDT above peg — arbitrage: swap USDT→USDC for yield"
		} else {
			s.SuggestedFraction = math.Min(0.5, devBPS/100)
			s.Summary = "USDT trading above peg — shift allocation to USDC"
		}
	}

	return s
}

// DetermineAction returns the action string based on strategy mode and risk score.
func DetermineAction(score Score, threshold uint8, strategyMode uint8) string {
	if strategyMode == StrategyModeSafe {
		// Safe: only act if risk exceeds configured threshold
		if score.RiskLevel < float64(threshold) {
			return "hold"
		}
	} else {
		// Yield: act on any spread > 5 risk points
		if score.RiskLevel < 5 {
			return "hold"
		}
	}
	return score.Action
}
