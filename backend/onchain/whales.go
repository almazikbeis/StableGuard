// whales.go — aggregates on-chain liquidity signals into a whale-risk score.
package onchain

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// WhaleSignal is the aggregated on-chain risk from whale movements.
type WhaleSignal struct {
	Score       float64     // 0–100 composite whale risk
	Alerts      []WhaleFeed // individual signals that fired
	Summary     string
	UpdatedAt   time.Time
}

// WhaleFeed is a single on-chain event worth surfacing in the UI.
type WhaleFeed struct {
	DexID       string  `json:"dex_id"`
	Token       string  `json:"token"`
	Signal      string  `json:"signal"`   // "sell_pressure" | "low_liquidity" | "price_drop" | "volume_spike"
	Severity    string  `json:"severity"` // "low" | "medium" | "high"
	Value       float64 `json:"value"`
	Description string  `json:"description"`
}

// ── Aggregator ─────────────────────────────────────────────────────────────

type Aggregator struct {
	mu        sync.RWMutex
	lastSignal WhaleSignal
	cacheTTL  time.Duration
	fetchedAt time.Time
}

func NewAggregator() *Aggregator {
	return &Aggregator{
		cacheTTL: 2 * time.Minute,
		lastSignal: WhaleSignal{
			Summary:   "Initializing on-chain monitor…",
			UpdatedAt: time.Now(),
		},
	}
}

// Fetch queries DexScreener for USDC + USDT pairs and computes whale risk.
// Results are cached for 2 minutes.
func (a *Aggregator) Fetch() WhaleSignal {
	a.mu.RLock()
	if time.Since(a.fetchedAt) < a.cacheTTL {
		sig := a.lastSignal
		a.mu.RUnlock()
		return sig
	}
	a.mu.RUnlock()

	sig := a.compute()

	a.mu.Lock()
	a.lastSignal = sig
	a.fetchedAt = time.Now()
	a.mu.Unlock()

	return sig
}

// Last returns the cached signal without triggering a fetch.
func (a *Aggregator) Last() WhaleSignal {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastSignal
}

func (a *Aggregator) compute() WhaleSignal {
	var alerts []WhaleFeed
	totalScore := 0.0
	count := 0.0

	for _, entry := range []struct {
		mint   string
		symbol string
	}{
		{MintUSDC, "USDC"},
		{MintUSDT, "USDT"},
	} {
		pairs, err := FetchTopPairs(entry.mint, 5)
		if err != nil {
			continue
		}
		for _, p := range pairs {
			score, feeds := analyzePair(p, entry.symbol)
			totalScore += score
			count++
			alerts = append(alerts, feeds...)
		}
	}

	var compositeScore float64
	if count > 0 {
		compositeScore = math.Min(100, totalScore/count)
	}

	summary := buildSummary(compositeScore, alerts)

	return WhaleSignal{
		Score:     compositeScore,
		Alerts:    alerts,
		Summary:   summary,
		UpdatedAt: time.Now(),
	}
}

// analyzePair scores a single DEX pair for whale signals.
func analyzePair(p PairInfo, symbol string) (float64, []WhaleFeed) {
	var score float64
	var feeds []WhaleFeed

	// 1. Sell pressure: sells >> buys in last hour
	buys := p.Txns.H1.Buys
	sells := p.Txns.H1.Sells
	if buys+sells > 10 {
		ratio := float64(sells) / float64(buys+sells)
		if ratio > 0.70 {
			severity := "medium"
			pts := 25.0
			if ratio > 0.85 {
				severity = "high"
				pts = 50.0
			}
			score += pts
			feeds = append(feeds, WhaleFeed{
				DexID:       p.DexID,
				Token:       symbol,
				Signal:      "sell_pressure",
				Severity:    severity,
				Value:       ratio * 100,
				Description: fmt.Sprintf("%s on %s: %.0f%% of txns are sells (1h)", symbol, p.DexID, ratio*100),
			})
		}
	}

	// 2. Price drop > 0.05% in 1h
	if p.PriceChange.H1 < -0.05 {
		pts := math.Min(60, math.Abs(p.PriceChange.H1)*600)
		severity := "medium"
		if p.PriceChange.H1 < -0.2 {
			severity = "high"
		}
		score += pts
		feeds = append(feeds, WhaleFeed{
			DexID:       p.DexID,
			Token:       symbol,
			Signal:      "price_drop",
			Severity:    severity,
			Value:       p.PriceChange.H1,
			Description: fmt.Sprintf("%s price dropped %.3f%% on %s in last hour", symbol, p.PriceChange.H1, p.DexID),
		})
	}

	// 3. Low liquidity < $1M — vulnerable pool
	if p.Liquidity.USD > 0 && p.Liquidity.USD < 1_000_000 {
		pts := math.Min(30, (1_000_000-p.Liquidity.USD)/33_333)
		score += pts
		feeds = append(feeds, WhaleFeed{
			DexID:       p.DexID,
			Token:       symbol,
			Signal:      "low_liquidity",
			Severity:    "low",
			Value:       p.Liquidity.USD,
			Description: fmt.Sprintf("%s/%s on %s: only $%.0fK liquidity", symbol, p.QuoteSymbol, p.DexID, p.Liquidity.USD/1000),
		})
	}

	// 4. Volume spike > $5M in 1h — unusual activity
	if p.Volume.H1 > 5_000_000 {
		pts := math.Min(20, p.Volume.H1/1_000_000)
		score += pts
		feeds = append(feeds, WhaleFeed{
			DexID:       p.DexID,
			Token:       symbol,
			Signal:      "volume_spike",
			Severity:    "medium",
			Value:       p.Volume.H1,
			Description: fmt.Sprintf("$%.1fM volume on %s/%s in last hour", p.Volume.H1/1e6, symbol, p.DexID),
		})
	}

	return math.Min(100, score), feeds
}

func buildSummary(score float64, alerts []WhaleFeed) string {
	if len(alerts) == 0 {
		return fmt.Sprintf("On-chain activity normal — whale score %.0f/100", score)
	}
	high := 0
	for _, a := range alerts {
		if a.Severity == "high" {
			high++
		}
	}
	if high > 0 {
		return fmt.Sprintf("⚠ %d high-severity whale signal(s) detected — score %.0f/100", high, score)
	}
	return fmt.Sprintf("%d on-chain signal(s) active — whale score %.0f/100", len(alerts), score)
}
