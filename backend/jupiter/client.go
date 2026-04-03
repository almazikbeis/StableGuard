// Package jupiter provides a thin wrapper around the Jupiter v6 Swap API.
// Quote endpoint is public and works without authentication.
// Note: swaps require mainnet; this client is used for quote display only.
package jupiter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const baseURL = "https://quote-api.jup.ag/v6"

var httpClient = &http.Client{Timeout: 8 * time.Second}

// QuoteRequest describes a swap quote request.
type QuoteRequest struct {
	InputMint   string // SPL token mint address
	OutputMint  string
	Amount      uint64 // in base units (lamports for SOL, micro-USDC for USDC, etc.)
	SlippageBps int    // e.g. 50 = 0.5%
}

// RoutePlan is one hop in the swap route.
type RoutePlan struct {
	SwapInfo struct {
		AmmKey  string `json:"ammKey"`
		Label   string `json:"label"`
		FeeAmt  string `json:"feeAmount"`
		FeeMint string `json:"feeMint"`
		InAmt   string `json:"inAmount"`
		OutAmt  string `json:"outAmount"`
	} `json:"swapInfo"`
	Percent int `json:"percent"`
}

// QuoteResponse is the response from GET /quote.
type QuoteResponse struct {
	InputMint            string      `json:"inputMint"`
	InAmount             string      `json:"inAmount"`
	OutputMint           string      `json:"outputMint"`
	OutAmount            string      `json:"outAmount"`
	OtherAmountThreshold string      `json:"otherAmountThreshold"`
	SwapMode             string      `json:"swapMode"`
	SlippageBps          int         `json:"slippageBps"`
	PriceImpactPct       string      `json:"priceImpactPct"`
	RoutePlan            []RoutePlan `json:"routePlan"`
	ContextSlot          int64       `json:"contextSlot"`
}

// GetQuote fetches a swap quote from Jupiter v6.
// Returns an error if the API is unreachable (e.g., on devnet / CI environments).
func GetQuote(req QuoteRequest) (*QuoteResponse, error) {
	params := url.Values{}
	params.Set("inputMint", req.InputMint)
	params.Set("outputMint", req.OutputMint)
	params.Set("amount", fmt.Sprintf("%d", req.Amount))
	slippage := req.SlippageBps
	if slippage == 0 {
		slippage = 50
	}
	params.Set("slippageBps", fmt.Sprintf("%d", slippage))

	endpoint := baseURL + "/quote?" + params.Encode()
	resp, err := httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("jupiter quote request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("jupiter read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jupiter API %d: %s", resp.StatusCode, string(body))
	}

	var quote QuoteResponse
	if err := json.Unmarshal(body, &quote); err != nil {
		return nil, fmt.Errorf("jupiter unmarshal: %w", err)
	}
	return &quote, nil
}
