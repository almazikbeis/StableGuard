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
	// FromIndex is the token slot to move funds from (-1 = no action)
	FromIndex int
	// ToIndex is the token slot to move funds to (-1 = no action)
	ToIndex int
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
// strategyMode: 0 = Safe (conservative), 1 = Yield (aggressive arbitrage).
// balances is the current virtual balance slice for all registered tokens;
// index 0 is assumed to be USDC, index 1 is USDT (adjust for your token ordering).
func Compute(snap *pyth.PriceSnapshot, balances []uint64, strategyMode uint8) Score {
	devPct := snap.Deviation()
	devBPS := devPct * 100 // convert percentage to bps

	// Base risk score (0–100)
	baseScore := math.Min(100, (devBPS/MaxRiskDeviation)*100)

	var effectiveScore float64
	var threshold float64

	if strategyMode == StrategyModeYield {
		effectiveScore = math.Min(100, baseScore*3)
		threshold = YieldDeviationThresholdBPS
	} else {
		effectiveScore = baseScore
		threshold = SafeDeviationThresholdBPS
	}

	s := Score{
		RiskLevel: math.Round(effectiveScore*100) / 100,
		Deviation: devPct,
		FromIndex: -1,
		ToIndex:   -1,
	}

	if devBPS < threshold {
		s.Action = "hold"
		if strategyMode == StrategyModeYield {
			s.Summary = "Spread too tight for yield arbitrage, holding"
		} else {
			s.Summary = "Prices within normal range, no rebalance needed"
		}
		return s
	}

	var total uint64
	for _, b := range balances {
		total += b
	}
	if total == 0 {
		s.Action = "hold"
		s.Summary = "Vault empty"
		return s
	}

	// Default: index 0 = USDC-equivalent, index 1 = USDT-equivalent.
	// Determine which direction to rebalance based on which token is overpriced.
	if snap.USDC.Price > snap.USDT.Price {
		// USDC > USDT: sell expensive USDC (index 0), buy cheap USDT (index 1)
		s.FromIndex = 0
		s.ToIndex = 1
		s.Action = "swap_usdc_to_usdt"
		if strategyMode == StrategyModeYield {
			s.SuggestedFraction = math.Min(0.5, devBPS/10)
			s.Summary = "USDC above peg — arbitrage: swap USDC→USDT for yield"
		} else {
			s.SuggestedFraction = math.Min(0.5, devBPS/100)
			s.Summary = "USDC trading above peg — shift allocation to USDT"
		}
	} else {
		// USDT > USDC: sell expensive USDT (index 1), buy cheap USDC (index 0)
		s.FromIndex = 1
		s.ToIndex = 0
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
		if score.RiskLevel < float64(threshold) {
			return "hold"
		}
	} else {
		if score.RiskLevel < 5 {
			return "hold"
		}
	}
	return score.Action
}
