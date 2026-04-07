package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"stableguard-backend/config"
	"stableguard-backend/jupiter"
	"stableguard-backend/pyth"
	"stableguard-backend/store"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// coingeckoSymbol maps token symbols to CoinGecko IDs for price conversion.
var coingeckoSymbol = map[string]string{
	"USDC":  "usd-coin",
	"USDT":  "tether",
	"ETH":   "ethereum",
	"SOL":   "solana",
	"BTC":   "bitcoin",
	"DAI":   "dai",
	"PYUSD": "paypal-usd",
}

// fetchCoingeckoPrices fetches USD prices for the given symbols via CoinGecko.
func fetchCoingeckoPrices(ctx context.Context, symbols ...string) (map[string]float64, error) {
	ids := make([]string, 0, len(symbols))
	symToID := map[string]string{}
	for _, sym := range symbols {
		if id, ok := coingeckoSymbol[sym]; ok {
			ids = append(ids, id)
			symToID[id] = sym
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no known CoinGecko IDs for symbols: %v", symbols)
	}

	url := "https://api.coingecko.com/api/v3/simple/price?ids=" + strings.Join(ids, ",") + "&vs_currencies=usd"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	prices := map[string]float64{}
	for id, data := range raw {
		if sym, ok := symToID[id]; ok {
			prices[sym] = data["usd"]
		}
	}
	return prices, nil
}

type Executor interface {
	WalletAddress() solana.PublicKey
	TokenAccountBalance(ctx context.Context, account string) (uint64, error)
	SimulateExternalTransaction(ctx context.Context, txBase64 string) (*rpc.SimulateTransactionResult, error)
	SendExternalTransaction(ctx context.Context, txBase64 string) (string, error)
	WaitForSignatureConfirmation(ctx context.Context, sig string, timeout time.Duration, required rpc.ConfirmationStatusType) (*rpc.SignatureStatusesResult, error)
	MintTokensTo(ctx context.Context, mintAddr, destATA string, amount uint64) (string, error)
}

type Service struct {
	executor Executor
	cfg      *config.Config
	store    *store.DB
}

type PreparedSwap struct {
	Quote         *jupiter.QuoteResponse
	SwapTx        *jupiter.SwapResponse
	SwapAmount    uint64
	TargetCustody string
}

type SubmissionResult struct {
	Job             *store.ExecutionJobRow
	SwapSig         string
	SimulationUnits uint64
	SourceDelta     uint64
	TargetDelta     uint64
}

func New(executor Executor, cfg *config.Config, db *store.DB) *Service {
	return &Service{executor: executor, cfg: cfg, store: db}
}

func (s *Service) CustodyAccount(token string) string {
	token = strings.ToUpper(strings.TrimSpace(token))
	if s.cfg == nil {
		return ""
	}
	switch token {
	case "USDC":
		return s.cfg.ExecutionCustodyUSDCAccount
	case "USDT":
		return s.cfg.ExecutionCustodyUSDTAccount
	case "ETH":
		return s.cfg.ExecutionCustodyETHAccount
	case "SOL":
		return s.cfg.ExecutionCustodySOLAccount
	case "BTC":
		return s.cfg.ExecutionCustodyBTCAccount
	case "DAI":
		return s.cfg.ExecutionCustodyDAIAccount
	case "PYUSD":
		return s.cfg.ExecutionCustodyPYUSDAccount
	default:
		return ""
	}
}

func (s *Service) validateExecutionPair(sourceSymbol, targetSymbol string) (pyth.TokenFeed, pyth.TokenFeed, string, string, error) {
	sourceSymbol = strings.ToUpper(strings.TrimSpace(sourceSymbol))
	targetSymbol = strings.ToUpper(strings.TrimSpace(targetSymbol))

	if sourceSymbol == "" || targetSymbol == "" {
		return pyth.TokenFeed{}, pyth.TokenFeed{}, "", "", fmt.Errorf("source and target symbols are required")
	}
	if sourceSymbol == targetSymbol {
		return pyth.TokenFeed{}, pyth.TokenFeed{}, "", "", fmt.Errorf("source and target symbols must differ")
	}

	sourceFeed, ok := tokenFeedBySymbol(sourceSymbol)
	if !ok {
		return pyth.TokenFeed{}, pyth.TokenFeed{}, "", "", fmt.Errorf("unknown source symbol %s", sourceSymbol)
	}
	targetFeed, ok := tokenFeedBySymbol(targetSymbol)
	if !ok {
		return pyth.TokenFeed{}, pyth.TokenFeed{}, "", "", fmt.Errorf("unknown target symbol %s", targetSymbol)
	}

	sourceCustody := s.CustodyAccount(sourceSymbol)
	if sourceCustody == "" {
		return pyth.TokenFeed{}, pyth.TokenFeed{}, "", "", fmt.Errorf("source execution custody account is not configured")
	}
	targetCustody := s.CustodyAccount(targetSymbol)
	if targetCustody == "" {
		return pyth.TokenFeed{}, pyth.TokenFeed{}, "", "", fmt.Errorf("target execution custody account is not configured")
	}

	return sourceFeed, targetFeed, sourceCustody, targetCustody, nil
}

func (s *Service) SlippageBps(requested int) int {
	if requested > 0 {
		return requested
	}
	if s.cfg != nil && s.cfg.ExecutionMaxSlippageBps > 0 {
		return s.cfg.ExecutionMaxSlippageBps
	}
	return 50
}

func (s *Service) ValidateQuote(quote *jupiter.QuoteResponse) error {
	if quote == nil {
		return fmt.Errorf("empty quote")
	}
	if s.cfg == nil {
		return nil
	}
	if s.cfg.ExecutionMaxRouteHops > 0 && len(quote.RoutePlan) > s.cfg.ExecutionMaxRouteHops {
		return fmt.Errorf("route has %d hops, exceeding configured maximum of %d", len(quote.RoutePlan), s.cfg.ExecutionMaxRouteHops)
	}
	if s.cfg.ExecutionMaxPriceImpactPct > 0 {
		impact, err := strconv.ParseFloat(quote.PriceImpactPct, 64)
		if err == nil && impact > s.cfg.ExecutionMaxPriceImpactPct {
			return fmt.Errorf("price impact %.4f%% exceeds configured maximum of %.4f%%", impact, s.cfg.ExecutionMaxPriceImpactPct)
		}
	}
	return nil
}

func (s *Service) PrepareSwap(ctx context.Context, job *store.ExecutionJobRow, slippageBps int, requestedAmount uint64) (*PreparedSwap, error) {
	if s.cfg == nil {
		return nil, fmt.Errorf("config not available")
	}
	if s.executor == nil {
		return nil, fmt.Errorf("executor not available")
	}
	if job == nil {
		return nil, fmt.Errorf("execution job is required")
	}

	readiness := s.cfg.ExecutionReadiness()
	if !readiness.ReadyForAutoSwap {
		return nil, fmt.Errorf("automatic Jupiter swap execution is unavailable: %s", readiness.Note)
	}
	if job.Stage != "custody_staged" {
		return nil, fmt.Errorf("execution job stage %s cannot build a Jupiter swap", job.Stage)
	}

	sourceFeed, targetFeed, sourceCustody, targetCustody, err := s.validateExecutionPair(job.SourceSymbol, job.TargetSymbol)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(job.CustodyAccount) == "" {
		return nil, fmt.Errorf("source execution custody account is missing from execution job")
	}
	if job.CustodyAccount != sourceCustody {
		return nil, fmt.Errorf("execution job custody account %s does not match configured source custody for %s", job.CustodyAccount, job.SourceSymbol)
	}

	available, err := s.executor.TokenAccountBalance(ctx, job.CustodyAccount)
	if err != nil {
		return nil, fmt.Errorf("source custody balance: %w", err)
	}

	swapAmount := requestedAmount
	if swapAmount == 0 {
		swapAmount = available
	}
	if swapAmount == 0 {
		return nil, fmt.Errorf("source execution custody has no staged balance")
	}
	if swapAmount > available {
		return nil, fmt.Errorf("requested amount exceeds staged custody balance")
	}

	quote, err := jupiter.GetQuote(jupiter.QuoteRequest{
		InputMint:   sourceFeed.MainnetMint,
		OutputMint:  targetFeed.MainnetMint,
		Amount:      swapAmount,
		SlippageBps: s.SlippageBps(slippageBps),
	})
	if err != nil {
		return nil, err
	}
	if err := s.ValidateQuote(quote); err != nil {
		return nil, err
	}

	swapTx, err := jupiter.BuildSwapTransaction(jupiter.SwapRequest{
		QuoteResponse:           *quote,
		UserPublicKey:           s.executor.WalletAddress().String(),
		WrapAndUnwrapSOL:        false,
		UseSharedAccounts:       true,
		DynamicComputeUnitLimit: true,
		AsLegacyTransaction:     false,
		SourceTokenAccount:      job.CustodyAccount,
		DestinationTokenAccount: targetCustody,
		PrioritizationFee:       "auto",
	})
	if err != nil {
		return nil, err
	}

	return &PreparedSwap{
		Quote:         quote,
		SwapTx:        swapTx,
		SwapAmount:    swapAmount,
		TargetCustody: targetCustody,
	}, nil
}

func (s *Service) SubmitAndReconcile(ctx context.Context, job *store.ExecutionJobRow, targetCustody, swapTxBase64, minOutAmount string) (*SubmissionResult, error) {
	if s.store == nil {
		return nil, fmt.Errorf("execution job store is unavailable")
	}
	if s.executor == nil {
		return nil, fmt.Errorf("executor not available")
	}
	if job == nil {
		return nil, fmt.Errorf("execution job is required")
	}
	if targetCustody == "" {
		return nil, fmt.Errorf("target execution custody account is not configured")
	}

	sourceBalanceBefore, err := s.executor.TokenAccountBalance(ctx, job.CustodyAccount)
	if err != nil {
		return nil, fmt.Errorf("source custody balance: %w", err)
	}
	targetBalanceBefore, err := s.executor.TokenAccountBalance(ctx, targetCustody)
	if err != nil {
		return nil, fmt.Errorf("target custody balance: %w", err)
	}

	job.SourceBalanceBefore = sourceBalanceBefore
	job.TargetBalanceBefore = targetBalanceBefore
	job.TargetCustodyAccount = targetCustody

	simResult, err := s.executor.SimulateExternalTransaction(ctx, swapTxBase64)
	if err != nil {
		return nil, fmt.Errorf("simulate swap: %w", err)
	}
	if simResult.Err != nil {
		return nil, fmt.Errorf("swap simulation returned an error: %v", simResult.Err)
	}

	swapSig, err := s.executor.SendExternalTransaction(ctx, swapTxBase64)
	if err != nil {
		return nil, fmt.Errorf("submit swap: %w", err)
	}

	job.Stage = "swap_submitted"
	job.SwapSig = swapSig
	job.Note = fmt.Sprintf("Swap submitted for %s -> %s. Awaiting confirmation and custody reconciliation.", job.SourceSymbol, job.TargetSymbol)
	if simResult.UnitsConsumed != nil {
		job.SimulationUnits = *simResult.UnitsConsumed
	}
	if err := s.store.UpdateExecutionJob(*job); err != nil {
		return nil, fmt.Errorf("update execution job after submission: %w", err)
	}

	if _, err := s.executor.WaitForSignatureConfirmation(ctx, swapSig, 45*time.Second, rpc.ConfirmationStatusConfirmed); err != nil {
		job.Note = fmt.Sprintf("Swap submitted but confirmation is still pending or uncertain: %v", err)
		_ = s.store.UpdateExecutionJob(*job)
		return nil, fmt.Errorf("swap confirmation did not complete yet: %w", err)
	}

	sourceBalanceAfter, err := s.executor.TokenAccountBalance(ctx, job.CustodyAccount)
	if err != nil {
		return nil, fmt.Errorf("source custody reconciliation failed: %w", err)
	}
	targetBalanceAfter, err := s.executor.TokenAccountBalance(ctx, targetCustody)
	if err != nil {
		return nil, fmt.Errorf("target custody reconciliation failed: %w", err)
	}

	sourceDelta := saturatingSubUint64(job.SourceBalanceBefore, sourceBalanceAfter)
	targetDelta := saturatingSubUint64(targetBalanceAfter, job.TargetBalanceBefore)
	minOutValue := parseUint64OrZero(minOutAmount)

	job.Stage = "swap_confirmed"
	job.SourceBalanceAfter = sourceBalanceAfter
	job.TargetBalanceAfter = targetBalanceAfter
	job.Note = fmt.Sprintf("Swap confirmed on-chain for %s -> %s. Reconciling custody balances.", job.SourceSymbol, job.TargetSymbol)
	if err := s.store.UpdateExecutionJob(*job); err != nil {
		return nil, fmt.Errorf("update execution job after confirmation: %w", err)
	}

	if sourceDelta == 0 || targetDelta == 0 {
		job.Note = fmt.Sprintf("Swap confirmed but custody reconciliation is incomplete: source delta=%d target delta=%d. Manual review required.", sourceDelta, targetDelta)
		_ = s.store.UpdateExecutionJob(*job)
		return nil, fmt.Errorf("custody balances did not move as expected")
	}
	if minOutValue > 0 && targetDelta < minOutValue {
		job.Note = fmt.Sprintf("Swap confirmed but target custody received only %d units, below minimum quoted output %d. Manual review required.", targetDelta, minOutValue)
		_ = s.store.UpdateExecutionJob(*job)
		return nil, fmt.Errorf("received amount fell below minimum quoted output")
	}

	job.Stage = "reconciled_in_custody"
	job.Note = fmt.Sprintf("Swap confirmed and custody reconciled: source delta=%d, target delta=%d.", sourceDelta, targetDelta)
	if err := s.store.UpdateExecutionJob(*job); err != nil {
		return nil, fmt.Errorf("update execution job after reconciliation: %w", err)
	}

	return &SubmissionResult{
		Job:             job,
		SwapSig:         swapSig,
		SimulationUnits: job.SimulationUnits,
		SourceDelta:     sourceDelta,
		TargetDelta:     targetDelta,
	}, nil
}

func (s *Service) MarkFailed(jobID int64, note string) error {
	if s.store != nil {
		if job, err := s.store.ExecutionJobByID(jobID); err == nil {
			job.Stage = "failed"
			job.Note = note
			if updateErr := s.store.UpdateExecutionJob(*job); updateErr != nil {
				return fmt.Errorf("update failed execution job %d: %w", jobID, updateErr)
			}
		}
	}
	return fmt.Errorf("%s", note)
}

func ReconciledAmount(job store.ExecutionJobRow) uint64 {
	return saturatingSubUint64(job.TargetBalanceAfter, job.TargetBalanceBefore)
}

func parseUint64OrZero(raw string) uint64 {
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func saturatingSubUint64(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return 0
}

func tokenFeedBySymbol(symbol string) (pyth.TokenFeed, bool) {
	for _, feed := range pyth.ActiveFeeds {
		if feed.Symbol == symbol {
			return feed, true
		}
	}
	return pyth.TokenFeed{}, false
}

// splDecimals returns the SPL token decimal places for a feed.
func splDecimals(f pyth.TokenFeed) int {
	switch f.Symbol {
	case "SOL":
		return 9
	default:
		return 6 // USDC, USDT, ETH (wormhole), BTC (wormhole), DAI, PYUSD all use 6
	}
}

// pow10 returns 10^exp as float64.
func pow10(exp float64) float64 {
	result := 1.0
	if exp >= 0 {
		for i := 0; i < int(exp); i++ {
			result *= 10
		}
	} else {
		for i := 0; i > int(exp); i-- {
			result /= 10
		}
	}
	return result
}

// DevnetMockSwapResult is returned by DevnetMockSwap.
type DevnetMockSwapResult struct {
	MintSig       string
	OutAmount     uint64
	TargetMint    string
	TargetCustody string
}

// DevnetMockSwap simulates a Jupiter swap on devnet by minting target tokens directly
// to the target custody account. Requires the executor wallet to be the mint authority.
// targetMint must be the SPL mint address for job.TargetSymbol on devnet.
// Used when Jupiter is unavailable (devnet / no internet).
func (s *Service) DevnetMockSwap(ctx context.Context, job *store.ExecutionJobRow, targetCustody, targetMint string) (*DevnetMockSwapResult, error) {
	if s.executor == nil {
		return nil, fmt.Errorf("executor not available")
	}
	if job == nil {
		return nil, fmt.Errorf("execution job is required")
	}
	_, _, sourceCustody, configuredTargetCustody, err := s.validateExecutionPair(job.SourceSymbol, job.TargetSymbol)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(job.CustodyAccount) == "" {
		return nil, fmt.Errorf("source execution custody account is missing from execution job")
	}
	if job.CustodyAccount != sourceCustody {
		return nil, fmt.Errorf("execution job custody account %s does not match configured source custody for %s", job.CustodyAccount, job.SourceSymbol)
	}
	if targetMint == "" {
		return nil, fmt.Errorf("target mint address is required for devnet mock swap")
	}
	if targetCustody == "" {
		return nil, fmt.Errorf("target custody account is required")
	}
	if targetCustody != configuredTargetCustody {
		return nil, fmt.Errorf("target custody account %s does not match configured custody for %s", targetCustody, job.TargetSymbol)
	}

	// Get staged balance
	staged, err := s.executor.TokenAccountBalance(ctx, job.CustodyAccount)
	if err != nil {
		return nil, fmt.Errorf("source custody balance: %w", err)
	}
	inAmount := staged
	if inAmount == 0 {
		inAmount = job.Amount
	}

	// Calculate real output amount using CoinGecko prices
	outAmount := inAmount
	priceCtx, priceCancel := context.WithTimeout(ctx, 5*time.Second)
	prices, priceErr := fetchCoingeckoPrices(priceCtx, job.SourceSymbol, job.TargetSymbol)
	priceCancel()
	if priceErr == nil {
		srcPrice := prices[job.SourceSymbol]
		dstPrice := prices[job.TargetSymbol]
		if srcPrice > 0 && dstPrice > 0 && srcPrice != dstPrice {
			// Convert: outAmount = inAmount * (srcPrice / dstPrice)
			// Adjust for decimal difference between source and target tokens
			srcFeed, srcOk := tokenFeedBySymbol(job.SourceSymbol)
			dstFeed, dstOk := tokenFeedBySymbol(job.TargetSymbol)
			if srcOk && dstOk {
				srcDecimals := float64(splDecimals(srcFeed))
				dstDecimals := float64(splDecimals(dstFeed))
				// inAmount is in source base units → convert to target base units
				outFloat := float64(inAmount) * (srcPrice / dstPrice) * (1e0) // same base if same decimals
				if dstDecimals != srcDecimals {
					outFloat = outFloat * pow10(dstDecimals-srcDecimals)
				}
				// Apply 0.3% fee to simulate swap cost
				outAmount = uint64(outFloat * 0.997)
			}
		}
	}

	sig, err := s.executor.MintTokensTo(ctx, targetMint, targetCustody, outAmount)
	if err != nil {
		return nil, fmt.Errorf("devnet mint swap: %w", err)
	}

	priceNote := ""
	if priceErr == nil {
		sp := prices[job.SourceSymbol]
		dp := prices[job.TargetSymbol]
		if sp > 0 && dp > 0 {
			priceNote = fmt.Sprintf(" (CoinGecko: 1 %s = $%.4f, 1 %s = $%.4f, rate=%.6f)",
				job.SourceSymbol, sp, job.TargetSymbol, dp, sp/dp)
		}
	}

	job.QuoteOutAmount = strconv.FormatUint(outAmount, 10)
	job.MinOutAmount = strconv.FormatUint(outAmount, 10)
	job.TargetCustodyAccount = targetCustody
	job.SwapSig = sig
	job.Stage = "reconciled_in_custody"
	job.Note = fmt.Sprintf("Devnet swap: %d %s → %d %s (real CoinGecko pricing, -0.3%% fee)%s. Mint sig: %s",
		inAmount, job.SourceSymbol, outAmount, job.TargetSymbol, priceNote, sig)
	if s.store != nil {
		_ = s.store.UpdateExecutionJob(*job)
	}

	return &DevnetMockSwapResult{
		MintSig:       sig,
		OutAmount:     outAmount,
		TargetMint:    targetMint,
		TargetCustody: targetCustody,
	}, nil
}
