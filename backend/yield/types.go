// Package yield fetches lending APY rates from DeFi protocols on Solana.
package yield

import "context"

// Protocol identifies a DeFi lending protocol.
type Protocol string

const (
	ProtocolKamino   Protocol = "kamino"
	ProtocolMarginfi Protocol = "marginfi"
	ProtocolDrift    Protocol = "drift"
)

// Opportunity represents a yield opportunity at a specific protocol.
type Opportunity struct {
	Protocol    Protocol `json:"protocol"`
	DisplayName string   `json:"display_name"`
	URL         string   `json:"url"`
	Token       string   `json:"token"`
	AssetType   string   `json:"asset_type"`   // "stable" | "volatile"
	SupplyAPY   float64  `json:"supply_apy"`   // annual % (e.g. 8.2 = 8.2%)
	BorrowAPY   float64  `json:"borrow_apy"`
	TVLMillions float64  `json:"tvl_millions"` // USD millions
	UtilRate    float64  `json:"util_rate"`    // 0-1
	UpdatedAt   int64    `json:"updated_at"`
	IsLive      bool     `json:"is_live"` // false = using fallback estimate
}

// AssetTypeFor returns "stable" for pegged assets, "volatile" otherwise.
func AssetTypeFor(symbol string) string {
	switch symbol {
	case "USDC", "USDT", "DAI", "PYUSD", "USDH", "USDR":
		return "stable"
	}
	return "volatile"
}

// Adapter is a yield protocol data source.
type Adapter interface {
	FetchOpportunities(ctx context.Context) ([]Opportunity, error)
}

// ByAPY sorts Opportunities descending by SupplyAPY.
type ByAPY []Opportunity

func (a ByAPY) Len() int           { return len(a) }
func (a ByAPY) Less(i, j int) bool { return a[i].SupplyAPY > a[j].SupplyAPY }
func (a ByAPY) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
