// Package pyth fetches real-time price feeds from Pyth Network's Hermes REST API.
// Price feed IDs: https://pyth.network/developers/price-feed-ids
package pyth

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

// Kept for backward-compat with any code that references these constants directly.
const (
	FeedIDUSDC = "0xeaa020c61cc479712813461ce153894a96a6c00b21ed0cfc2798d1f9a9e9c94a"
	FeedIDUSDT = "0x2b89b9dc8fdf9f34709a5b106b472f0f39bb6ca9ce04b0fd7f2e971688e2e53b"
)

// PriceData holds a parsed price from Pyth Hermes.
type PriceData struct {
	FeedID      string
	Price       float64 // e.g. 1.0002 USD
	Confidence  float64 // ±conf in USD
	PublishTime time.Time
}

// PriceSnapshot holds the latest prices for all monitored stablecoins.
//
// All is the canonical map — keyed by Symbol (e.g. "USDC").
// USDC and USDT are kept for backward-compat with existing callers.
type PriceSnapshot struct {
	All       map[string]PriceData // symbol → price; always populated
	USDC      PriceData            // backward-compat alias for All["USDC"]
	USDT      PriceData            // backward-compat alias for All["USDT"]
	FetchedAt time.Time
}

// Deviation returns |USDC.Price - USDT.Price| as a percentage.
// Kept for backward-compat; prefer DeviationBetween for multi-token use.
func (s PriceSnapshot) Deviation() float64 {
	return s.DeviationBetween("USDC", "USDT")
}

// DeviationBetween returns |price(a) - price(b)| as a percentage.
func (s PriceSnapshot) DeviationBetween(a, b string) float64 {
	pa, ok1 := s.All[a]
	pb, ok2 := s.All[b]
	if !ok1 || !ok2 || pa.Price == 0 || pb.Price == 0 {
		return 0
	}
	diff := math.Abs(pa.Price - pb.Price)
	avg := (pa.Price + pb.Price) / 2
	if avg == 0 {
		return 0
	}
	return (diff / avg) * 100
}

// MaxDeviation returns the maximum pairwise deviation among stablecoin tokens only.
// Volatile assets (BTC/ETH/SOL) are excluded to avoid false circuit-breaker triggers.
func (s PriceSnapshot) MaxDeviation() float64 {
	stables := StableFeeds()
	tokens := make([]PriceData, 0, len(stables))
	for _, f := range stables {
		if pd, ok := s.All[f.Symbol]; ok && pd.Price > 0 {
			tokens = append(tokens, pd)
		}
	}
	max := 0.0
	for i := 0; i < len(tokens); i++ {
		for j := i + 1; j < len(tokens); j++ {
			a, b := tokens[i].Price, tokens[j].Price
			if a == 0 || b == 0 {
				continue
			}
			d := math.Abs(a-b) / ((a + b) / 2) * 100
			if d > max {
				max = d
			}
		}
	}
	return max
}

type hermesResponse struct {
	Parsed []struct {
		ID    string `json:"id"`
		Price struct {
			Price       string `json:"price"`
			Conf        string `json:"conf"`
			Expo        int    `json:"expo"`
			PublishTime int64  `json:"publish_time"`
		} `json:"price"`
	} `json:"parsed"`
}

// Monitor fetches Pyth prices on demand.
type Monitor struct {
	hermesURL string
	client    *http.Client
}

// New creates a new Pyth price monitor.
func New(hermesURL string) *Monitor {
	return &Monitor{
		hermesURL: hermesURL,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// FetchSnapshot fetches latest prices for all ActiveFeeds from Pyth Hermes.
func (m *Monitor) FetchSnapshot() (*PriceSnapshot, error) {
	ids := AllFeedIDs()
	params := make([]string, len(ids))
	for i, id := range ids {
		params[i] = "ids[]=" + id
	}
	url := fmt.Sprintf("%s/v2/updates/price/latest?%s&parsed=true",
		m.hermesURL, strings.Join(params, "&"))

	resp, err := m.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("pyth fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pyth http %d", resp.StatusCode)
	}

	var hr hermesResponse
	if err := json.NewDecoder(resp.Body).Decode(&hr); err != nil {
		return nil, fmt.Errorf("pyth decode: %w", err)
	}

	return buildSnapshot(&hr), nil
}

// buildSnapshot turns a hermesResponse into a PriceSnapshot with All map populated.
func buildSnapshot(hr *hermesResponse) *PriceSnapshot {
	snap := &PriceSnapshot{
		All:       make(map[string]PriceData, len(ActiveFeeds)),
		FetchedAt: time.Now(),
	}
	for _, p := range hr.Parsed {
		pd, err := parsePrice(p.ID, p.Price.Price, p.Price.Conf, p.Price.Expo, p.Price.PublishTime)
		if err != nil {
			continue
		}
		// Match by feed ID (with or without 0x)
		feed, ok := FeedByID(p.ID)
		if !ok {
			continue
		}
		snap.All[feed.Symbol] = pd
	}
	// Populate backward-compat fields
	snap.USDC = snap.All["USDC"]
	snap.USDT = snap.All["USDT"]
	return snap
}

func parsePrice(id, priceStr, confStr string, expo int, publishTime int64) (PriceData, error) {
	var priceRaw, confRaw int64
	if _, err := fmt.Sscanf(priceStr, "%d", &priceRaw); err != nil {
		return PriceData{}, err
	}
	fmt.Sscanf(confStr, "%d", &confRaw)

	scale := math.Pow10(expo)
	return PriceData{
		FeedID:      id,
		Price:       float64(priceRaw) * scale,
		Confidence:  float64(confRaw) * scale,
		PublishTime: time.Unix(publishTime, 0),
	}, nil
}
