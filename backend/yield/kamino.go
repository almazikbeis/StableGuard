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

// kaminoBaseURL is the Kamino Finance public API.
// Docs: https://docs.kamino.finance/
const kaminoBaseURL = "https://api.kamino.finance"

// Known Kamino Lend market addresses (mainnet).
// Main market, JLP market, Altcoin market
var kaminoMarkets = []string{
	"7u3HeL2w6L1YfPHfVwZdmBbTZTLLhtjJxHdJC1ZKYWJR", // Main market
}

// Stablecoin mint addresses on Solana mainnet.
var stableMints = map[string]string{
	"USDC":  "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
	"USDT":  "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
	"PYUSD": "2b1kV6DkPAnxd5ixfnxCpjxmKwqjjaYmCZfHsFu24GXo",
}

// KaminoAdapter fetches APY data from Kamino Finance Lend.
type KaminoAdapter struct {
	client *http.Client
}

func NewKaminoAdapter() *KaminoAdapter {
	return &KaminoAdapter{
		client: &http.Client{Timeout: 8 * time.Second},
	}
}

// kaminoReserveResp mirrors the Kamino Lend API response shape.
type kaminoReserveResp struct {
	Reserve struct {
		Address string `json:"address"`
		Mint    string `json:"mint"`
		Symbol  string `json:"symbol"`
	} `json:"reserve"`
	Metrics struct {
		SupplyInterestAPY float64 `json:"supplyInterestAPY"`
		BorrowInterestAPY float64 `json:"borrowInterestAPY"`
		TotalSupply       float64 `json:"totalSupply"`
		UtilizationRate   float64 `json:"utilizationRate"`
	} `json:"metrics"`
}

func (k *KaminoAdapter) FetchOpportunities(ctx context.Context) ([]Opportunity, error) {
	// 1. Try DeFiLlama (real, publicly accessible)
	if opps, err := FetchDefiLlamaPools(ctx, ProtocolKamino); err == nil && len(opps) > 0 {
		log.Printf("[yield/kamino] DeFiLlama: %d pools (live)", len(opps))
		return opps, nil
	}
	// 2. Try Kamino's own API
	for _, marketAddr := range kaminoMarkets {
		opps, err := k.fetchMarket(ctx, marketAddr)
		if err == nil && len(opps) > 0 {
			return opps, nil
		}
	}
	log.Printf("[yield/kamino] all sources failed, using estimates")
	return k.fallback(), nil
}

func (k *KaminoAdapter) fetchMarket(ctx context.Context, marketAddr string) ([]Opportunity, error) {
	url := fmt.Sprintf("%s/kamino-market/%s/reserves", kaminoBaseURL, marketAddr)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "StableGuard/1.0")

	resp, err := k.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}

	// Try array format first
	var reserves []kaminoReserveResp
	if err := json.NewDecoder(resp.Body).Decode(&reserves); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	now := time.Now().Unix()
	var opps []Opportunity
	for _, r := range reserves {
		sym := r.Reserve.Symbol
		supplyAPY := r.Metrics.SupplyInterestAPY
		// Kamino API may return values as decimals (0.082) or percentages (8.2)
		if supplyAPY > 0 && supplyAPY < 1 {
			supplyAPY *= 100
		}
		borrowAPY := r.Metrics.BorrowInterestAPY
		if borrowAPY > 0 && borrowAPY < 1 {
			borrowAPY *= 100
		}
		opps = append(opps, Opportunity{
			Protocol:    ProtocolKamino,
			DisplayName: "Kamino Lend",
			URL:         "https://app.kamino.finance/lending",
			Token:       strings.ToUpper(sym),
			AssetType:   AssetTypeFor(strings.ToUpper(sym)),
			SupplyAPY:   supplyAPY,
			BorrowAPY:   borrowAPY,
			TVLMillions: r.Metrics.TotalSupply / 1_000_000,
			UtilRate:    r.Metrics.UtilizationRate,
			UpdatedAt:   now,
			IsLive:      true,
		})
	}

	return opps, nil
}

// fallback returns realistic Kamino APY estimates when the API is unavailable.
// Values are based on typical Kamino Lend mainnet rates (2024–2025).
func (k *KaminoAdapter) fallback() []Opportunity {
	now := time.Now().Unix()
	return []Opportunity{
		{Protocol: ProtocolKamino, DisplayName: "Kamino Lend", URL: "https://app.kamino.finance/lending", Token: "USDC", AssetType: "stable",   SupplyAPY: 7.82, BorrowAPY: 12.40, TVLMillions: 420, UtilRate: 0.71, UpdatedAt: now, IsLive: false},
		{Protocol: ProtocolKamino, DisplayName: "Kamino Lend", URL: "https://app.kamino.finance/lending", Token: "USDT", AssetType: "stable",   SupplyAPY: 8.15, BorrowAPY: 13.10, TVLMillions: 280, UtilRate: 0.74, UpdatedAt: now, IsLive: false},
		{Protocol: ProtocolKamino, DisplayName: "Kamino Lend", URL: "https://app.kamino.finance/lending", Token: "SOL",  AssetType: "volatile", SupplyAPY: 6.24, BorrowAPY: 9.80,  TVLMillions: 890, UtilRate: 0.68, UpdatedAt: now, IsLive: false},
		{Protocol: ProtocolKamino, DisplayName: "Kamino Lend", URL: "https://app.kamino.finance/lending", Token: "ETH",  AssetType: "volatile", SupplyAPY: 2.95, BorrowAPY: 5.20,  TVLMillions: 145, UtilRate: 0.61, UpdatedAt: now, IsLive: false},
		{Protocol: ProtocolKamino, DisplayName: "Kamino Lend", URL: "https://app.kamino.finance/lending", Token: "BTC",  AssetType: "volatile", SupplyAPY: 1.42, BorrowAPY: 3.10,  TVLMillions: 320, UtilRate: 0.52, UpdatedAt: now, IsLive: false},
	}
}

func isStable(sym string) bool {
	s := strings.ToUpper(sym)
	for k := range stableMints {
		if strings.ToUpper(k) == s {
			return true
		}
	}
	return false
}
