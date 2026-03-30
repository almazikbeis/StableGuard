// Package pyth fetches real-time price feeds from Pyth Network's Hermes REST API.
// Price feed IDs: https://pyth.network/developers/price-feed-ids
package pyth

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"
)

// Pyth price feed IDs for USDC and USDT on Solana
const (
	FeedIDUSDC = "0xeaa020c61cc479712813461ce153894a96a6c00b21ed0cfc2798d1f9a9e9c94a"
	FeedIDUSDT = "0x2b89b9dc8fdf9f34709a5b106b472f0f39bb6ca9ce04b0fd7f2e971688e2e53b"
)

// PriceData holds a parsed price from Pyth Hermes.
type PriceData struct {
	FeedID     string
	Price      float64 // e.g. 1.0002 USD
	Confidence float64 // ±conf in USD
	PublishTime time.Time
}

// PriceSnapshot holds a snapshot of both USDC and USDT prices.
type PriceSnapshot struct {
	USDC      PriceData
	USDT      PriceData
	FetchedAt time.Time
}

// Deviation returns |USDC.Price - USDT.Price| as a percentage.
func (s PriceSnapshot) Deviation() float64 {
	diff := math.Abs(s.USDC.Price - s.USDT.Price)
	avg := (s.USDC.Price + s.USDT.Price) / 2
	if avg == 0 {
		return 0
	}
	return (diff / avg) * 100
}

type hermesResponse struct {
	Parsed []struct {
		ID    string `json:"id"`
		Price struct {
			Price    string `json:"price"`
			Conf     string `json:"conf"`
			Expo     int    `json:"expo"`
			PublishTime int64 `json:"publish_time"`
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

// FetchSnapshot fetches latest USDC and USDT prices from Pyth Hermes.
func (m *Monitor) FetchSnapshot() (*PriceSnapshot, error) {
	url := fmt.Sprintf(
		"%s/v2/updates/price/latest?ids[]=%s&ids[]=%s&parsed=true",
		m.hermesURL, FeedIDUSDC, FeedIDUSDT,
	)

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

	snap := &PriceSnapshot{FetchedAt: time.Now()}
	for _, p := range hr.Parsed {
		pd, err := parsePrice(p.ID, p.Price.Price, p.Price.Conf, p.Price.Expo, p.Price.PublishTime)
		if err != nil {
			continue
		}
		switch "0x" + p.ID {
		case FeedIDUSDC:
			snap.USDC = pd
		case FeedIDUSDT:
			snap.USDT = pd
		}
	}

	return snap, nil
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
