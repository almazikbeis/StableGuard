package yield

import (
	"context"
	"encoding/json"
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

func (m *MarginfiAdapter) FetchOpportunities(ctx context.Context) ([]Opportunity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, marginfiBaseURL+"/v0/banks", nil)
	if err != nil {
		return m.fallback(), nil
	}
	req.Header.Set("Accept", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		log.Printf("[yield/marginfi] API unreachable, using estimates: %v", err)
		return m.fallback(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[yield/marginfi] API returned %d, using estimates", resp.StatusCode)
		return m.fallback(), nil
	}

	var banks []marginfiBank
	if err := json.NewDecoder(resp.Body).Decode(&banks); err != nil {
		return m.fallback(), nil
	}

	now := time.Now().Unix()
	var opps []Opportunity
	for _, b := range banks {
		sym := b.Mint.Symbol
		if !isStable(sym) {
			continue
		}
		opps = append(opps, Opportunity{
			Protocol:    ProtocolMarginfi,
			DisplayName: "Marginfi",
			URL:         "https://app.marginfi.com/",
			Token:       strings.ToUpper(sym),
			SupplyAPY:   b.DepositRate * 100,
			BorrowAPY:   b.BorrowRate * 100,
			TVLMillions: b.TotalAssets / 1_000_000,
			UtilRate:    b.UtilRate,
			UpdatedAt:   now,
			IsLive:      true,
		})
	}

	if len(opps) == 0 {
		return m.fallback(), nil
	}
	return opps, nil
}

// fallback returns realistic Marginfi APY estimates.
func (m *MarginfiAdapter) fallback() []Opportunity {
	now := time.Now().Unix()
	return []Opportunity{
		{
			Protocol: ProtocolMarginfi, DisplayName: "Marginfi",
			URL: "https://app.marginfi.com/", Token: "USDC",
			SupplyAPY: 6.91, BorrowAPY: 11.20, TVLMillions: 185, UtilRate: 0.68,
			UpdatedAt: now, IsLive: false,
		},
		{
			Protocol: ProtocolMarginfi, DisplayName: "Marginfi",
			URL: "https://app.marginfi.com/", Token: "USDT",
			SupplyAPY: 7.34, BorrowAPY: 11.80, TVLMillions: 142, UtilRate: 0.70,
			UpdatedAt: now, IsLive: false,
		},
	}
}
