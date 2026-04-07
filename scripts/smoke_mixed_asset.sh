#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080/api/v1}"

require_bin() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required binary: $1" >&2
    exit 1
  fi
}

require_bin curl
require_bin jq

echo "== StableGuard Mixed-Asset Smoke Check =="
echo "Base URL: $BASE_URL"
echo

health_json="$(curl -fsS "$BASE_URL/health")"

echo "-- Runtime health --"
echo "$health_json" | jq '{
  status,
  pipeline_running,
  control_mode,
  ai_mode,
  mainnet_rpc,
  execution: .execution.mode,
  ready_for_staging: .execution.ready_for_staging,
  ready_for_auto_swap: .execution.ready_for_auto_swap
}'
echo

echo "-- Tracked assets --"
echo "$health_json" | jq '.tracked_assets'
echo

echo "-- Demo event: reserve shock --"
depeg_json="$(curl -fsS -X POST "$BASE_URL/demo/simulate-event" \
  -H 'Content-Type: application/json' \
  -d '{"kind":"depeg","magnitude_pct":2.0}')"
echo "$depeg_json" | jq '{
  kind,
  asset,
  magnitude_pct,
  risk_level: .score.risk_level,
  action: .decision.action,
  from_index: .decision.from_index,
  to_index: .decision.to_index,
  on_chain_sig,
  error
}'
echo

echo "-- Demo event: crypto crash --"
crash_json="$(curl -fsS -X POST "$BASE_URL/demo/simulate-event" \
  -H 'Content-Type: application/json' \
  -d '{"kind":"crash","asset":"BTC","magnitude_pct":15.0}')"
echo "$crash_json" | jq '{
  kind,
  asset,
  magnitude_pct,
  price_before,
  price_after,
  risk_level: .score.risk_level,
  action: .decision.action,
  from_index: .decision.from_index,
  to_index: .decision.to_index,
  on_chain_sig,
  error
}'
echo

echo "-- Demo rebalance path --"
rebalance_json="$(curl -fsS -X POST "$BASE_URL/demo/full-rebalance" \
  -H 'Content-Type: application/json' \
  -d '{"from_index":2,"to_index":0,"amount":500000000,"rationale":"Smoke test rotation from ETH into USDC reserve sleeve","confidence":82}')"
echo "$rebalance_json" | jq '{
  message,
  note,
  error,
  step_count: (.steps | length),
  steps: [.steps[] | {step, name, sig}]
}'
echo

echo "Smoke check completed."
