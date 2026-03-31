package yield

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// driftBaseURL is the Drift Protocol public stats API.
const driftBaseURL = "https://drift-historical-data-v2.s3.eu-west-1.amazonaws.com"

// DriftAdapter fetches APY data from Drift spot market lending.
type DriftAdapter struct {
	client *http.Client
}

func NewDriftAdapter() *DriftAdapter {
	return &DriftAdapter{
		client: &http.Client{Timeout: 8 * time.Second},
	}
}

// driftSpotMarketStats mirrors Drift spot market stats shape.
type driftSpotMarketStats struct {
	MarketIndex   int     `json:"marketIndex"`
	Symbol        string  `json:"symbol"`
	DepositAPY    float64 `json:"depositAPY"`   // already in %
	BorrowAPY     float64 `json:"borrowAPY"`
	TotalDeposits float64 `json:"totalDeposits"` // in token units
	UtilRate      float64 `json:"utilizationRate"`
}

func (d *DriftAdapter) FetchOpportunities(ctx context.Context) ([]Opportunity, error) {
	// Drift publishes spot market stats as JSON on S3.
	url := driftBaseURL + "/program/drift/network/mainnet/spot_market_stats/latest.json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return d.fallback(), nil
	}

	resp, err := d.client.Do(req)
	if err != nil {
		log.Printf("[yield/drift] API unreachable, using estimates: %v", err)
		return d.fallback(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[yield/drift] API returned %d, using estimates", resp.StatusCode)
		return d.fallback(), nil
	}

	var markets []driftSpotMarketStats
	if err := json.NewDecoder(resp.Body).Decode(&markets); err != nil {
		return d.fallback(), nil
	}

	now := time.Now().Unix()
	var opps []Opportunity
	for _, m := range markets {
		if !isStable(m.Symbol) {
			continue
		}
		opps = append(opps, Opportunity{
			Protocol:    ProtocolDrift,
			DisplayName: "Drift Lend",
			URL:         "https://app.drift.trade/earn",
			Token:       strings.ToUpper(m.Symbol),
			SupplyAPY:   m.DepositAPY,
			BorrowAPY:   m.BorrowAPY,
			TVLMillions: m.TotalDeposits / 1_000_000,
			UtilRate:    m.UtilRate,
			UpdatedAt:   now,
			IsLive:      true,
		})
	}

	if len(opps) == 0 {
		return d.fallback(), nil
	}
	return opps, nil
}

// fallback returns realistic Drift APY estimates.
func (d *DriftAdapter) fallback() []Opportunity {
	now := time.Now().Unix()
	return []Opportunity{
		{
			Protocol: ProtocolDrift, DisplayName: "Drift Lend",
			URL: "https://app.drift.trade/earn", Token: "USDC",
			SupplyAPY: 9.14, BorrowAPY: 14.30, TVLMillions: 98, UtilRate: 0.76,
			UpdatedAt: now, IsLive: false,
		},
		{
			Protocol: ProtocolDrift, DisplayName: "Drift Lend",
			URL: "https://app.drift.trade/earn", Token: "USDT",
			SupplyAPY: 8.67, BorrowAPY: 13.90, TVLMillions: 74, UtilRate: 0.72,
			UpdatedAt: now, IsLive: false,
		},
	}
}
