// feeds.go — registry of all stablecoins monitored by StableGuard.
//
// ╔══════════════════════════════════════════════════════════════════╗
// ║  TO ADD A NEW TOKEN — edit ActiveFeeds below.                  ║
// ║  1. Add a TokenFeed entry (Symbol, Name, FeedID, VaultSlot)    ║
// ║  2. Add its SPL mint to .env  (e.g. MINT_USDC=<pubkey>)        ║
// ║  3. On-chain: call register_token(slot, mint) via API          ║
// ╚══════════════════════════════════════════════════════════════════╝
//
// Pyth feed IDs: https://pyth.network/developers/price-feed-ids
package pyth

// TokenFeed describes one stablecoin monitored via Pyth Hermes.
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
}

// ActiveFeeds is the list of stablecoins StableGuard monitors.
//
// ┌─────────────────────────────────────────────────────────────────┐
// │  ADD YOUR TOKEN HERE — one line per stablecoin.                │
// └─────────────────────────────────────────────────────────────────┘
var ActiveFeeds = []TokenFeed{
	{
		Symbol:      "USDC",
		Name:        "USD Coin",
		FeedID:      "0xeaa020c61cc479712813461ce153894a96a6c00b21ed0cfc2798d1f9a9e9c94a",
		VaultSlot:   0,
		MainnetMint: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
	},
	{
		Symbol:      "USDT",
		Name:        "Tether USD",
		FeedID:      "0x2b89b9dc8fdf9f34709a5b106b472f0f39bb6ca9ce04b0fd7f2e971688e2e53b",
		VaultSlot:   1,
		MainnetMint: "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
	},
	{
		Symbol:      "DAI",
		Name:        "Dai Stablecoin",
		FeedID:      "0xb0948a5e5313200c632b51bb5ca32f6de0d36e9950a942d19751e833f70dabfd",
		VaultSlot:   2,
		MainnetMint: "EjmyN6qEC1Tf1JxiG1ae7UTJhUxSwk1TCWNWqxWV4J6o",
	},
	{
		Symbol:      "PYUSD",
		Name:        "PayPal USD",
		FeedID:      "0xc1da1b73d7f01e7ddd54b3766cf7fcd644395ad14f70aa706ec5384c59e76692",
		VaultSlot:   3,
		MainnetMint: "2b1kV6DkPAnxd5ixfnxCpjxmKwqjjaYmCZfHsFu24GXo",
	},

	// ── Examples of how to add more ───────────────────────────────────
	//
	// FDUSD (First Digital USD):
	// {
	// 	Symbol:    "FDUSD",
	// 	Name:      "First Digital USD",
	// 	FeedID:    "0x<feed_id_from_pyth.network>",
	// 	VaultSlot: 4,
	// 	MainnetMint: "<solana_mint_address>",
	// },
	//
	// USDC.e (Bridged USDC via Wormhole):
	// {
	// 	Symbol:    "USDCe",
	// 	Name:      "Bridged USDC (Wormhole)",
	// 	FeedID:    "0xeaa020c61cc479712813461ce153894a96a6c00b21ed0cfc2798d1f9a9e9c94a",
	// 	VaultSlot: 5,
	// 	MainnetMint: "A9mUU4qviSctJVPJdBJWkb28deg915LYJKrzQ2ZE1B",
	// },
	//
	// TUSD (TrueUSD):
	// {
	// 	Symbol:    "TUSD",
	// 	Name:      "TrueUSD",
	// 	FeedID:    "0x<feed_id_from_pyth.network>",
	// 	VaultSlot: 6,
	// 	MainnetMint: "<solana_mint_address>",
	// },
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
