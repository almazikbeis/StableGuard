// scorer_v2.go — enhanced risk engine with sliding window, trend, velocity, volatility.
package risk

import (
	"fmt"
	"math"
	"sync"
	"time"

	"stableguard-backend/pyth"
)

const defaultWindowSize = 20

// Strategy mode constants (backend — 3 levels).
// On-chain contract maps: 0=SAFE, 1=YIELD (BALANCED and YIELD both map to on-chain 1).
const (
	StrategyModeBalanced uint8 = 1 // new default
	StrategyModeYieldV2  uint8 = 2 // aggressive
)

// PricePoint is a single timestamped observation in the sliding window.
type PricePoint struct {
	Time              time.Time
	USDCPrice         float64
	USDTPrice         float64
	Deviation         float64            // |USDC-USDT| / avg * 100
	VolatilePctChange float64            // max single-tick % change among volatile assets
	VolatilePrices    map[string]float64 // symbol → price (BTC/ETH/SOL)
}

// ScoreV2 holds the enhanced risk metrics from the windowed scorer.
type ScoreV2 struct {
	RiskLevel         float64            `json:"risk_level"`          // 0–100 (weighted composite)
	Deviation         float64            `json:"deviation_pct"`       // current price deviation %
	Trend             float64            `json:"trend"`               // linear slope of deviation over window
	Velocity          float64            `json:"velocity"`            // absolute change from previous tick
	Volatility        float64            `json:"volatility"`          // std-dev of deviations in window
	StableRisk        float64            `json:"stable_risk"`         // stablecoin peg component 0–100
	VolatileRisk      float64            `json:"volatile_risk"`       // volatile crash component 0–100
	VolatilePrices    map[string]float64 `json:"volatile_prices"`     // BTC/ETH/SOL current prices
	FromIndex         int                `json:"from_index"`          // token slot to rebalance from (-1 = hold)
	ToIndex           int                `json:"to_index"`            // token slot to rebalance into  (-1 = hold)
	SuggestedFraction float64            `json:"suggested_fraction"`  // 0–0.5 of total vault
	Action            string             `json:"action"`              // "hold" | "rebalance"
	Summary           string             `json:"summary"`
	WindowSize        int                `json:"window_size"`
}

// WindowedScorer maintains a sliding window of price observations.
type WindowedScorer struct {
	mu        sync.RWMutex
	window    []PricePoint
	maxSize   int
	lastScore ScoreV2
}

// NewWindowedScorer creates a scorer with the given window size.
func NewWindowedScorer(windowSize int) *WindowedScorer {
	if windowSize <= 0 {
		windowSize = defaultWindowSize
	}
	return &WindowedScorer{
		maxSize: windowSize,
		lastScore: ScoreV2{FromIndex: -1, ToIndex: -1, Action: "hold", Summary: "initializing"},
	}
}

// Push adds a new price snapshot to the sliding window.
func (w *WindowedScorer) Push(snap *pyth.PriceSnapshot) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Track volatile asset prices and compute max 1-tick % change.
	volatileFeeds := pyth.VolatileFeeds()
	volatilePrices := make(map[string]float64, len(volatileFeeds))
	var maxChange float64
	for _, f := range volatileFeeds {
		if pd, ok := snap.All[f.Symbol]; ok && pd.Price > 0 {
			volatilePrices[f.Symbol] = pd.Price
			if len(w.window) > 0 {
				prev := w.window[len(w.window)-1]
				if prevPrice, ok2 := prev.VolatilePrices[f.Symbol]; ok2 && prevPrice > 0 {
					change := math.Abs(pd.Price-prevPrice) / prevPrice * 100
					if change > maxChange {
						maxChange = change
					}
				}
			}
		}
	}

	pt := PricePoint{
		Time:              snap.FetchedAt,
		USDCPrice:         snap.USDC.Price,
		USDTPrice:         snap.USDT.Price,
		Deviation:         snap.Deviation(),
		VolatilePctChange: maxChange,
		VolatilePrices:    volatilePrices,
	}
	w.window = append(w.window, pt)
	if len(w.window) > w.maxSize {
		w.window = w.window[1:]
	}
}

