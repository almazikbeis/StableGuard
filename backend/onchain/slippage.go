package onchain

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"sync"
	"time"

	"stableguard-backend/jupiter"
)

// DefaultInputMint and DefaultOutputMint are used when no mints are specified.
// MintUSDC / MintUSDT are defined in dexscreener.go and shared within the package.

// SlippageMeasurement captures price impact at 3 trade sizes.
type SlippageMeasurement struct {
	ImpactAt10K  float64   `json:"impact_10k"`
	ImpactAt100K float64   `json:"impact_100k"`
	ImpactAt1M   float64   `json:"impact_1m"`
	LiquidityScore int     `json:"liquidity_score"` // 0–100
	DrainDetected  bool    `json:"drain_detected"`
	InputMint    string    `json:"input_mint"`
	OutputMint   string    `json:"output_mint"`
	MeasuredAt   time.Time `json:"measured_at"`
}

// SlippageWindow holds a rolling window of measurements for drain detection.
type SlippageWindow struct {
	measurements []SlippageMeasurement
	maxSize      int
}

func newSlippageWindow(size int) *SlippageWindow {
	return &SlippageWindow{maxSize: size}
}

func (w *SlippageWindow) add(m SlippageMeasurement) {
	w.measurements = append(w.measurements, m)
	if len(w.measurements) > w.maxSize {
		w.measurements = w.measurements[1:]
	}
}

func (w *SlippageWindow) oldest() *SlippageMeasurement {
	if len(w.measurements) == 0 {
		return nil
	}
	return &w.measurements[0]
}

func (w *SlippageWindow) latest() *SlippageMeasurement {
	if len(w.measurements) == 0 {
		return nil
	}
	return &w.measurements[len(w.measurements)-1]
}

func (w *SlippageWindow) all() []SlippageMeasurement {
	out := make([]SlippageMeasurement, len(w.measurements))
	copy(out, w.measurements)
	return out
}

// SlippageAnalyzer measures on-chain liquidity depth via Jupiter quotes.
type SlippageAnalyzer struct {
	mu      sync.Mutex
	windows map[string]*SlippageWindow // key = "inputMint:outputMint"
}

// NewSlippageAnalyzer creates a new analyzer.
func NewSlippageAnalyzer() *SlippageAnalyzer {
	return &SlippageAnalyzer{
		windows: make(map[string]*SlippageWindow),
	}
}

// Measure fetches Jupiter quotes at 3 trade sizes and computes liquidity metrics.
// Returns gracefully (score=100, impact=0) if Jupiter is unavailable (devnet / offline).
func (a *SlippageAnalyzer) Measure(inputMint, outputMint string) SlippageMeasurement {
	impact10k := getImpact(inputMint, outputMint, 10_000*1_000_000)
	impact100k := getImpact(inputMint, outputMint, 100_000*1_000_000)
	impact1m := getImpact(inputMint, outputMint, 1_000_000*1_000_000)

	score := liquidityScore(impact100k)

	key := fmt.Sprintf("%s:%s", inputMint, outputMint)

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.windows[key] == nil {
		a.windows[key] = newSlippageWindow(5)
	}
	w := a.windows[key]

	drain := false
	oldest := w.oldest()
	if oldest != nil && oldest.ImpactAt100K > 0 && impact100k > 0 {
		// Drain detected: 3x degradation vs oldest measurement ≥ 45 min ago
		if time.Since(oldest.MeasuredAt) >= 45*time.Minute && impact100k/oldest.ImpactAt100K > 3.0 {
			drain = true
			log.Printf("[slippage] DRAIN DETECTED: %s→%s impact %.4f%% vs oldest %.4f%%",
				inputMint[:8], outputMint[:8], impact100k, oldest.ImpactAt100K)
		}
	}

	m := SlippageMeasurement{
		ImpactAt10K:    impact10k,
		ImpactAt100K:   impact100k,
		ImpactAt1M:     impact1m,
		LiquidityScore: score,
		DrainDetected:  drain,
		InputMint:      inputMint,
		OutputMint:     outputMint,
		MeasuredAt:     time.Now(),
	}

	w.add(m)
	return m
}

// Window returns all measurements in the sliding window for the given pair.
func (a *SlippageAnalyzer) Window(inputMint, outputMint string) []SlippageMeasurement {
	key := fmt.Sprintf("%s:%s", inputMint, outputMint)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.windows[key] == nil {
		return nil
	}
	return a.windows[key].all()
}

// getImpact fetches a Jupiter quote and returns the price impact percent (as float64).
// Returns 0.0 on error (Jupiter mainnet-only → graceful devnet fallback).
func getImpact(inputMint, outputMint string, amount uint64) float64 {
	resp, err := jupiter.GetQuote(jupiter.QuoteRequest{
		InputMint:  inputMint,
		OutputMint: outputMint,
		Amount:     amount,
	})
	if err != nil {
		return 0.0
	}
	f, err := strconv.ParseFloat(resp.PriceImpactPct, 64)
	if err != nil {
		return 0.0
	}
	return math.Abs(f)
}

// liquidityScore converts 100K trade impact to a 0–100 score (higher = more liquid).
func liquidityScore(impact float64) int {
	switch {
	case impact < 0.01:
		return 100
	case impact < 0.05:
		return 80
	case impact < 0.1:
		return 60
	case impact < 0.3:
		return 40
	case impact < 0.5:
		return 20
	default:
		return 0
	}
}
