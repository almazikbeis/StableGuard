# API Reference — StableGuard

Base URL: `http://localhost:8080/api/v1`

All responses are JSON. No authentication required on devnet.

---

## Health & Status

### `GET /health`
System health check.

```json
{
  "status": "ok",
  "service": "stableguard-backend",
  "pipeline_running": true,
  "ai_configured": true,
  "ai_mode": "live",
  "control_mode": "GUARDED",
  "solana_rpc_url": "https://api.devnet.solana.com"
}
```

---

## Prices & Risk

### `GET /prices`
Current USDC/USDT prices from Pyth + deviation.

```json
{
  "usdc": { "price": 0.9999, "confidence": 0.00054 },
  "usdt": { "price": 0.9999, "confidence": 0.00049 },
  "deviation_pct": 0.005,
  "fetched_at": "2026-04-07T15:37:37Z"
}
```

### `GET /tokens`
All 7 monitored tokens with live prices and asset types.

```json
{
  "tokens": [
    { "symbol": "USDC", "asset_type": "stable",   "price": 0.9999, "vault_slot": 0 },
    { "symbol": "ETH",  "asset_type": "volatile",  "price": 2088.35, "vault_slot": 2 },
    ...
  ],
  "max_deviation": 0.031
}
```

### `GET /risk`
Simple v1 risk score (backward compat).

### `GET /risk/v2`
Full windowed risk score with trend, velocity, volatility.

```json
{
  "risk_level": 1.6,
  "deviation_pct": 0.005,
  "trend": -0.00019,
  "velocity": 0.0,
  "volatility": 0.00145,
  "stable_risk": 0.02,
  "volatile_risk": 0.17,
  "volatile_prices": { "BTC": 68247.5, "ETH": 2088.35, "SOL": 79.37 },
  "action": "hold",
  "summary": "Risk 1.6 < threshold 20 — HOLD"
}
```

---

## Pipeline & AI

### `GET /pipeline/status`
Current AI decision + execution status.

```json
{
  "decision": {
    "action": "HOLD",
    "confidence": 82,
    "rationale": "Risk below threshold — holding",
    "risk_analysis": "Stablecoin pegs exceptionally stable...",
    "yield_analysis": "No spread opportunity at current deviation...",
    "from_index": -1,
    "to_index": -1
  },
  "last_exec_status": "standby",
  "last_exec_sig": "",
  "policy": {
    "verdict": "allowed",
    "control_mode": "GUARDED"
  },
  "risk": { "risk_level": 1.6, ... }
}
```

### `POST /decide`
Force an immediate AI decision cycle (ignores interval timer).

```bash
curl -X POST http://localhost:8080/api/v1/decide
```

### `GET /stream`
SSE real-time stream. Connect once — receive updates every price tick.

```bash
curl -N http://localhost:8080/api/v1/stream
# data: {"ts":1775558252,"risk":{...},"prices":{...},"decision":{...}}
```

---

## Vault

### `GET /vault`
On-chain VaultState account.

```json
{
  "authority": "HNaWnYg4...",
  "balances": [40000000000, 40000000000, 10000000000, 5000000000, 5000000000, 0, 0],
  "total_deposited": 100000000000,
  "decision_count": 2761,
  "total_rebalances": 11,
  "is_paused": false,
  "strategy_mode": 0,
  "num_tokens": 7
}
```

### `POST /rebalance`
Trigger a manual rebalance between vault slots.

```bash
curl -X POST http://localhost:8080/api/v1/rebalance \
  -H "Content-Type: application/json" \
  -d '{"from_index": 2, "to_index": 0, "amount": 500000000}'
```

### `POST /strategy`
Change vault strategy mode.

```bash
curl -X POST http://localhost:8080/api/v1/strategy \
  -H "Content-Type: application/json" \
  -d '{"mode": 1}'   # 0=SAFE, 1=BALANCED/YIELD
```

### `POST /threshold`
Update rebalance risk threshold.

```bash
curl -X POST http://localhost:8080/api/v1/threshold \
  -H "Content-Type: application/json" \
  -d '{"threshold": 15}'
```

### `POST /emergency`
Emergency pause the vault.

```bash
curl -X POST http://localhost:8080/api/v1/emergency
```

---

## History

