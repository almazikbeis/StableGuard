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
	Time      time.Time
	USDCPrice float64
	USDTPrice float64
	Deviation float64 // |USDC-USDT| / avg * 100
}

// ScoreV2 holds the enhanced risk metrics from the windowed scorer.
type ScoreV2 struct {
	RiskLevel         float64 // 0–100 (weighted composite)
	Deviation         float64 // current price deviation %
	Trend             float64 // linear slope of deviation over window (positive = worsening)
	Velocity          float64 // absolute change from previous tick
	Volatility        float64 // std-dev of deviations in window
	FromIndex         int     // token slot to rebalance from (-1 = hold)
	ToIndex           int     // token slot to rebalance into  (-1 = hold)
	SuggestedFraction float64 // 0–0.5 of total vault
	Action            string  // "hold" | "rebalance"
	Summary           string
	WindowSize        int // how many points in window
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

	pt := PricePoint{
		Time:      snap.FetchedAt,
		USDCPrice: snap.USDC.Price,
		USDTPrice: snap.USDT.Price,
		Deviation: snap.Deviation(),
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

	// ── Normalize each component to 0–100 ─────────────────────────────────
	// Thresholds chosen for stablecoin context:
	//   0.10% deviation = full crisis
	//   0.02% velocity per tick = high velocity
	//   0.005% slope per tick = clear trend
	//   0.05% std-dev = high volatility
	devScore := math.Min(100, current/0.10*100)
	velScore := math.Min(100, velocity/0.02*100)
	trendScore := math.Min(100, math.Max(0, trend/0.005*100))
	volScore := math.Min(100, volatility/0.05*100)

	// ── Weighted composite ─────────────────────────────────────────────────
	raw := devScore*0.4 + velScore*0.2 + trendScore*0.2 + volScore*0.2

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
		RiskLevel:  riskLevel,
		Deviation:  current,
		Trend:      trend,
		Velocity:   velocity,
		Volatility: volatility,
		FromIndex:  -1,
		ToIndex:    -1,
		WindowSize: len(pts),
	}

	if riskLevel < threshold {
		s.Action = "hold"
		s.Summary = fmt.Sprintf(
			"Risk %.1f < threshold %.0f | dev=%.5f%% vel=%.6f trend=%.6f vol=%.6f — HOLD",
			riskLevel, threshold, current, velocity, trend, volatility,
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
	// Sell the overpriced token, buy the underpriced one.
	if snap.USDC.Price > snap.USDT.Price {
		s.FromIndex = 0 // USDC slot
		s.ToIndex = 1   // USDT slot
		s.Action = "rebalance"
	} else {
		s.FromIndex = 1 // USDT slot
		s.ToIndex = 0   // USDC slot
		s.Action = "rebalance"
	}

	s.SuggestedFraction = math.Min(0.5, riskLevel/200)
	s.Summary = fmt.Sprintf(
		"Risk %.1f/100 | dev=%.5f%% vel=%.6f trend=%.6f vol=%.6f → REBALANCE [%d→%d] %.1f%%",
		riskLevel, current, velocity, trend, volatility,
		s.FromIndex, s.ToIndex, s.SuggestedFraction*100,
	)

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
