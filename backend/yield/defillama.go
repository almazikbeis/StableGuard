package yield

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defiLlamaPoolsURL = "https://yields.llama.fi/pools"

// llamaPool is one pool entry from the DeFiLlama Yields API.
type llamaPool struct {
	Pool      string  `json:"pool"`
	Project   string  `json:"project"`
	Chain     string  `json:"chain"`
	Symbol    string  `json:"symbol"`
	APY       float64 `json:"apy"`
	APYBase   float64 `json:"apyBase"`
	APYReward float64 `json:"apyReward"`
	TVLUsd    float64 `json:"tvlUsd"`
}

// defiLlamaProjectMap maps our protocol names to DeFiLlama project slugs.
// Multiple slugs are tried in order; first match with APY>0 wins.
var defiLlamaProjectMap = map[Protocol][]string{
	ProtocolKamino:   {"kamino-lend"},
	ProtocolMarginfi: {"marginfi", "save", "marginfi-lst"},
	ProtocolDrift:    {"drift-protocol", "drift", "project-0"},
}

// defiLlamaTokens is the set of token symbols we care about.
var defiLlamaTokens = map[string]bool{
	"USDC": true, "USDT": true, "SOL": true,
	"ETH": true, "BTC": true, "PYUSD": true, "DAI": true,
}

// FetchDefiLlamaPools returns Solana lending opportunities from DeFiLlama
// for the given protocol name (ProtocolKamino, ProtocolMarginfi, ProtocolDrift).
// Returns nil, err if the API is unreachable.
func FetchDefiLlamaPools(ctx context.Context, protocol Protocol) ([]Opportunity, error) {
	slugs, ok := defiLlamaProjectMap[protocol]
	if !ok {
		return nil, fmt.Errorf("unknown protocol: %s", protocol)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, defiLlamaPoolsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "StableGuard/1.0")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("defillama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("defillama returned %d", resp.StatusCode)
	}

	var body struct {
		Data []llamaPool `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("defillama decode: %w", err)
	}

	// Index pools by project slug
	bySlug := map[string][]llamaPool{}
	for _, p := range body.Data {
		if strings.ToLower(p.Chain) != "solana" {
			continue
		}
		if !defiLlamaTokens[p.Symbol] {
			continue
		}
		if p.APY <= 0 {
			continue
		}
		slug := strings.ToLower(p.Project)
		bySlug[slug] = append(bySlug[slug], p)
	}

	// Pick best pools: for each token, use the entry with highest APY
	var pools []llamaPool
	for _, slug := range slugs {
		if ps, ok := bySlug[slug]; ok && len(ps) > 0 {
			pools = ps
			break
		}
	}
	if len(pools) == 0 {
		return nil, fmt.Errorf("no pools found for protocol=%s slugs=%v", protocol, slugs)
	}

	// Deduplicate by symbol — keep highest APY per token
	best := map[string]llamaPool{}
	for _, p := range pools {
		if existing, ok := best[p.Symbol]; !ok || p.APY > existing.APY {
			best[p.Symbol] = p
		}
	}

	now := time.Now().Unix()
	protocolMeta := defiLlamaProtocolMeta(protocol)
	var opps []Opportunity
	proto := protocol
	for _, p := range best {
		opps = append(opps, Opportunity{
			Protocol:    proto,
			DisplayName: protocolMeta.displayName,
			URL:         protocolMeta.url,
			Token:       p.Symbol,
			AssetType:   AssetTypeFor(p.Symbol),
			SupplyAPY:   p.APY,
			BorrowAPY:   p.APY * 1.6, // estimate borrow = supply * 1.6
			TVLMillions: p.TVLUsd / 1_000_000,
			UtilRate:    estimateUtilRate(p.APYBase, p.APY),
			UpdatedAt:   now,
			IsLive:      true,
		})
	}
	return opps, nil
}

type protocolMeta struct {
	displayName string
	url         string
}

func defiLlamaProtocolMeta(protocol Protocol) protocolMeta {
	switch protocol {
	case ProtocolKamino:
		return protocolMeta{"Kamino Lend", "https://app.kamino.finance/lending"}
	case ProtocolMarginfi:
		return protocolMeta{"Marginfi", "https://app.marginfi.com/"}
	case ProtocolDrift:
		return protocolMeta{"Drift Lend", "https://app.drift.trade/earn"}
	}
	return protocolMeta{string(protocol), ""}
}

func estimateUtilRate(apyBase, apyTotal float64) float64 {
	if apyTotal <= 0 {
		return 0
	}
	// Higher base APY relative to total → higher util rate estimate
	base := apyBase
	if base <= 0 {
		base = apyTotal
	}
	util := base / 15.0 // rough: max util at ~15% base APY
	if util > 0.95 {
		util = 0.95
	}
	if util < 0.1 {
		util = 0.1
	}
	return util
}
