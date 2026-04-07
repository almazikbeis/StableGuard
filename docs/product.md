# Product — StableGuard

## What is StableGuard?

StableGuard is an autonomous AI treasury agent for DAOs and crypto funds. It monitors a multi-asset portfolio in real time, makes intelligent decisions using Claude AI, and executes protective actions directly on Solana — without human intervention.

Think of it as a **24/7 portfolio manager** that never sleeps, never panics, and records every decision on-chain for full accountability.

---

## Who is it for?

### Primary users

**DAO Treasuries**
DAOs like Uniswap, Aave, MakerDAO hold hundreds of millions in stablecoins. When USDC de-pegged during SVB in March 2023, governance proposals took days. StableGuard acts in seconds.

**Crypto Funds & Multisigs**
Investment funds with mixed portfolios (stablecoins + ETH/SOL/BTC) need automated risk management. StableGuard detects market crashes and rebalances autonomously.

**DeFi Protocols**
Protocols holding reserve assets can use StableGuard to protect their treasury from depeg events and volatility spikes.

---

## Core User Scenarios

### Scenario 1: Stablecoin Depeg
```
Situation: USDT starts drifting from $1.00 → $0.982 over 3 minutes

StableGuard response:
  T+0s   Pyth detects $0.997 (deviation 0.3%) → Risk: 22
  T+30s  Pyth detects $0.991 (deviation 0.9%) → Risk: 54
         AI: "Depeg trend confirmed, velocity accelerating"
         Decision: PROTECT (confidence 78%)
         TX: execute_rebalance(from=USDT, to=USDC, amount=40%)  ← on-chain ✓
  T+90s  Pyth detects $0.982 (deviation 1.8%) → circuit breaker
         TX: toggle_pause → vault locked                         ← on-chain ✓
         Alert: Telegram + Discord critical alert sent
```

### Scenario 2: Crypto Market Crash
```
Situation: ETH drops 18% in 2 hours (similar to FTX collapse, Nov 2022)

StableGuard response:
  T+0m   ETH $1,800 → Risk volatile component: 31
  T+20m  ETH $1,530 → Risk volatile component: 72
         AI: "Sustained drawdown confirmed across volatile sleeve"
         Decision: PROTECT (confidence 85%)
         TX: execute_rebalance(from=ETH slot, to=USDC slot)     ← on-chain ✓
  T+40m  ETH $1,480 → Risk: 89 → Emergency alert sent
```

### Scenario 3: Yield Optimization
```
Situation: USDC/USDT spread widens slightly, calm market

StableGuard response:
  Risk: 8/100 (low)
  Agent 2 (Yield): "USDT offers 0.012% spread vs USDC, low risk"
  Decision: OPTIMIZE — shift 15% from USDC to USDT
  TX: execute_rebalance(from=USDC, to=USDT, fraction=0.15)      ← on-chain ✓
```

---

## Key Differentiators

### vs. Manual Treasury Management
| | Manual | StableGuard |
|---|---|---|
| Response time | Days (governance) | Seconds |
| 24/7 monitoring | No | Yes |
| Audit trail | Off-chain spreadsheet | On-chain, immutable |
| Cost per decision | High (human hours) | ~0.002 SOL |

### vs. Simple Rule-Based Bots
| | Rule-based bot | StableGuard |
|---|---|---|
| Decision logic | `if price < X then sell` | Multi-agent AI reasoning |
| Context awareness | None | Portfolio, market, strategy profile |
| Rationale | None | Full text rationale on-chain |
| Adaptability | Static | Configurable profiles + dynamic thresholds |

### vs. Other AI DeFi tools
Most "AI + DeFi" projects give recommendations. StableGuard **executes** — and proves every execution on Solana.

---

## Autonomy Levels

Users choose how much control to give the AI:

```
MANUAL     — AI watches and reports only. You decide.
GUARDED    — AI intervenes only on extreme risk (>80/100).
BALANCED   — Moderate automation with circuit breakers.
YIELD MAX  — Maximum autonomy. AI optimizes yield aggressively.
```

The circuit breaker always applies regardless of mode — vault auto-pauses at 1.5% depeg.

---

## What Gets Stored On-Chain (forever)

Every AI decision is immutable on Solana:

```
DecisionLog PDA {
  action:           PROTECT
  rationale:        "Stablecoin deviation accelerating at 0.3%/min,
                     volatile sleeve showing correlated stress,
                     recommend protective rotation to preserve NAV"
  confidence_score: 85
  timestamp:        2026-04-07 15:42:11 UTC
  sequence:         2761
}
```

This is not a database entry. It is a Solana account — verifiable by anyone, forever.

---

## Supported Assets

| Asset | Type | Role in vault |
|---|---|---|
| USDC | Stablecoin | Safe haven — target for PROTECT actions |
| USDT | Stablecoin | Safe haven + yield spread candidate |
| DAI | Stablecoin | Decentralized alternative |
| PYUSD | Stablecoin | PayPal-backed, monitored for issuer risk |
| ETH | Volatile | Growth sleeve — rebalanced on crash signal |
| SOL | Volatile | Native ecosystem asset |
| BTC | Volatile | Market-wide risk indicator |

---

## Alerts

Configure once → get notified when it matters:

- **Risk > 60** → warning alert
- **Risk > 80** → high risk alert
- **Depeg > 1.5%** → vault paused alert
- **Depeg > 3%** → critical emergency alert

Supported: **Telegram Bot** and **Discord Webhook**. Smart cooldown deduplication prevents spam.
