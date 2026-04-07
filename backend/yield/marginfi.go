package yield

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// marginfiBaseURL is the Marginfi public API.
// Docs: https://docs.marginfi.com/
const marginfiBaseURL = "https://production.api.marginfi.com"

// MarginfiAdapter fetches APY data from Marginfi.
type MarginfiAdapter struct {
	client *http.Client
}

func NewMarginfiAdapter() *MarginfiAdapter {
	return &MarginfiAdapter{
		client: &http.Client{Timeout: 8 * time.Second},
	}
}

// marginfiBank mirrors the Marginfi /v0/banks response shape.
type marginfiBank struct {
	Address string `json:"address"`
	Mint    struct {
		Address string `json:"address"`
		Symbol  string `json:"symbol"`
	} `json:"mint"`
	DepositRate float64 `json:"deposit_rate"` // annual rate 0-1 (e.g. 0.082 = 8.2%)
	BorrowRate  float64 `json:"borrow_rate"`
	TotalAssets float64 `json:"total_assets_usd"`
	UtilRate    float64 `json:"utilization_rate"`
}

// marginfiEndpoints lists known MarginFi API endpoint variants to try.
var marginfiEndpoints = []string{
	marginfiBaseURL + "/v0/banks",
	"https://marginfi-v2-ui-data.s3.amazonaws.com/banks.json",
}

func (m *MarginfiAdapter) FetchOpportunities(ctx context.Context) ([]Opportunity, error) {
	// 1. Try DeFiLlama (real, publicly accessible)
	if opps, err := FetchDefiLlamaPools(ctx, ProtocolMarginfi); err == nil && len(opps) > 0 {
		log.Printf("[yield/marginfi] DeFiLlama: %d pools (live)", len(opps))
		return opps, nil
	}
	// 2. Try MarginFi's own API
	for _, endpoint := range marginfiEndpoints {
		opps, err := m.fetchEndpoint(ctx, endpoint)
		if err == nil && len(opps) > 0 {
			return opps, nil
		}
		log.Printf("[yield/marginfi] endpoint %s failed: %v", endpoint, err)
	}
	log.Printf("[yield/marginfi] all sources failed, using estimates")
	return m.fallback(), nil
}

func (m *MarginfiAdapter) fetchEndpoint(ctx context.Context, endpoint string) ([]Opportunity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "StableGuard/1.0")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}

	var banks []marginfiBank
	if err := json.NewDecoder(resp.Body).Decode(&banks); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	now := time.Now().Unix()
	var opps []Opportunity
	for _, b := range banks {
		sym := b.Mint.Symbol
		if sym == "" {
			// Some endpoints use a flat symbol field
			sym = b.Mint.Address
		}
		depositRate := b.DepositRate
		if depositRate > 0 && depositRate < 1 {
			depositRate *= 100 // convert from decimal to percentage
		}
		borrowRate := b.BorrowRate
		if borrowRate > 0 && borrowRate < 1 {
			borrowRate *= 100
		}
		opps = append(opps, Opportunity{
			Protocol:    ProtocolMarginfi,
			DisplayName: "Marginfi",
			URL:         "https://app.marginfi.com/",
			Token:       strings.ToUpper(sym),
			AssetType:   AssetTypeFor(strings.ToUpper(sym)),
			SupplyAPY:   depositRate,
			BorrowAPY:   borrowRate,
			TVLMillions: b.TotalAssets / 1_000_000,
			UtilRate:    b.UtilRate,
			UpdatedAt:   now,
			IsLive:      true,
		})
	}

	if len(opps) == 0 {
		return nil, fmt.Errorf("no opportunities found in response")
	}
	return opps, nil
}

// fallback returns realistic Marginfi APY estimates.
func (m *MarginfiAdapter) fallback() []Opportunity {
	now := time.Now().Unix()
	return []Opportunity{
		{Protocol: ProtocolMarginfi, DisplayName: "Marginfi", URL: "https://app.marginfi.com/", Token: "USDC", AssetType: "stable",   SupplyAPY: 6.91, BorrowAPY: 11.20, TVLMillions: 185, UtilRate: 0.68, UpdatedAt: now, IsLive: false},
		{Protocol: ProtocolMarginfi, DisplayName: "Marginfi", URL: "https://app.marginfi.com/", Token: "USDT", AssetType: "stable",   SupplyAPY: 7.34, BorrowAPY: 11.80, TVLMillions: 142, UtilRate: 0.70, UpdatedAt: now, IsLive: false},
		{Protocol: ProtocolMarginfi, DisplayName: "Marginfi", URL: "https://app.marginfi.com/", Token: "SOL",  AssetType: "volatile", SupplyAPY: 5.88, BorrowAPY: 9.10,  TVLMillions: 510, UtilRate: 0.65, UpdatedAt: now, IsLive: false},
		{Protocol: ProtocolMarginfi, DisplayName: "Marginfi", URL: "https://app.marginfi.com/", Token: "ETH",  AssetType: "volatile", SupplyAPY: 2.71, BorrowAPY: 4.90,  TVLMillions: 98,  UtilRate: 0.58, UpdatedAt: now, IsLive: false},
		{Protocol: ProtocolMarginfi, DisplayName: "Marginfi", URL: "https://app.marginfi.com/", Token: "BTC",  AssetType: "volatile", SupplyAPY: 1.18, BorrowAPY: 2.85,  TVLMillions: 210, UtilRate: 0.49, UpdatedAt: now, IsLive: false},
	}
}