### `GET /history/decisions?limit=20`
Recent AI decisions with full rationale.

```json
{
  "decisions": [
    {
      "id": 2761,
      "ts": 1775558252,
      "action": "HOLD",
      "confidence": 82,
      "rationale": "Risk below threshold — holding",
      "risk_analysis": "...",
      "yield_analysis": "...",
      "risk_level": 1.6,
      "exec_sig": ""
    }
  ]
}
```

### `GET /history/prices?symbol=USDC&limit=200`
Price history for charting.

### `GET /history/rebalances?limit=20`
Rebalance history with on-chain signatures.

### `GET /history/risk-events?limit=50`
Risk threshold breach events.

### `GET /history/stats`
Aggregate statistics.

```json
{
  "total_decisions": 2761,
  "total_rebalances": 11,
  "total_risk_events": 89,
  "avg_risk_level": 42.5
}
```

---

## Settings & Alerts

### `GET /settings`
Current system configuration.

### `POST /settings/telegram`
Configure Telegram alerts.

```bash
curl -X POST http://localhost:8080/api/v1/settings/telegram \
  -H "Content-Type: application/json" \
  -d '{"bot_token": "123:ABC...", "chat_id": "-100123456"}'
```

### `POST /settings/discord`
Configure Discord webhook.

```bash
curl -X POST http://localhost:8080/api/v1/settings/discord \
  -H "Content-Type: application/json" \
  -d '{"webhook_url": "https://discord.com/api/webhooks/..."}'
```

### `POST /settings/test-alert`
Send a test alert to configured channels.

---

## Demo Endpoints

Used for hackathon demo and devnet testing. Disabled on mainnet.

### `POST /demo/simulate-depeg`
Simulate a stablecoin depeg event and run the full AI pipeline.

```bash
curl -X POST http://localhost:8080/api/v1/demo/simulate-depeg \
  -H "Content-Type: application/json" \
  -d '{"depeg_pct": 2.0}'
```

```json
{
  "depeg_pct": 2.0,
  "score": { "risk_level": 78.4, "action": "rebalance" },
  "decision": { "action": "PROTECT", "confidence": 82, "rationale": "..." },
  "on_chain_sig": "4HVqec...",
  "explorer_url": "https://explorer.solana.com/tx/4HVqec...?cluster=devnet"
}
```

### `POST /demo/simulate-crash`
Simulate a crypto asset crash.

```bash
curl -X POST http://localhost:8080/api/v1/demo/simulate-crash \
  -H "Content-Type: application/json" \
  -d '{"asset": "SOL", "crash_pct": 30}'
```

### `POST /demo/set-balances`
Write demo balances into vault without real SPL tokens (devnet only).

```bash
# Default: $100K treasury (USDC 40% / USDT 40% / ETH 10% / SOL 5% / BTC 5%)
curl -X POST http://localhost:8080/api/v1/demo/set-balances \
  -H "Content-Type: application/json" -d '{}'

# Custom balances
curl -X POST http://localhost:8080/api/v1/demo/set-balances \
  -H "Content-Type: application/json" \
  -d '{"balances": [40000000000, 40000000000, 10000000000, 5000000000, 5000000000, 0, 0, 0]}'
```

### `POST /demo/full-rebalance`
Execute a complete rebalance demo: `execute_rebalance` + `record_decision` + `record_swap_result`.

```bash
curl -X POST http://localhost:8080/api/v1/demo/full-rebalance \
  -H "Content-Type: application/json" \
  -d '{"from_index": 2, "to_index": 0, "amount": 500000000, "confidence": 85}'
```

### `GET /demo/latest-proof`
Returns the latest AI decision with its Solana Explorer link.

```bash
curl http://localhost:8080/api/v1/demo/latest-proof
```

---

## Yield

### `GET /yield/opportunities`
APY data from Kamino, MarginFi, Drift.

### `GET /yield/position`
Current active yield position.

### `GET /yield/history`
Yield position history.

---

## On-Chain Slippage

### `GET /onchain/slippage`
Jupiter price impact at $10K / $100K / $1M trade sizes.

```json
{
  "impact_10k": 0.012,
  "impact_100k": 0.089,
  "impact_1m": 0.94,
  "liquidity_score": 87,
  "drain_detected": false
}
```
