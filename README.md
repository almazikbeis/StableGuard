# StableGuard

![Build](https://img.shields.io/badge/build-passing-brightgreen)
![Solana](https://img.shields.io/badge/Solana-devnet-9945FF?logo=solana)
![Anchor](https://img.shields.io/badge/Anchor-0.29-blue)
![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go)
![Next.js](https://img.shields.io/badge/Next.js-16-black?logo=next.js)
![Claude AI](https://img.shields.io/badge/Claude-Haiku-orange)
![License](https://img.shields.io/badge/license-MIT-green)
![Hackathon](https://img.shields.io/badge/Decentrathon-Case%202%20AI%20%2B%20Blockchain-blueviolet)

> **Autonomous AI agent that protects on-chain treasuries in real time — no human required.**

---

## The Problem

DAOs hold **billions in stablecoins and crypto** with zero autonomous protection.

- **May 2022** — UST collapses. $40B gone in 72 hours. DAO governance took **3 days** to respond.
- **March 2023** — USDC de-pegs to $0.87 during SVB crisis. Treasuries had no circuit breakers.
- Smart contracts are **static** — they can't see market conditions, they can't think, they can't act.
- There is **no transparent way** to use AI in financial systems today.

## The Solution

StableGuard is an AI agent that **acts**, not just monitors.

- Watches 7 assets (USDC, USDT, DAI, PYUSD, ETH, SOL, BTC) via Pyth oracle every second
- Runs 3 specialized Claude AI agents every 30 seconds
- When risk is detected — submits a Solana transaction **autonomously**
- Every decision is **immutably stored on-chain** — fully auditable

```
UST collapse scenario with StableGuard:
  10:15 UTC — Risk score hits 52. AI: PROTECT → execute_rebalance TX on Solana ✓
  11:00 UTC — Depeg 3%. Circuit breaker trips → vault paused via toggle_pause TX ✓
  14:00 UTC — UST at $0.10. Vault already safe. Zero loss.

Manual governance: 3 days. StableGuard: seconds.
```

---

## Why Solana

Solana is not a formality here — it is load-bearing infrastructure:

| What | Why Solana specifically |
|---|---|
| **Sub-second price push** | `update_price_and_check` every ~400ms — impossible on slow L1s |
| **Immutable AI audit trail** | Each decision → PDA account on-chain. Costs ~0.002 SOL. On Ethereum that's $50+ |
| **Delegate agent signing** | AI holds its own keypair, signs `execute_rebalance` without touching user funds |
| **Anchor program — 17 instructions** | Full vault lifecycle: init, deposit, rebalance, pause, record decision, swap result |
| **Jupiter v6 DEX** | Solana-native swap routing for real token rebalancing |
| **Pyth Network** | Real-time oracle built on Solana — 400ms price updates via SSE |

---

## Demo

### Live autonomous loop — simulate a depeg event

Open the dashboard → **AI Agent** tab → **Live Demo** panel:

```
① Select scenario:  "2% USDT Depeg"  or  "SOL -30% Crash"
② Click:            [Run Demo]
③ Watch:
     Price injected  →  Risk computed (78/100)
     AI agents run   →  PROTECT decision
     Solana TX       →  on-chain state changes
     Explorer link   →  verify yourself
```

**Real transactions on devnet (verifiable now):**

| Instruction | TX |
|---|---|
| `set_demo_balances` | [`4SgxTf...`](https://explorer.solana.com/tx/4SgxTfPm1e8wSzq2GprX3zQq4PRhaeXUB2DmH2u6E1rWypcsGQJj4BKWUW38HcAFWtpYHqG1f6TB21XLwo3yBAXb?cluster=devnet) |
| `execute_rebalance` (ETH→USDC) | [`2jdNcL...`](https://explorer.solana.com/tx/2jdNcLueQnss6kLphJu2rZYGRogPAoaVKHRM2zmjE8vXbiqHix6pWzx728zu2sf7p9yPca2koMMgR3LimZLC3oje?cluster=devnet) |
| `record_decision` (PROTECT 85%) | [`4HVqec...`](https://explorer.solana.com/tx/4HVqecqc1EyxyNtiQGyEV5RKW3Rw2CSX5z7k1RTCJcPznceMSznwQnf4SEoFj836TpWWc7M4TaKQyuYpZfyf8HkL?cluster=devnet) |
| `record_swap_result` | [`51rK3g...`](https://explorer.solana.com/tx/51rK3ggtC52Ybf4pbRFRh28UPrCYattzMTLpmRnqfBwrC8ESkYWYH9PqgUaqWxc9GwdjVUrSDbJKBM4iEog3fG31?cluster=devnet) |

**Program on devnet:**
[`GPSJJqicuDSJ6LXhEZpmUboThjzdefG5wZAZkL2hd7es`](https://explorer.solana.com/address/GPSJJqicuDSJ6LXhEZpmUboThjzdefG5wZAZkL2hd7es?cluster=devnet)

### Dashboard overview

```
┌─────────────────────────────────────────────────┐
│  MarketTape: BTC $68,247 · ETH $2,088 · SOL $79 │  ← live Pyth prices
├───────────┬─────────────────────────────────────┤
│  Sidebar  │  Risk Gauge      1.6 / 100  🟢       │
│  Overview │  AI Decision     HOLD (conf 82%)      │
│  Risk     │  Total Decisions 2,760                │
│  Yield    │  Rebalances      11                   │
│  AI Agent │  Risk Events     89                   │
│  On-Chain │                                       │
├───────────┴─────────────────────────────────────┤
│  TokenPrices: USDC · USDT · DAI · ETH · SOL · BTC│
│  PriceChart  AllocationPie  DecisionFeed          │
└─────────────────────────────────────────────────┘
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  STEP 1 — Pyth Network (real-time oracle)                   │
│  SSE stream → USDC / USDT / DAI / PYUSD / ETH / SOL / BTC  │
└─────────────────────┬───────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────┐
│  STEP 2 — Risk Engine v2 (Go)                               │
│  WindowedScorer: deviation×0.35 + velocity×0.15             │
│                + trend×0.10 + volatility×0.10               │
│                + volatile_crash×0.30  =  RiskLevel 0–100    │
└─────────────────────┬───────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────┐
│  STEP 3 — 3 Claude Haiku AI Agents (sequential)             │
│  Agent 1 (Risk Analyst)    → is this depeg real or noise?   │
│  Agent 2 (Yield Analyst)   → is there a spread opportunity? │
│  Agent 3 (Strategy Agent)  → HOLD | PROTECT | OPTIMIZE      │
└─────────────────────┬───────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────┐
│  STEP 4 — ON-CHAIN: record_decision                         │
│  Creates DecisionLog PDA → decision_count += 1  ✓           │
└─────────────────────┬───────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────┐
│  STEP 5 — ON-CHAIN: execute_rebalance                       │
│  vault.balances[from] -= amount                             │
│  vault.balances[to]   += amount                             │
│  vault.total_rebalances += 1              ✓ state mutated   │
└─────────────────────┬───────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────┐
│  STEP 6 — Circuit Breaker                                   │
│  depeg > 1.5% → toggle_pause → vault.is_paused = true  ✓   │
│  depeg > 3.0% → Telegram / Discord critical alert           │
└─────────────────────────────────────────────────────────────┘
```

### Solana Program — 17 instructions

| Instruction | Mutates on-chain state | AI-triggered |
|---|---|---|
| `initialize_vault` | Creates VaultState PDA | — |
| `deposit` | `balances[i] += amount` | — |
| `withdraw` | `balances[i] -= amount` | — |
| `execute_rebalance` | `balances[from] -= x`, `balances[to] += x`, `total_rebalances++` | **Yes** |
| `record_decision` | Creates DecisionLog PDA, `decision_count++` | **Yes** |
| `delegate_agent` | `vault.delegated_agent = agent_pubkey` | — |
| `toggle_pause` | `vault.is_paused = true/false` | **Yes (circuit breaker)** |
| `update_price_and_check` | `vault.last_price = price` | **Yes (every tick)** |
| `set_demo_balances` | `vault.balances[i] = x` (devnet demo) | — |
| `record_swap_result` | Swap receipt PDA | **Yes** |
| `emergency_withdraw` | Drains all balances | — |
| + 6 more | register, strategy, threshold, payment… | — |

### Codebase

```
backend/
  ai/agents.go          ← 3 Claude Haiku agents (Risk, Yield, Strategy)
  pipeline/engine.go    ← main loop: Pyth → scorer → agents → on-chain
  risk/scorer_v2.go     ← WindowedScorer (20-tick sliding window)
  solana/executor.go    ← all Anchor instructions via go-solana
  pyth/monitor.go       ← Pyth Hermes SSE stream + polling fallback
  alerts/alerts.go      ← Telegram + Discord with cooldown dedup

programs/stableguard/src/instructions/
  rebalance.rs          ← mutates balances + emits RebalanceExecuted
  record_decision.rs    ← immutable AI decision PDA
  delegate_agent.rs     ← AI autonomous signing pattern

frontend/
  app/dashboard/        ← 5-tab live dashboard (SSE feed)
  components/
    LiveDemoFlow.tsx     ← depeg + crash simulator
    PipelineVisualizer.tsx
    RiskGauge.tsx
    TokenPrices.tsx
    WhaleIntelligence.tsx
```

---

## Quick Start

```bash
# 1. Backend
cd backend && cp .env.example .env
# set ANTHROPIC_API_KEY=sk-ant-... in .env
go run main.go

# 2. Frontend
cd frontend && npm install && npm run dev

# 3. Set up demo vault (devnet)
curl -X POST http://localhost:8080/api/v1/demo/set-balances \
  -H "Content-Type: application/json" -d '{}'
```

Open **http://localhost:3000/dashboard**

### Key env variables

```env
ANTHROPIC_API_KEY=sk-ant-...          # Claude AI (required)
SOLANA_RPC_URL=https://api.devnet.solana.com
AUTO_EXECUTE=true
AI_INTERVAL_SEC=30
AI_DECISION_PROFILE=balanced          # cautious | balanced | aggressive
CIRCUIT_BREAKER_ENABLED=true
CIRCUIT_BREAKER_PAUSE_PCT=1.5
TELEGRAM_BOT_TOKEN=...                # optional
DISCORD_WEBHOOK_URL=...               # optional
```

---

## Roadmap

- [x] Anchor program on devnet (17 instructions)
- [x] 3-agent Claude AI pipeline
- [x] Real-time Pyth price feed (7 assets)
- [x] On-chain AI decision audit trail (DecisionLog PDA)
- [x] execute_rebalance with real vault state mutation
- [x] Circuit breaker + Telegram/Discord alerts
- [x] Mixed treasury: stablecoins + volatile assets (ETH/SOL/BTC)
- [x] Live demo simulator (depeg + crypto crash scenarios)
- [ ] Mainnet deploy with Jupiter real swaps
- [ ] Multi-wallet vault (DAO treasury support)
- [ ] On-chain governance for AI parameter updates

---

## Links

- **Program (devnet):** [`GPSJJqicuDSJ6LXhEZpmUboThjzdefG5wZAZkL2hd7es`](https://explorer.solana.com/address/GPSJJqicuDSJ6LXhEZpmUboThjzdefG5wZAZkL2hd7es?cluster=devnet)
- **Deploy TX:** [`34rMqZ...`](https://explorer.solana.com/tx/34rMqZqdbv1p4n5W6jwTUNWQE8y4imQZWTTEmh4C28JfKWYemxNizuL3XAEmwm8agHodFezgSf3SWTn3tfmHZ8xJ?cluster=devnet)
- **Hackathon:** National Solana Hackathon by Decentrathon — Case 2: AI + Blockchain

---

*Built for National Solana Hackathon by Decentrathon*
