package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"stableguard-backend/ai"
	"stableguard-backend/config"
	"stableguard-backend/pyth"
	"stableguard-backend/risk"
)

// SimulationResult is returned by SimulateDepeg.
type SimulationResult struct {
	DepegPct    float64            `json:"depeg_pct"`
	Prices      map[string]float64 `json:"prices"`
	Score       risk.ScoreV2       `json:"score"`
	Decision    *ai.FinalDecision  `json:"decision"`
	OnChainSig  string             `json:"on_chain_sig"`
	ExplorerURL string             `json:"explorer_url"`
	Error       string             `json:"error,omitempty"`
}

// SimulateDepeg injects a fake USDT depeg scenario and runs the full
// AI + on-chain pipeline synchronously.
//
// depegPct is the USDT deviation to simulate (e.g. 2.0 = USDT at $0.9800).
// Returns a SimulationResult with on-chain tx sig and Solana Explorer link.
func (e *Engine) SimulateDepeg(ctx context.Context, depegPct float64) *SimulationResult {
	if depegPct < 0.1 {
		depegPct = 0.1
	}
	if depegPct > 10.0 {
		depegPct = 10.0
	}

	result := &SimulationResult{DepegPct: depegPct}

	usdtPrice := 1.0 - (depegPct / 100.0)
	now := time.Now()

	snap := &pyth.PriceSnapshot{
		USDC: pyth.PriceData{FeedID: pyth.FeedIDUSDC, Price: 1.0000, Confidence: 0.0001, PublishTime: now},
		USDT: pyth.PriceData{FeedID: pyth.FeedIDUSDT, Price: usdtPrice, Confidence: 0.0001, PublishTime: now},
		All: map[string]pyth.PriceData{
			"USDC":  {FeedID: pyth.FeedIDUSDC, Price: 1.0000, Confidence: 0.0001, PublishTime: now},
			"USDT":  {FeedID: pyth.FeedIDUSDT, Price: usdtPrice, Confidence: 0.0001, PublishTime: now},
			"DAI":   {Price: 1.0000, Confidence: 0.0001, PublishTime: now},
			"PYUSD": {Price: 1.0000, Confidence: 0.0001, PublishTime: now},
		},
		FetchedAt: now,
	}

	result.Prices = map[string]float64{
		"USDC":  1.0000,
		"USDT":  usdtPrice,
		"DAI":   1.0000,
		"PYUSD": 1.0000,
	}

	// Step 1: push to windowed scorer and compute v2 risk
	e.scorer.Push(snap)
	balances := e.fetchBalances(ctx)
	score := e.scorer.Compute(snap, balances, e.cfg.StrategyMode)
	result.Score = score

	// Step 2: run AI agents
	decision, err := e.agents.Run(ctx, snap, score, balances, e.cfg.StrategyMode)
	if err != nil {
		result.Error = fmt.Sprintf("AI agents failed: %v", err)
		return result
	}
	result.Decision = decision

	// Step 3: update engine in-memory state so SSE clients see it immediately
	e.mu.Lock()
	e.lastDecision = decision
	e.lastScore = score
	e.lastSnap = snap
	e.mu.Unlock()

	// Step 4: record decision on-chain (record_decision PDA)
	// Errors here are non-fatal for the demo — log and continue.
	sig, err := e.recordDecisionTrace(ctx, decision, score)
	if err != nil {
		fmt.Printf("[demo] record_decision skipped: %v\n", err)
	} else {
		result.OnChainSig = sig
		result.ExplorerURL = explorerTxURL(e.cfg.SolanaRPCURL, sig)
	}

	return result
}

// CrashSimResult is returned by SimulateCrash.
type CrashSimResult struct {
	Asset       string             `json:"asset"`
	CrashPct    float64            `json:"crash_pct"`
	PriceBefore float64            `json:"price_before"`
	PriceAfter  float64            `json:"price_after"`
	Prices      map[string]float64 `json:"prices"`
	Score       risk.ScoreV2       `json:"score"`
	Decision    *ai.FinalDecision  `json:"decision"`
	OnChainSig  string             `json:"on_chain_sig"`
	ExplorerURL string             `json:"explorer_url"`
	Error       string             `json:"error,omitempty"`
}

