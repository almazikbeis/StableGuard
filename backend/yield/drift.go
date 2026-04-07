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
	// 1. Try DeFiLlama (real, publicly accessible)
	if opps, err := FetchDefiLlamaPools(ctx, ProtocolDrift); err == nil && len(opps) > 0 {
		log.Printf("[yield/drift] DeFiLlama: %d pools (live)", len(opps))
		return opps, nil
	}

	// 2. Try Drift's own S3 stats endpoint
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
		opps = append(opps, Opportunity{
			Protocol:    ProtocolDrift,
			DisplayName: "Drift Lend",
			URL:         "https://app.drift.trade/earn",
			Token:       strings.ToUpper(m.Symbol),
			AssetType:   AssetTypeFor(strings.ToUpper(m.Symbol)),
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
		{Protocol: ProtocolDrift, DisplayName: "Drift Lend", URL: "https://app.drift.trade/earn", Token: "USDC", AssetType: "stable",   SupplyAPY: 9.14, BorrowAPY: 14.30, TVLMillions: 98,  UtilRate: 0.76, UpdatedAt: now, IsLive: false},
		{Protocol: ProtocolDrift, DisplayName: "Drift Lend", URL: "https://app.drift.trade/earn", Token: "USDT", AssetType: "stable",   SupplyAPY: 8.67, BorrowAPY: 13.90, TVLMillions: 74,  UtilRate: 0.72, UpdatedAt: now, IsLive: false},
		{Protocol: ProtocolDrift, DisplayName: "Drift Lend", URL: "https://app.drift.trade/earn", Token: "SOL",  AssetType: "volatile", SupplyAPY: 7.45, BorrowAPY: 11.60, TVLMillions: 220, UtilRate: 0.73, UpdatedAt: now, IsLive: false},
		{Protocol: ProtocolDrift, DisplayName: "Drift Lend", URL: "https://app.drift.trade/earn", Token: "ETH",  AssetType: "volatile", SupplyAPY: 2.48, BorrowAPY: 4.70,  TVLMillions: 65,  UtilRate: 0.60, UpdatedAt: now, IsLive: false},
		{Protocol: ProtocolDrift, DisplayName: "Drift Lend", URL: "https://app.drift.trade/earn", Token: "BTC",  AssetType: "volatile", SupplyAPY: 1.65, BorrowAPY: 3.40,  TVLMillions: 88,  UtilRate: 0.55, UpdatedAt: now, IsLive: false},
	}
}