// Compute calculates the v2 risk score from the current window + latest snapshot.
//
// Formula (all components normalized 0–100, then weighted):
//
//	risk = deviation*0.4 + velocity*0.2 + trend_bias*0.2 + volatility*0.2
func (w *WindowedScorer) Compute(snap *pyth.PriceSnapshot, balances []uint64, strategyMode uint8) ScoreV2 {
	w.mu.RLock()
	pts := make([]PricePoint, len(w.window))
	copy(pts, w.window)
	w.mu.RUnlock()

	current := snap.Deviation()

	var trend, velocity, volatility float64
	if len(pts) >= 2 {
		// velocity: deviation change from last window point to now
		velocity = math.Abs(current - pts[len(pts)-1].Deviation)
		// trend: linear regression slope over window (positive = worsening)
		trend = linearSlope(pts)
		// volatility: population std-dev of deviations in window
		volatility = stdDev(pts)
	}

	// ── Window-level crash detection (computed early, used for risk score) ──
	// Compare window-start prices to current snap prices for each volatile asset.
	// A sustained 20% drop over the window = max volatile signal (100).
	currentVolatilePrices := make(map[string]float64)
	for _, f := range pyth.VolatileFeeds() {
		if pd, ok := snap.All[f.Symbol]; ok && pd.Price > 0 {
			currentVolatilePrices[f.Symbol] = pd.Price
		}
	}

	var windowCrashPct float64
	crashSlot := -1
	crashSymbol := ""
	if len(pts) >= 1 {
		first := pts[0]
		for _, f := range pyth.VolatileFeeds() {
			firstP, ok1 := first.VolatilePrices[f.Symbol]
			lastP, ok2 := currentVolatilePrices[f.Symbol]
			if !ok2 && len(pts) >= 2 {
				lastP, ok2 = pts[len(pts)-1].VolatilePrices[f.Symbol]
			}
			if ok1 && ok2 && firstP > 0 && lastP > 0 {
				drop := (firstP - lastP) / firstP * 100 // positive = price dropped
				if drop > windowCrashPct {
					windowCrashPct = drop
					crashSlot = f.VaultSlot
					crashSymbol = f.Symbol
				}
			}
		}
	}
	// 10% sustained crash over the window = full volatile signal (100).
	// Threshold lowered from 20% to 10% so a real 10%+ drop registers as critical.
	volatileScore := math.Min(100, windowCrashPct/10.0*100)

	// ── Normalize each component to 0–100 ─────────────────────────────────
	devScore := math.Min(100, current/0.10*100)
	velScore := math.Min(100, velocity/0.02*100)
	trendScore := math.Min(100, math.Max(0, trend/0.005*100))
	volScore := math.Min(100, volatility/0.05*100)

	// ── Weighted composite ─────────────────────────────────────────────────
	// Normal market: Stable peg 35% | velocity 15% | trend 10% | vol 10% | crash 30%
	// Crisis market: when crash signal dominates (>50), shift weights so crash
	//               drives the overall score — mirrors how a real risk manager
	//               would treat a major drawdown vs minor peg noise.
	var raw float64
	if volatileScore > 50 {
		// Crisis mode: crash takes 65% weight, stable components share 35%
		raw = devScore*0.15 + velScore*0.08 + trendScore*0.06 + volScore*0.06 + volatileScore*0.65
	} else {
		raw = devScore*0.35 + velScore*0.15 + trendScore*0.10 + volScore*0.10 + volatileScore*0.30
	}

	// ── Strategy sensitivity ───────────────────────────────────────────────
	var sensitivity, threshold float64
	switch strategyMode {
	case StrategyModeSafe:
		sensitivity = 0.7
		threshold = 20
	case StrategyModeYieldV2:
		sensitivity = 1.5
		threshold = 5
	default: // BALANCED
		sensitivity = 1.0
		threshold = 10
	}

	riskLevel := math.Min(100, math.Round(raw*sensitivity*100)/100)

	s := ScoreV2{
		RiskLevel:      riskLevel,
		Deviation:      current,
		Trend:          trend,
		Velocity:       velocity,
		Volatility:     volatility,
		StableRisk:     math.Round(devScore*100) / 100,
		VolatileRisk:   math.Round(volatileScore*100) / 100,
		VolatilePrices: currentVolatilePrices,
		FromIndex:      -1,
		ToIndex:        -1,
		WindowSize:     len(pts),
	}

	if riskLevel < threshold {
		s.Action = "hold"
		s.Summary = fmt.Sprintf(
			"Risk %.1f < threshold %.0f | dev=%.5f%% vel=%.6f trend=%.6f vol=%.6f crash=%.2f%% — HOLD",
			riskLevel, threshold, current, velocity, trend, volatility, windowCrashPct,
		)
		w.mu.Lock()
		w.lastScore = s
		w.mu.Unlock()
		return s
	}

	var total uint64
	for _, b := range balances {
		total += b
	}
	if total == 0 {
		s.Action = "hold"
		s.Summary = "Vault empty — HOLD"
		w.mu.Lock()
		w.lastScore = s
		w.mu.Unlock()
		return s
	}

	// ── Direction ────────────────────────────────────────────────────────
	// Volatile crash takes priority: move crashing asset to USDC.
	// Threshold: 3% sustained drop over the window.
	if crashSlot >= 0 && windowCrashPct > 3.0 {
		s.FromIndex = crashSlot
		s.ToIndex = 0 // USDC is always slot 0
		s.Action = "rebalance"
		s.SuggestedFraction = math.Min(0.5, windowCrashPct/50)
		s.Summary = fmt.Sprintf(
			"Risk %.1f/100 | %s -%.1f%% crash over window → REBALANCE [slot%d→USDC] %.1f%%",
			riskLevel, crashSymbol, windowCrashPct, crashSlot, s.SuggestedFraction*100,
		)
	} else if snap.USDC.Price > snap.USDT.Price {
		// Stablecoin depeg: sell expensive USDC, buy USDT
		s.FromIndex = 0
		s.ToIndex = 1
		s.Action = "rebalance"
		s.SuggestedFraction = math.Min(0.5, riskLevel/200)
		s.Summary = fmt.Sprintf(
			"Risk %.1f/100 | dev=%.5f%% vel=%.6f trend=%.6f vol=%.6f → REBALANCE [USDC→USDT] %.1f%%",
			riskLevel, current, velocity, trend, volatility, s.SuggestedFraction*100,
		)
	} else {
		// Stablecoin depeg: sell expensive USDT, buy USDC
		s.FromIndex = 1
		s.ToIndex = 0
		s.Action = "rebalance"
		s.SuggestedFraction = math.Min(0.5, riskLevel/200)
		s.Summary = fmt.Sprintf(
			"Risk %.1f/100 | dev=%.5f%% vel=%.6f trend=%.6f vol=%.6f → REBALANCE [USDT→USDC] %.1f%%",
			riskLevel, current, velocity, trend, volatility, s.SuggestedFraction*100,
		)
	}

	w.mu.Lock()
	w.lastScore = s
	w.mu.Unlock()
	return s
}

// LastScore returns the most recently computed score (safe for concurrent reads).
func (w *WindowedScorer) LastScore() ScoreV2 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastScore
}

// ── Math helpers ──────────────────────────────────────────────────────────────

// linearSlope returns the linear-regression slope of deviations over pts.
// Positive slope = deviation is growing (risk increasing).
func linearSlope(pts []PricePoint) float64 {
	n := float64(len(pts))
	if n < 2 {
		return 0
	}
	var sumX, sumY, sumXY, sumX2 float64
	for i, pt := range pts {
		x := float64(i)
		y := pt.Deviation
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0
	}
	return (n*sumXY - sumX*sumY) / denom
}

// stdDev returns the population standard deviation of deviations in the window.
func stdDev(pts []PricePoint) float64 {
	n := float64(len(pts))
	if n < 2 {
		return 0
	}
	var sum float64
	for _, pt := range pts {
		sum += pt.Deviation
	}
	mean := sum / n
	var variance float64
	for _, pt := range pts {
		d := pt.Deviation - mean
		variance += d * d
	}
	return math.Sqrt(variance / n)
}
