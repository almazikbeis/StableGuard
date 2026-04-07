// feeds.go — registry of all tokens monitored by StableGuard.
//
// ╔══════════════════════════════════════════════════════════════════╗
// ║  TO ADD A NEW TOKEN — edit ActiveFeeds below.                  ║
// ║  1. Add a TokenFeed entry (Symbol, Name, FeedID, VaultSlot)    ║
// ║  2. Set AssetType to "stable" or "volatile"                    ║
// ║  3. Add its SPL mint to .env  (e.g. MINT_USDC=<pubkey>)        ║
// ║  4. On-chain: call register_token(slot, mint) via API          ║
// ╚══════════════════════════════════════════════════════════════════╝
//
// Pyth feed IDs: https://pyth.network/developers/price-feed-ids
package pyth

// TokenFeed describes one token monitored via Pyth Hermes.
type TokenFeed struct {
	// Symbol is the short ticker, e.g. "USDC".
	Symbol string
	// Name is the full name, e.g. "USD Coin".
	Name string
	// FeedID is the Pyth hex feed ID (with or without 0x prefix).
	FeedID string
	// VaultSlot is the index in the on-chain vault (0–7).
	// Use -1 if not registered in the vault.
	VaultSlot int
	// MainnetMint is the SPL token mint address on Solana mainnet.
	// Used for documentation; actual mint comes from env/config.
	MainnetMint string
	// AssetType is "stable" (pegged to $1) or "volatile" (BTC/ETH/SOL).
	AssetType string
}

// IsVolatile returns true if this is a volatile asset (BTC, ETH, SOL, etc.).
func (f TokenFeed) IsVolatile() bool { return f.AssetType == "volatile" }

// ActiveFeeds is the list of tokens StableGuard monitors.
// The slice order intentionally matches the on-chain vault slot order.
var ActiveFeeds = []TokenFeed{
	// ── Stablecoins ────────────────────────────────────────────────────
	{
		Symbol:      "USDC",
		Name:        "USD Coin",
		FeedID:      "0xeaa020c61cc479712813461ce153894a96a6c00b21ed0cfc2798d1f9a9e9c94a",
		VaultSlot:   0,
		MainnetMint: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		AssetType:   "stable",
	},
	{
		Symbol:      "USDT",
		Name:        "Tether USD",
		FeedID:      "0x2b89b9dc8fdf9f34709a5b106b472f0f39bb6ca9ce04b0fd7f2e971688e2e53b",
		VaultSlot:   1,
		MainnetMint: "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
		AssetType:   "stable",
	},

	// ── Volatile assets ───────────────────────────────────────────────
	// These slots are used by the mixed-asset treasury path and align
	// with the frontend asset catalog and on-chain registration flow.
	{
		Symbol:      "ETH",
		Name:        "Ethereum",
		FeedID:      "0xff61491a931112ddf1bd8147cd1b641375f79f5825126d665480874634fd0ace",
		VaultSlot:   2,
		MainnetMint: "7vfCXTUXx5WJV5JADk17DUJ4ksgau7utNKj4b963voxs", // Wrapped ETH (Wormhole)
		AssetType:   "volatile",
	},
	{
		Symbol:      "SOL",
		Name:        "Solana",
		FeedID:      "0xef0d8b6fda2ceba41da15d4095d1da392a0d2f8ed0c6c7bc0f4cfac8c280b56d",
		VaultSlot:   3,
		MainnetMint: "So11111111111111111111111111111111111111112",
		AssetType:   "volatile",
	},
	{
		Symbol:      "BTC",
		Name:        "Bitcoin",
		FeedID:      "0xe62df6c8b4a85fe1a67db44dc12de5db330f7ac66b72dc658afedf0f4a415b43",
		VaultSlot:   4,
		MainnetMint: "3NZ9JMVBmGAqocybic2c7LQCJScmgsAZ6vQqTDzcqmJh", // Wrapped BTC (Wormhole)
		AssetType:   "volatile",
	},

	// ── Additional stablecoins ────────────────────────────────────────
	{
		Symbol:      "DAI",
		Name:        "Dai Stablecoin",
		FeedID:      "0xb0948a5e5313200c632b51bb5ca32f6de0d36e9950a942d19751e833f70dabfd",
		VaultSlot:   5,
		MainnetMint: "EjmyN6qEC1Tf1JxiG1ae7UTJhUxSwk1TCWNWqxWV4J6o",
		AssetType:   "stable",
	},
	{
		Symbol:      "PYUSD",
		Name:        "PayPal USD",
		FeedID:      "0xc1da1b73d7f01e7ddd54b3766cf7fcd644395ad14f70aa706ec5384c59e76692",
		VaultSlot:   6,
		MainnetMint: "2b1kV6DkPAnxd5ixfnxCpjxmKwqjjaYmCZfHsFu24GXo",
		AssetType:   "stable",
	},
}

// MainnetMintBySlot returns the mainnet SPL mint address for a given vault slot index.
// Returns "" if the slot is not registered in ActiveFeeds.
func MainnetMintBySlot(slot int) string {
	for _, f := range ActiveFeeds {
		if f.VaultSlot == slot {
			return f.MainnetMint
		}
	}
	return ""
}

// FeedBySlot returns the TokenFeed registered for a specific vault slot.
func FeedBySlot(slot int) (TokenFeed, bool) {
	for _, f := range ActiveFeeds {
		if f.VaultSlot == slot {
			return f, true
		}
	}
	return TokenFeed{}, false
}

// StableFeeds returns only stablecoin feeds.
func StableFeeds() []TokenFeed {
	var out []TokenFeed
	for _, f := range ActiveFeeds {
		if f.AssetType == "stable" {
			out = append(out, f)
		}
	}
	return out
}

// VolatileFeeds returns only volatile asset feeds (BTC/ETH/SOL).
func VolatileFeeds() []TokenFeed {
	var out []TokenFeed
	for _, f := range ActiveFeeds {
		if f.AssetType == "volatile" {
			out = append(out, f)
		}
	}
	return out
}

// FeedIDBySymbol returns the Pyth feed ID for the given symbol, or "".
func FeedIDBySymbol(symbol string) string {
	for _, f := range ActiveFeeds {
		if f.Symbol == symbol {
			return f.FeedID
		}
	}
	return ""
}

// FeedByID returns the TokenFeed for the given Pyth hex ID (with or without 0x).
func FeedByID(id string) (TokenFeed, bool) {
	// normalise: strip 0x if present
	clean := id
	if len(clean) > 2 && clean[:2] == "0x" {
		clean = clean[2:]
	}
	for _, f := range ActiveFeeds {
		fid := f.FeedID
		if len(fid) > 2 && fid[:2] == "0x" {
			fid = fid[2:]
		}
		if fid == clean {
			return f, true
		}
	}
	return TokenFeed{}, false
}

// AllFeedIDs returns all active Pyth feed IDs (for bulk fetching).
func AllFeedIDs() []string {
	ids := make([]string, len(ActiveFeeds))
	for i, f := range ActiveFeeds {
		ids[i] = f.FeedID
	}
	return ids
}
