// Package jupiter provides a full Jupiter v6 Swap API client.
// Quote + swap work on mainnet. On devnet/localnet Jupiter returns "no route"
// which is handled gracefully — callers fall back to accounting-only mode.
package jupiter

import (
	"bytes"
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

type SwapRequest struct {
	QuoteResponse           QuoteResponse `json:"quoteResponse"`
	UserPublicKey           string        `json:"userPublicKey"`
	WrapAndUnwrapSOL        bool          `json:"wrapAndUnwrapSol"`
	UseSharedAccounts       bool          `json:"useSharedAccounts,omitempty"`
	DynamicComputeUnitLimit bool          `json:"dynamicComputeUnitLimit,omitempty"`
	AsLegacyTransaction     bool          `json:"asLegacyTransaction,omitempty"`
	SourceTokenAccount      string        `json:"sourceTokenAccount,omitempty"`
	DestinationTokenAccount string        `json:"destinationTokenAccount,omitempty"`
	PrioritizationFee       any           `json:"prioritizationFeeLamports,omitempty"`
}

type SwapResponse struct {
	SwapTransaction           string `json:"swapTransaction"`
	LastValidBlockHeight      uint64 `json:"lastValidBlockHeight"`
	PrioritizationFeeLamports int64  `json:"prioritizationFeeLamports"`
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

// PrepareSwapTx fetches a Jupiter quote and builds the signed-ready swap transaction.
// Returns the SwapResponse (contains base64 TX), the matched quote, and any error.
// Set asLegacyTx=true to get a legacy transaction (simpler signing, avoids ALT complexity).
func PrepareSwapTx(req QuoteRequest, userPubkey string, asLegacyTx bool) (*SwapResponse, *QuoteResponse, error) {
	quote, err := GetQuote(req)
	if err != nil {
		return nil, nil, fmt.Errorf("jupiter quote: %w", err)
	}

	swapReq := SwapRequest{
		QuoteResponse:           *quote,
		UserPublicKey:           userPubkey,
		WrapAndUnwrapSOL:        true,
		DynamicComputeUnitLimit: true,
		AsLegacyTransaction:     asLegacyTx,
	}

	swapResp, err := BuildSwapTransaction(swapReq)
	if err != nil {
		return nil, quote, fmt.Errorf("jupiter build swap tx: %w", err)
	}

	return swapResp, quote, nil
}

// SwapExecutionResult holds the result of a successfully executed Jupiter swap.
type SwapExecutionResult struct {
	Signature   string         `json:"signature"`
	Quote       *QuoteResponse `json:"quote"`
	InAmount    string         `json:"in_amount"`
	OutAmount   string         `json:"out_amount"`
	PriceImpact string         `json:"price_impact_pct"`
	Routes      []string       `json:"routes"`
}

// RouteSummary extracts route labels from a QuoteResponse.
func RouteSummary(q *QuoteResponse) []string {
	if q == nil {
		return nil
	}
	labels := make([]string, 0, len(q.RoutePlan))
	for _, r := range q.RoutePlan {
		if r.SwapInfo.Label != "" {
			labels = append(labels, r.SwapInfo.Label)
		}
	}
	return labels
}

func BuildSwapTransaction(req SwapRequest) (*SwapResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("jupiter swap marshal: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, baseURL+"/swap", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("jupiter swap request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("jupiter swap request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("jupiter swap read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jupiter swap API %d: %s", resp.StatusCode, string(respBody))
	}

	var out SwapResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("jupiter swap unmarshal: %w", err)
	}
	if out.SwapTransaction == "" {
		return nil, fmt.Errorf("jupiter swap API returned empty swapTransaction")
	}
	return &out, nil
}