// SimulateCrash injects a progressive price crash for a volatile asset (BTC/ETH/SOL),
// runs the multi-agent AI pipeline, and records the decision on-chain.
//
// asset: "BTC", "ETH", or "SOL"
// crashPct: percentage drop to simulate (e.g. 15.0 = -15%)
func (e *Engine) SimulateCrash(ctx context.Context, asset string, crashPct float64) *CrashSimResult {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	if asset == "" {
		asset = "BTC"
	}
	if crashPct < 1.0 {
		crashPct = 1.0
	}
	if crashPct > 50.0 {
		crashPct = 50.0
	}

	result := &CrashSimResult{Asset: asset, CrashPct: crashPct}

	// ── Baseline prices (from last live snapshot or hardcoded fallback) ────
	btcBase, ethBase, solBase := 68000.0, 3500.0, 140.0
	if snap := e.LastSnap(); snap != nil {
		if pd, ok := snap.All["BTC"]; ok && pd.Price > 0 {
			btcBase = pd.Price
		}
		if pd, ok := snap.All["ETH"]; ok && pd.Price > 0 {
			ethBase = pd.Price
		}
		if pd, ok := snap.All["SOL"]; ok && pd.Price > 0 {
			solBase = pd.Price
		}
	}

	prices := map[string]float64{
		"BTC": btcBase, "ETH": ethBase, "SOL": solBase,
		"USDC": 1.0000, "USDT": 1.0000, "DAI": 1.0000, "PYUSD": 1.0000,
	}
	basePrice := prices[asset]
	result.PriceBefore = basePrice

	// ── Push 5 progressive crash steps into the scorer window ─────────────
	// This creates a believable downtrend so the window-level crash detection fires.
	now := time.Now()
	makeSnap := func(p map[string]float64) *pyth.PriceSnapshot {
		all := make(map[string]pyth.PriceData, len(p))
		for sym, price := range p {
			all[sym] = pyth.PriceData{Price: price, Confidence: price * 0.001, PublishTime: now}
		}
		return &pyth.PriceSnapshot{
			USDC:      all["USDC"],
			USDT:      all["USDT"],
			All:       all,
			FetchedAt: now,
		}
	}

	steps := 5
	for i := 0; i <= steps; i++ {
		pct := crashPct * float64(i) / float64(steps)
		stepPrices := make(map[string]float64, len(prices))
		for k, v := range prices {
			stepPrices[k] = v
		}
		stepPrices[asset] = basePrice * (1.0 - pct/100.0)
		e.scorer.Push(makeSnap(stepPrices))
	}

	// ── Final crash snapshot ───────────────────────────────────────────────
	crashedPrice := basePrice * (1.0 - crashPct/100.0)
	prices[asset] = crashedPrice
	result.PriceAfter = crashedPrice
	result.Prices = prices

	crashSnap := makeSnap(prices)

	// ── Run risk + AI ──────────────────────────────────────────────────────
	balances := e.fetchBalances(ctx)
	score := e.scorer.Compute(crashSnap, balances, e.cfg.StrategyMode)
	result.Score = score

	decision, err := e.agents.Run(ctx, crashSnap, score, balances, e.cfg.StrategyMode)
	if err != nil {
		result.Error = fmt.Sprintf("AI agents failed: %v", err)
		return result
	}
	result.Decision = decision

	// ── Update in-memory engine state ─────────────────────────────────────
	e.mu.Lock()
	e.lastDecision = decision
	e.lastScore = score
	e.lastSnap = crashSnap
	e.mu.Unlock()

	// ── Record decision on-chain ───────────────────────────────────────────
	// Errors here are non-fatal for the demo — log and continue.
	sig, err := e.recordDecisionTrace(ctx, decision, score)
	if err != nil {
		fmt.Printf("[demo] record_decision skipped: %v\n", err)
	} else {
		result.OnChainSig = sig
		result.ExplorerURL = explorerTxURL(e.cfg.SolanaRPCURL, sig)
	}

	return result
}

func explorerTxURL(rpcURL, sig string) string {
	if sig == "" {
		return ""
	}
	cluster := "?cluster=devnet"
	if config.IsMainnetRPC(rpcURL) {
		cluster = ""
	}
	return "https://explorer.solana.com/tx/" + sig + cluster
}
