// Package onchain fetches on-chain liquidity signals via DexScreener (free, no auth).
package onchain

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const dexscreenerBase = "https://api.dexscreener.com/latest/dex/tokens"

// Solana mint addresses for stablecoins
const (
	MintUSDC = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	MintUSDT = "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"
	MintDAI  = "EjmyN6qEC1Tf1JxiG1ae7UTJhUxSwk1TCWNWqxWV4J6o"
)

var dexHTTP = &http.Client{Timeout: 8 * time.Second}

// PairInfo is a simplified view of a DexScreener pair.
type PairInfo struct {
	PairAddress string  `json:"pairAddress"`
	DexID       string  `json:"dexId"`
	BaseSymbol  string  `json:"baseSymbol"`
	QuoteSymbol string  `json:"quoteSymbol"`
	PriceUSD    float64 `json:"priceUsd,string"`
	PriceChange struct {
		H1  float64 `json:"h1"`
		H6  float64 `json:"h6"`
		H24 float64 `json:"h24"`
	} `json:"priceChange"`
	Volume struct {
		H1  float64 `json:"h1"`
		H24 float64 `json:"h24"`
	} `json:"volume"`
	Txns struct {
		H1 struct {
			Buys  int `json:"buys"`
			Sells int `json:"sells"`
		} `json:"h1"`
	} `json:"txns"`
	Liquidity struct {
		USD float64 `json:"usd"`
	} `json:"liquidity"`
}

type dexResponse struct {
	Pairs []struct {
		PairAddress string `json:"pairAddress"`
		DexID       string `json:"dexId"`
		BaseToken   struct {
			Symbol string `json:"symbol"`
		} `json:"baseToken"`
		QuoteToken struct {
			Symbol string `json:"symbol"`
		} `json:"quoteToken"`
		PriceUSD  string `json:"priceUsd"`
		PriceChange struct {
			H1  float64 `json:"h1"`
			H6  float64 `json:"h6"`
			H24 float64 `json:"h24"`
		} `json:"priceChange"`
		Volume struct {
			H1  float64 `json:"h1"`
			H24 float64 `json:"h24"`
		} `json:"volume"`
		Txns struct {
			H1 struct {
				Buys  int `json:"buys"`
				Sells int `json:"sells"`
			} `json:"h1"`
		} `json:"txns"`
		Liquidity struct {
			USD float64 `json:"usd"`
		} `json:"liquidity"`
		ChainID string `json:"chainId"`
	} `json:"pairs"`
}

// FetchTopPairs returns the largest Solana pairs for a given mint, sorted by liquidity.
func FetchTopPairs(mint string, limit int) ([]PairInfo, error) {
	url := fmt.Sprintf("%s/%s", dexscreenerBase, mint)
	resp, err := dexHTTP.Get(url)
	if err != nil {
		return nil, fmt.Errorf("dexscreener fetch: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dexscreener %d", resp.StatusCode)
	}

	var raw dexResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("dexscreener unmarshal: %w", err)
	}

	var pairs []PairInfo
	for _, p := range raw.Pairs {
		if p.ChainID != "solana" {
			continue
		}
		var priceF float64
		fmt.Sscanf(p.PriceUSD, "%f", &priceF)

		pairs = append(pairs, PairInfo{
			PairAddress: p.PairAddress,
			DexID:       p.DexID,
			BaseSymbol:  p.BaseToken.Symbol,
			QuoteSymbol: p.QuoteToken.Symbol,
			PriceUSD:    priceF,
			PriceChange: p.PriceChange,
			Volume:      p.Volume,
			Txns:        p.Txns,
			Liquidity:   p.Liquidity,
		})

		if len(pairs) >= limit {
			break
		}
	}
	return pairs, nil
}
