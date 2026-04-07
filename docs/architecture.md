# Architecture — StableGuard

## Overview

StableGuard is a three-layer system: a real-time data layer (Pyth oracle), an AI decision layer (3 Claude agents), and an on-chain execution layer (Anchor program on Solana). All three are connected in a continuous autonomous loop.

---

## The Autonomous Loop

```
Every Pyth tick (~1s):
  Pyth SSE → WindowedScorer → risk score updated

Every 30 seconds (or on risk threshold breach):
  Risk score → Agent 1 (Risk Analyst)
             → Agent 2 (Yield Analyst)
             → Agent 3 (Strategy Agent)
             → FinalDecision { action, from_index, to_index, rationale, confidence }
             → record_decision  (Solana TX → DecisionLog PDA)
             → execute_rebalance (Solana TX → vault.balances mutated)

On depeg > 1.5%:
  → toggle_pause (Solana TX → vault.is_paused = true)
  → Telegram / Discord alert

Every tick (hot path, ~400ms):
  → update_price_and_check (Solana TX → vault.last_price updated)
```

---

## Layer 1 — Data (Pyth Network)

**File:** `backend/pyth/monitor.go`, `backend/pyth/feeds.go`

- Connects to Pyth Hermes SSE endpoint for real-time price updates
- Falls back to polling every 5s if SSE drops
- Tracks 7 assets simultaneously:

| Symbol | Type | Vault Slot | Pyth Feed ID |
|---|---|---|---|
| USDC | stable | 0 | `0xeaa020c6...` |
| USDT | stable | 1 | `0x2b89b9dc...` |
| ETH | volatile | 2 | `0xff61491a...` |
| SOL | volatile | 3 | `0xef0d8b6f...` |
| BTC | volatile | 4 | `0xe62df6c8...` |
| DAI | stable | 5 | `0xb0948a5e...` |
| PYUSD | stable | 6 | `0xc1da1b73...` |

Each `PriceSnapshot` contains all 7 prices + confidence intervals, published to the pipeline channel.

---

## Layer 2 — Risk Engine (Go)

**File:** `backend/risk/scorer_v2.go`

`WindowedScorer` maintains a sliding window of 20 price snapshots and computes:

```
RiskLevel = deviation  × 0.35   ← stablecoin peg drift
          + velocity   × 0.15   ← speed of change (1-tick delta)
          + trend      × 0.10   ← linear slope over window
          + volatility × 0.10   ← std-dev of deviations
          + crash      × 0.30   ← volatile asset drawdown (BTC/ETH/SOL)
```

Output `ScoreV2`:
```go
type ScoreV2 struct {
    RiskLevel      float64  // 0–100 composite
    StableRisk     float64  // stablecoin peg component
    VolatileRisk   float64  // crypto crash component
    Deviation      float64  // current peg deviation %
    Trend          float64  // direction of drift
    Velocity       float64  // rate of change
    Volatility     float64  // price noise level
    VolatilePrices map[string]float64  // BTC/ETH/SOL live prices
}
```

---

## Layer 3 — AI Agents (Claude Haiku)

**File:** `backend/ai/agents.go`

Three agents run sequentially, each receiving the full market context:

### Agent 1 — Risk Analyst
**Input:** stablecoin prices, deviation %, risk score, BTC/ETH/SOL prices, vault balances  
**Task:** Determine if the risk signal is real or noise. Assess severity.  
**Output:** `AgentResult { Summary, Action, Confidence }`

### Agent 2 — Yield Analyst
**Input:** same context + Agent 1 output  
**Task:** Find rebalancing opportunities. Is there a spread to capture?  
**Output:** `AgentResult { Summary, Action, Confidence }`

### Agent 3 — Strategy Agent
**Input:** both previous analyses + strategy mode (SAFE/BALANCED/YIELD)  
**Task:** Synthesize → final decision  
**Output:** `FinalDecision { action, from_index, to_index, suggested_fraction, rationale, confidence }`

Actions: `HOLD` | `PROTECT` (risk-driven) | `OPTIMIZE` (yield-driven)

Decision profiles:
- `cautious` — only acts on high-confidence signals
- `balanced` — default, moderate threshold
- `aggressive` — acts on smaller signals, maximizes yield

**Dry-run mode:** if `ANTHROPIC_API_KEY` is empty, agents return mock decisions — system stays fully functional for testing.

---

## Layer 4 — On-Chain Execution (Anchor / Solana)

**Files:** `backend/solana/executor.go`, `programs/stableguard/src/`

### VaultState PDA
```
seeds: ["vault", authority_pubkey]

Fields:
  authority          Pubkey      — vault owner
  balances           [u64; 8]    — per-slot accounting (mutated by AI)
  total_deposited    u64
  decision_count     u64         — incremented on every record_decision
  total_rebalances   u64         — incremented on every execute_rebalance
  is_paused          bool        — circuit breaker flag
  strategy_mode      u8          — 0=SAFE, 1=YIELD
  delegated_agent    Pubkey      — AI agent keypair (autonomous signing)
  last_price         u64         — hot path price (updated every tick)
  circuit_breaker_threshold u64
```

### DecisionLog PDA
```
seeds: ["decision", vault_pubkey, decision_count]

Fields:
  vault             Pubkey
  sequence          u64
  action            u8      — 0=HOLD, 1=OPTIMIZE, 2=PROTECT
  rationale         String  — Claude's reasoning (on-chain, immutable)
  confidence_score  u8      — 0–100
  timestamp         i64
```

Every AI decision is **permanently stored on Solana** — publicly verifiable by anyone.

### Delegate Agent Pattern
```
vault.delegated_agent = AI_AGENT_PUBKEY
```
The AI agent signs `execute_rebalance` and `record_decision` with its own keypair. The treasury owner never hands over their private key. True autonomous operation.

---

## Policy Layer

**File:** `backend/pipeline/engine.go`

Between AI decision and on-chain execution sits a policy layer that checks:

| Control Mode | Behavior |
|---|---|
| `MANUAL` | AI monitors only, never executes |
| `GUARDED` | Executes only on extreme risk (>80/100) |
| `BALANCED` | Moderate automation with guardrails |
| `YIELD_MAX` | Maximum autonomy with circuit breakers |

---

## Real-Time Frontend

**File:** `frontend/`

- SSE stream from `GET /api/v1/stream` → dashboard updates every tick
- Falls back to polling every 30s after 3 SSE failures
- Toast alerts fire when risk crosses 60 and 80 thresholds

---

## Database

**File:** `backend/store/store.go`

SQLite (`stableguard.db`) stores:
- `price_snapshots` — full price history per symbol
- `ai_decisions` — all AI decisions with rationale
- `rebalance_history` — executed rebalances
- `risk_events` — threshold breach events
- `yield_positions` — yield protocol positions

---

## Program ID

```
GPSJJqicuDSJ6LXhEZpmUboThjzdefG5wZAZkL2hd7es  (Solana Devnet)
```
