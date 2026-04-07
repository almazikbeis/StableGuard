# Roadmap — StableGuard

## Current State (Hackathon MVP — April 2026)

Everything below is **built and working** on Solana devnet.

### On-Chain Program (Anchor)
- [x] `initialize_vault` — VaultState PDA with full accounting
- [x] `deposit` / `withdraw` — real SPL token transfers
- [x] `execute_rebalance` — mutates `balances[from]` and `balances[to]` on-chain
- [x] `record_decision` — immutable DecisionLog PDA per AI decision
- [x] `delegate_agent` — AI keypair gets autonomous signing rights
- [x] `toggle_pause` — circuit breaker, auto-triggered by pipeline
- [x] `update_price_and_check` — hot path price push every ~400ms
- [x] `record_swap_result` — on-chain swap receipt
- [x] `set_demo_balances` — devnet demo setup
- [x] Deployed on Solana devnet: `GPSJJqicuDSJ6LXhEZpmUboThjzdefG5wZAZkL2hd7es`

### AI Pipeline
- [x] 3 Claude Haiku agents (Risk Analyst, Yield Analyst, Strategy Agent)
- [x] Windowed risk scorer v2 (deviation + velocity + trend + volatility + crash)
- [x] Configurable decision profiles: cautious / balanced / aggressive
- [x] Dry-run mode (no API key needed for testing)
- [x] 2,761+ real AI decisions logged

### Data & Oracles
- [x] Pyth Network SSE real-time feed (7 assets, ~1s latency)
- [x] Polling fallback if SSE drops
- [x] Volatile asset monitoring: BTC, ETH, SOL crash detection
- [x] DexScreener whale intelligence signals

### Execution & Safety
- [x] Circuit breaker: auto-pause at 1.5% depeg
- [x] Emergency alerts at 3% depeg
- [x] Policy layer: MANUAL / GUARDED / BALANCED / YIELD_MAX
- [x] Jupiter v6 integration (quote + swap, devnet falls back gracefully)

### Frontend
- [x] Live dashboard with SSE real-time updates
- [x] 5-tab interface: Overview, Risk & Intel, Yield, AI Agent, On-Chain
- [x] Live demo simulator (depeg + crypto crash scenarios)
- [x] Pipeline visualizer
- [x] Wallet connect + onboarding flow

### Alerts
- [x] Telegram Bot integration
- [x] Discord Webhook integration
- [x] Smart cooldown deduplication

---

## Phase 2 — Production Ready (Q2 2026)

### Mainnet Deployment
- [ ] Deploy Anchor program to Solana mainnet
- [ ] Jupiter v6 real swap execution (USDC/USDT/DAI mainnet mints)
- [ ] Custody account setup for autonomous swap execution
- [ ] Slippage guardrails and MEV protection

### Multi-Asset Vault
- [ ] Register all 7 token slots with real mainnet mints
- [ ] Real SPL token deposits from user wallets
- [ ] Allocation pie reflects actual on-chain balances

### Yield Automation
- [ ] Real Kamino Finance API integration (live APY, not estimates)
- [ ] Real MarginFi supply/borrow rates
- [ ] Drift Protocol perpetuals for hedging
- [ ] Auto-compound yield into vault

### Security
- [ ] Multisig authority (Squads protocol)
- [ ] Time-lock on strategy changes
- [ ] AI action spend limits (max % per rebalance)
- [ ] Formal security review of Anchor program

---

## Phase 3 — DAO Infrastructure (Q3 2026)

### Multi-Wallet Vault
- [ ] Multiple authorized signers for vault (DAO treasury model)
- [ ] Governance: on-chain vote to change AI parameters
- [ ] Role-based access: viewer / guardian / executor

### AI Upgrades
- [ ] Claude Opus integration for high-stakes decisions
- [ ] Historical backtesting against UST, USDC depeg events
- [ ] Sentiment analysis from on-chain data (whale movements)
- [ ] Cross-protocol risk correlation (if one stablecoin depegs, check others)

### Integrations
- [ ] Realms DAO governance integration
- [ ] Squads multisig native support
- [ ] Jito restaking integration for yield
- [ ] Helius webhooks for real-time event triggers

---

## Phase 4 — Protocol (Q4 2026)

### StableGuard as Infrastructure
- [ ] SDK for other protocols to integrate AI treasury protection
- [ ] White-label vault for DeFi protocols
- [ ] On-chain AI oracle: publish risk scores for other contracts to read
- [ ] Agent marketplace: community-built specialized AI agents

### Tokenomics
- [ ] GUARD token for protocol governance
- [ ] Fee sharing for vault operators
- [ ] Staking for enhanced protection tiers

---

## Key Metrics to Track

| Metric | MVP | Phase 2 | Phase 3 |
|---|---|---|---|
| Assets monitored | 7 | 7 | 20+ |
| AI decisions/day | ~2,900 | ~2,900 | ~2,900 |
| Networks | Devnet | Mainnet | Mainnet + L2s |
| TVL | Demo | Real tokens | DAO adoption |
| Execution | Record-only | Jupiter swaps | Full automation |
| Users | Hackathon | Beta DAOs | Production |

---

## Links

- **Program (devnet):** [`GPSJJqicuDSJ6LXhEZpmUboThjzdefG5wZAZkL2hd7es`](https://explorer.solana.com/address/GPSJJqicuDSJ6LXhEZpmUboThjzdefG5wZAZkL2hd7es?cluster=devnet)
- **Hackathon:** National Solana Hackathon by Decentrathon — Case 2
- **Deadline:** April 7, 2026 23:59 GMT+5
