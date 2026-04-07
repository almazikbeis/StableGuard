#!/usr/bin/env bash
# StableGuard devnet setup script
# Creates devnet USDC/USDT test tokens, initializes vault, registers tokens, deposits.
# Run: bash scripts/setup_devnet.sh
set -euo pipefail

WALLET="/Users/almazbeisenov/.config/solana/devnet.json"
RPC="https://api.devnet.solana.com"
PROGRAM_ID="GPSJJqicuDSJ6LXhEZpmUboThjzdefG5wZAZkL2hd7es"

echo "=== StableGuard Devnet Setup ==="
echo ""

# ── 1. Check wallet ──────────────────────────────────────────────────────────
PUBKEY=$(solana-keygen pubkey "$WALLET")
echo "Wallet: $PUBKEY"

BALANCE=$(solana balance "$PUBKEY" --url "$RPC" | awk '{print $1}')
echo "Current balance: $BALANCE SOL"

# Airdrop if needed
if (( $(echo "$BALANCE < 2" | bc -l) )); then
  echo "Requesting airdrop..."
  solana airdrop 2 "$PUBKEY" --url "$RPC" || true
  sleep 5
  BALANCE=$(solana balance "$PUBKEY" --url "$RPC" | awk '{print $1}')
  echo "New balance: $BALANCE SOL"
fi

# ── 2. Create USDC-test and USDT-test mints ──────────────────────────────────
echo ""
echo "=== Creating test token mints ==="

# USDC test mint (6 decimals)
echo "Creating USDC-test mint..."
USDC_MINT=$(spl-token create-token --decimals 6 --url "$RPC" --owner "$WALLET" 2>&1 | grep "Creating token" | awk '{print $3}')
echo "USDC-test mint: $USDC_MINT"

# USDT test mint (6 decimals)
echo "Creating USDT-test mint..."
USDT_MINT=$(spl-token create-token --decimals 6 --url "$RPC" --owner "$WALLET" 2>&1 | grep "Creating token" | awk '{print $3}')
echo "USDT-test mint: $USDT_MINT"

# ── 3. Create authority token accounts ──────────────────────────────────────
echo ""
echo "=== Creating authority token accounts ==="

USDC_ATA=$(spl-token create-account "$USDC_MINT" --url "$RPC" --owner "$WALLET" 2>&1 | grep "Creating account" | awk '{print $3}')
echo "USDC ATA: $USDC_ATA"

USDT_ATA=$(spl-token create-account "$USDT_MINT" --url "$RPC" --owner "$WALLET" 2>&1 | grep "Creating account" | awk '{print $3}')
echo "USDT ATA: $USDT_ATA"

# ── 4. Mint test tokens to authority ────────────────────────────────────────
echo ""
echo "=== Minting test tokens ==="

# Mint 10,000 USDC-test (10000 * 10^6 = 10000000000 base units)
spl-token mint "$USDC_MINT" 10000 "$USDC_ATA" --url "$RPC" --owner "$WALLET"
echo "Minted 10,000 USDC-test"

spl-token mint "$USDT_MINT" 10000 "$USDT_ATA" --url "$RPC" --owner "$WALLET"
echo "Minted 10,000 USDT-test"

# ── 5. Write env file ────────────────────────────────────────────────────────
echo ""
echo "=== Writing .env.devnet ==="

cat > backend/.env.devnet <<EOF
ANTHROPIC_API_KEY=$(grep ANTHROPIC_API_KEY backend/.env | cut -d= -f2-)
SOLANA_RPC_URL=https://api.devnet.solana.com
PROGRAM_ID=$PROGRAM_ID
WALLET_KEY_PATH=$WALLET
PYTH_HERMES_URL=https://hermes.pyth.network
PORT=8080
AI_AGENT_MODEL=claude-haiku-4-5
AI_DECISION_PROFILE=balanced
AI_INTERVAL_SEC=30
AUTO_EXECUTE=true
STRATEGY_MODE=1
CIRCUIT_BREAKER_ENABLED=true
CIRCUIT_BREAKER_PAUSE_PCT=1.5
CIRCUIT_BREAKER_EMERGENCY_PCT=3.0
EXECUTION_APPROVAL_MODE=auto
EXECUTION_MAX_SLIPPAGE_BPS=50
EXECUTION_MAX_PRICE_IMPACT_PCT=1.5
EXECUTION_MAX_ROUTE_HOPS=3
EXECUTION_AUTO_SETTLE=true
EXECUTION_CUSTODY_USDC_ACCOUNT=$USDC_ATA
EXECUTION_CUSTODY_USDT_ACCOUNT=$USDT_ATA
YIELD_ENABLED=false
GROWTH_SLEEVE_ENABLED=false
# Token mints (devnet test tokens)
MINT_A=$USDC_MINT
MINT_B=$USDT_MINT
EOF

echo ""
echo "=== Setup complete! ==="
echo ""
echo "USDC mint:     $USDC_MINT"
echo "USDT mint:     $USDT_MINT"
echo "USDC account:  $USDC_ATA"
echo "USDT account:  $USDT_ATA"
echo ""
echo "Next steps:"
echo "  1. Deploy program:  anchor build && anchor deploy"
echo "  2. Init vault:      curl -X POST http://localhost:8080/api/v1/demo/init-vault"
echo "  3. Register USDC:   curl -X POST http://localhost:8080/api/v1/register-token -d '{\"mint\":\"$USDC_MINT\",\"token_index\":0}'"
echo "  4. Register USDT:   curl -X POST http://localhost:8080/api/v1/register-token -d '{\"mint\":\"$USDT_MINT\",\"token_index\":1}'"
echo "  5. Deposit USDC:    curl -X POST http://localhost:8080/api/v1/vault/deposit -d '{\"token_index\":0,\"amount\":1000000000,\"authority_token_account\":\"$USDC_ATA\"}'"
echo "  6. Run simulation:  curl -X POST http://localhost:8080/api/v1/demo/simulate-depeg -d '{\"depeg_pct\":2.1}'"
