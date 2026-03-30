#!/usr/bin/env node
/**
 * Solana Trading Monitor - Autonomous Trading Bot
 * 
 * Environment Configuration:
 * - SOLANA_RPC_URL: Solana RPC endpoint
 * - SOLANA_WALLET_PATH: Path to wallet keypair JSON
 * - POSITION_SIZE_USD: Max USD per position (default: 10)
 * - MAX_POSITIONS: Max concurrent positions (default: 4) 
 * - MIN_SCORE: Minimum opportunity score (default: 25)
 * - TAKE_PROFIT_PCT: Take profit percentage (default: 50)
 * - STOP_LOSS_PCT: Stop loss percentage (default: -25)
 * - TRAILING_STOP_PCT: Trailing stop percentage (default: 15)
 */

const { Connection, Keypair, VersionedTransaction } = require("@solana/web3.js");
const fs = require("fs");
const path = require("path");

// Configuration from environment
const CONFIG = {
  rpcUrl: process.env.SOLANA_RPC_URL || "https://api.mainnet-beta.solana.com",
  walletPath: process.env.SOLANA_WALLET_PATH || path.join(process.env.HOME, ".openclaw/workspace/solana-wallet.json"),
  positionSizeUsd: parseFloat(process.env.POSITION_SIZE_USD) || 10,
  maxPositions: parseInt(process.env.MAX_POSITIONS) || 4,
  minScore: parseFloat(process.env.MIN_SCORE) || 25,
  takeProfitPct: parseFloat(process.env.TAKE_PROFIT_PCT) || 50,
  stopLossPct: parseFloat(process.env.STOP_LOSS_PCT) || -25,
  trailingStopPct: parseFloat(process.env.TRAILING_STOP_PCT) || 15,
};

const JUPITER_LITE = "https://lite-api.jup.ag";
const WORKSPACE_DIR = process.env.HOME + "/.openclaw/workspace";
const TRADES_PATH = path.join(WORKSPACE_DIR, "trades.json");
const STATE_PATH = path.join(WORKSPACE_DIR, "trading-state.json");

const MINTS = {
  SOL: "So11111111111111111111111111111111111111112",
  USDC: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
};

// Ensure workspace directory exists
if (!fs.existsSync(WORKSPACE_DIR)) {
  fs.mkdirSync(WORKSPACE_DIR, { recursive: true });
}

function loadWallet() {
  const secretKey = JSON.parse(fs.readFileSync(CONFIG.walletPath, "utf8"));
  return Keypair.fromSecretKey(Uint8Array.from(secretKey));
}

function loadTrades() {
  if (fs.existsSync(TRADES_PATH)) return JSON.parse(fs.readFileSync(TRADES_PATH, "utf8"));
  return [];
}

function saveTrades(trades) { 
  fs.writeFileSync(TRADES_PATH, JSON.stringify(trades, null, 2)); 
}

function loadState() {
  if (fs.existsSync(STATE_PATH)) return JSON.parse(fs.readFileSync(STATE_PATH, "utf8"));
  return { 
    positions: {}, 
    usdcBalance: 100, 
    solBalance: 0.2, 
    startingValueUsd: 120, 
    strategyStats: {},
    lastRun: null
  };
}

function saveState(state) { 
  state.lastRun = new Date().toISOString();
  fs.writeFileSync(STATE_PATH, JSON.stringify(state, null, 2)); 
}

async function fetchJSON(url, opts) {
  const res = await fetch(url, opts);
  if (res.status === 429) throw new Error("Rate limited");
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${res.statusText}`);
  return res.json();
}

function sleep(ms) { 
  return new Promise(r => setTimeout(r, ms)); 
}

async function getOrder(inputMint, outputMint, amount, taker) {
  const data = await fetchJSON(`${JUPITER_LITE}/ultra/v1/order?inputMint=${inputMint}&outputMint=${outputMint}&amount=${amount}&taker=${taker}`);
  if (data.errorCode || data.error) throw new Error(data.errorMessage || data.error || JSON.stringify(data));
  return data;
}

async function doSwap(inputMint, outputMint, amountRaw, label) {
  const wallet = loadWallet();
  const taker = wallet.publicKey.toBase58();
  
  try {
    const order = await getOrder(inputMint, outputMint, amountRaw.toString(), taker);
    
    const tx = VersionedTransaction.deserialize(Buffer.from(order.transaction, "base64"));
    tx.sign([wallet]);
    const signed = Buffer.from(tx.serialize()).toString("base64");
    
    const result = await fetchJSON(`${JUPITER_LITE}/ultra/v1/execute`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ signedTransaction: signed, requestId: order.requestId }),
    });

    const trades = loadTrades();
    const trade = {
      id: trades.length + 1,
      timestamp: new Date().toISOString(),
      inputMint, outputMint, label,
      inputAmount: order.inAmount,
      outputAmount: order.outAmount,
      inUsdValue: order.inUsdValue,
      outUsdValue: order.outUsdValue,
      priceImpact: order.priceImpactPct,
      router: order.router,
      signature: result.signature || null,
      status: result.status || "unknown",
    };
    trades.push(trade);
    saveTrades(trades);
    
    return { order, result, trade };
  } catch (error) {
    console.error(`Swap failed (${label}):`, error.message);
    throw error;
  }
}

async function getSolPrice() {
  const data = await fetchJSON("https://api.coingecko.com/api/v3/simple/price?ids=solana&vs_currencies=usd&include_24hr_change=true");
  return data.solana;
}

async function getTokenInfo(mint) {
  const data = await fetchJSON(`https://api.dexscreener.com/latest/dex/tokens/${mint}`);
  const pairs = (data.pairs || []).sort((a, b) => (b.liquidity?.usd || 0) - (a.liquidity?.usd || 0));
  if (!pairs.length) return null;
  
  const p = pairs[0];
  return {
    symbol: p.baseToken?.symbol || 'UNKNOWN',
    priceUsd: parseFloat(p.priceUsd || 0),
    m5: p.priceChange?.m5 || 0,
    h1: p.priceChange?.h1 || 0,
    h6: p.priceChange?.h6 || 0,
    h24: p.priceChange?.h24 || 0,
    vol24h: p.volume?.h24 || 0,
    liq: p.liquidity?.usd || 0,
    buys24h: p.txns?.h24?.buys || 0,
    sells24h: p.txns?.h24?.sells || 0,
    fdv: p.fdv || 0,
  };
}

async function findOpportunities() {
  const candidates = [];
  const seen = new Set();
  
  console.log("  🔍 Scanning DexScreener boosted tokens...");
  
  // DexScreener boosted tokens (paid promotion = attention)
  try {
    const boosted = await fetchJSON("https://api.dexscreener.com/token-boosts/latest/v1");
    const solTokens = (boosted || []).filter(t => t.chainId === "solana");
    for (const t of solTokens.slice(0, 10)) {
      if (!seen.has(t.tokenAddress)) {
        seen.add(t.tokenAddress);
      }
    }
    console.log(`    Found ${solTokens.length} boosted tokens`);
  } catch (e) {
    console.warn("    DexScreener boosted error:", e.message);
  }
  
  await sleep(1500); // Rate limit
  
  console.log("  🔍 Scanning GeckoTerminal trending pools...");
  
  // GeckoTerminal trending pools
  try {
    const data = await fetchJSON("https://api.geckoterminal.com/api/v2/networks/solana/trending_pools?include=base_token");
    for (const pool of (data.data || []).slice(0, 15)) {
      const baseTokenId = pool.relationships?.base_token?.data?.id || "";
      const tokenAddr = baseTokenId.replace("solana_", "");
      if (tokenAddr && tokenAddr.length > 10 && !seen.has(tokenAddr)) {
        seen.add(tokenAddr);
      }
    }
    console.log(`    Found ${data.data?.length || 0} trending pools`);
  } catch (e) {
    console.warn("    GeckoTerminal trending error:", e.message);
  }
  
  await sleep(1500); // Rate limit
  
  console.log("  🔍 Scanning GeckoTerminal new pools...");
  
  // GeckoTerminal new pools (early opportunities)
  try {
    const data = await fetchJSON("https://api.geckoterminal.com/api/v2/networks/solana/new_pools?page=1&include=base_token");
    for (const pool of (data.data || []).slice(0, 20)) {
      const attr = pool.attributes;
      const liq = parseFloat(attr.reserve_in_usd || 0);
      const ageHours = (Date.now() - new Date(attr.pool_created_at).getTime()) / 3600000;
      
      if (liq >= 10000 && ageHours < 12) {
        const baseTokenId = pool.relationships?.base_token?.data?.id || "";
        const tokenAddr = baseTokenId.replace("solana_", "");
        if (tokenAddr && tokenAddr.length > 10 && !seen.has(tokenAddr)) {
          seen.add(tokenAddr);
        }
      }
    }
    console.log(`    Found ${data.data?.length || 0} new pools`);
  } catch (e) {
    console.warn("    GeckoTerminal new pools error:", e.message);
  }
  
  console.log(`  📊 Scoring ${seen.size} unique tokens...`);
  
  // Score all candidates via DexScreener
  let lookupCount = 0;
  for (const mint of seen) {
    if (lookupCount > 0 && lookupCount % 5 === 0) await sleep(1000); // Pace requests
    lookupCount++;
    
    try {
      const info = await getTokenInfo(mint);
      if (!info) continue;
      
      // Filter: need minimum liquidity and volume
      if (info.liq < 10000) continue;
      if (info.vol24h < 5000 && info.liq < 50000) continue;
      
      const score = calculateScore(info);
      candidates.push({ mint, ...info, score });
    } catch (e) {
      // Silent fail on individual token lookups
    }
  }
  
  return candidates.sort((a, b) => b.score - a.score);
}

function calculateScore(info) {
  let score = 0;
  
  // Early stage bonus (lower FDV = more upside)
  if (info.fdv > 0 && info.fdv < 500000) score += 30;       // Micro cap
  else if (info.fdv < 2000000) score += 20;                 // Small cap
  else if (info.fdv < 10000000) score += 10;                // Mid cap
  
  // Momentum (rising price)
  if (info.m5 > 0 && info.m5 < 15) score += info.m5 * 3;   // Steady rise
  if (info.h1 > 0 && info.h1 < 30) score += info.h1 * 2;   // Hourly momentum
  if (info.h1 > 5 && info.m5 > 0) score += 15;             // Accelerating
  
  // Volume/Liquidity ratio (high = hype)
  const volLiqRatio = info.vol24h / (info.liq || 1);
  if (volLiqRatio > 2) score += 20;
  if (volLiqRatio > 5) score += 15;
  
  // Buy pressure
  const buyRatio = info.buys24h / (info.sells24h || 1);
  if (buyRatio > 1.3) score += 15;
  if (buyRatio > 2) score += 10;
  
  // Penalties
  if (info.m5 > 30) score -= 25;        // Already pumped (rug risk)
  if (info.h1 < -15) score -= 20;       // Dumping
  if (info.h24 < -40) score -= 20;      // Dead
  if (info.liq < 15000) score -= 10;    // Too thin
  if (buyRatio < 0.5) score -= 15;      // Sell pressure
  
  return Math.max(0, score);
}

async function checkPositionExits(state) {
  const exits = [];
  
  for (const [mint, pos] of Object.entries(state.positions)) {
    if (mint === MINTS.SOL || mint === MINTS.USDC) continue;
    
    try {
      const info = await getTokenInfo(mint);
      if (!info) continue;
      
      const currentValue = info.priceUsd * pos.amount / Math.pow(10, pos.decimals || 6);
      const pnlPct = pos.totalCostUsd > 0 ? ((currentValue - pos.totalCostUsd) / pos.totalCostUsd) * 100 : 0;
      
      pos.currentValue = currentValue;
      pos.currentPrice = info.priceUsd;
      pos.pnlPct = pnlPct;
      
      // Clean up dust positions
      if (currentValue < 0.50 && pos.totalCostUsd < 0.50) {
        console.log(`    ${pos.symbol}: dust position ($${currentValue.toFixed(4)}) — skipping`);
        continue;
      }
      
      // Skip positions with no valid cost basis
      if (!pos.totalCostUsd || pos.totalCostUsd <= 0) {
        console.log(`    ${pos.symbol}: no cost basis — skipping exit check`);
        continue;
      }
      
      console.log(`    ${pos.symbol}: cost=$${pos.totalCostUsd.toFixed(2)} now=$${currentValue.toFixed(2)} pnl=${pnlPct.toFixed(1)}% | 5m:${info.m5}% 1h:${info.h1}%`);
      
      // Track highest PnL for trailing stop
      if (!pos.highPnlPct || pnlPct > pos.highPnlPct) {
        pos.highPnlPct = pnlPct;
      }
      
      // Exit logic
      if (pnlPct > CONFIG.takeProfitPct) {
        exits.push({ mint, reason: "take_profit", pnlPct, pos, info });
      }
      else if (pos.highPnlPct > 30 && pnlPct < pos.highPnlPct - CONFIG.trailingStopPct) {
        exits.push({ mint, reason: "trailing_stop", pnlPct, pos, info });
      }
      else if (pnlPct < CONFIG.stopLossPct) {
        exits.push({ mint, reason: "stop_loss", pnlPct, pos, info });
      }
      else if (info.m5 < -8 && info.h1 < -15) {
        exits.push({ mint, reason: "momentum_loss", pnlPct, pos, info });
      }
    } catch (e) {
      console.warn(`    Error checking ${pos.symbol || mint}:`, e.message);
    }
  }
  
  return exits;
}

async function run() {
  console.log(`\n${"=".repeat(80)}`);
  console.log(`🤖 SOLANA TRADING MONITOR - ${new Date().toISOString()}`);
  console.log(`${"=".repeat(80)}`);
  
  const state = loadState();
  if (!state.startingValueUsd) state.startingValueUsd = 120;
  
  const sol = await getSolPrice();
  console.log(`💎 SOL: $${sol.usd.toFixed(2)} (24h: ${sol.usd_24h_change?.toFixed(1)}%)`);
  console.log(`💰 Portfolio: USDC=$${state.usdcBalance.toFixed(2)} SOL=${state.solBalance.toFixed(4)} ($${(state.solBalance * sol.usd).toFixed(2)})`);
  
  // === 1. Check exits on spot positions ===
  console.log(`\n📊 [1] Checking ${Object.keys(state.positions).length} Positions`);
  const exits = await checkPositionExits(state);
  
  for (const exit of exits) {
    console.log(`\n🚨 EXIT: ${exit.pos.symbol} (${exit.reason}, PnL: ${exit.pnlPct.toFixed(1)}%)`);
    try {
      const result = await doSwap(exit.mint, MINTS.USDC, exit.pos.amount.toString(), `exit_${exit.reason}_${exit.pos.symbol}`);
      const received = parseInt(result.order.outAmount) / 1e6;
      state.usdcBalance += received;
      
      // Track stats
      const costBasis = exit.pos.totalCostUsd || 0;
      const pnl = received - costBasis;
      const strat = exit.pos.strategy || "momentum";
      if (!state.strategyStats[strat]) state.strategyStats[strat] = { wins: 0, losses: 0, totalPnl: 0 };
      
      if (costBasis > 0) {
        state.strategyStats[strat].totalPnl += pnl;
        if (pnl > 0) state.strategyStats[strat].wins++;
        else state.strategyStats[strat].losses++;
        console.log(`    📈 ${strat} ${pnl > 0 ? 'WIN' : 'LOSS'} $${pnl.toFixed(2)} | Record: ${state.strategyStats[strat].wins}W/${state.strategyStats[strat].losses}L`);
      }
      
      delete state.positions[exit.mint];
      saveState(state); // Save immediately after each exit
      
      console.log(`    ✅ Sold for $${received.toFixed(2)} | PnL: $${pnl.toFixed(2)}`);
    } catch (e) {
      console.error(`    ❌ Exit failed:`, e.message);
    }
  }
  
  // === 2. SOL swing logic ===
  console.log(`\n💎 [2] SOL Swing Trading`);
  if (sol.usd_24h_change < -7 && state.usdcBalance >= 15) {
    console.log(`    🟢 SOL deep dip (${sol.usd_24h_change.toFixed(1)}%) — buying`);
    try {
      const buyAmt = Math.floor(Math.min(15, state.usdcBalance * 0.25) * 1e6);
      const result = await doSwap(MINTS.USDC, MINTS.SOL, buyAmt, `sol_dip_buy`);
      state.usdcBalance -= parseInt(result.order.inAmount) / 1e6;
      state.solBalance += parseInt(result.order.outAmount) / 1e9;
      console.log(`    ✅ Bought SOL`);
    } catch (e) {
      console.error(`    ❌ SOL buy failed:`, e.message);
    }
  } else if (sol.usd_24h_change > 6 && state.solBalance > 0.25) {
    console.log(`    🟡 SOL pumping (${sol.usd_24h_change.toFixed(1)}%) — taking profit`);
    try {
      const sellAmt = Math.floor(Math.min(state.solBalance * 0.3, 0.15) * 1e9);
      const result = await doSwap(MINTS.SOL, MINTS.USDC, sellAmt, `sol_profit_take`);
      state.solBalance -= parseInt(result.order.inAmount) / 1e9;
      state.usdcBalance += parseInt(result.order.outAmount) / 1e6;
      console.log(`    ✅ Sold SOL`);
    } catch (e) {
      console.error(`    ❌ SOL sell failed:`, e.message);
    }
  } else {
    console.log(`    ⏸️  No swing trigger (24h: ${sol.usd_24h_change?.toFixed(1)}%)`);
  }
  
  // === 3. Momentum entries ===
  console.log(`\n🎯 [3] Momentum Scanning`);
  const posCount = Object.keys(state.positions).length;
  
  if (state.usdcBalance >= CONFIG.positionSizeUsd && posCount < CONFIG.maxPositions) {
    const opps = await findOpportunities();
    const newOpps = opps.filter(o => !state.positions[o.mint]);
    
    // Log top candidates
    console.log("  📋 Top Opportunities:");
    for (const o of newOpps.slice(0, 8)) {
      console.log(`    ${o.symbol}: score=${o.score.toFixed(0)} fdv=$${(o.fdv/1000).toFixed(0)}k liq=$${(o.liq/1000).toFixed(0)}k m5=${o.m5}% h1=${o.h1}%`);
    }
    
    // Buy top picks that pass threshold
    const slots = CONFIG.maxPositions - posCount;
    const picks = newOpps.filter(o => o.score >= CONFIG.minScore).slice(0, Math.min(slots, 2));
    
    for (const pick of picks) {
      const posSize = Math.min(CONFIG.positionSizeUsd, Math.max(3, state.usdcBalance * 0.25));
      if (state.usdcBalance < posSize) break;
      
      console.log(`\n  🟢 BUY: ${pick.symbol} (score: ${pick.score.toFixed(0)}, fdv: $${(pick.fdv/1000).toFixed(0)}k)`);
      
      try {
        const result = await doSwap(MINTS.USDC, pick.mint, Math.floor(posSize * 1e6).toString(), `momentum_buy_${pick.symbol}`);
        state.usdcBalance -= parseInt(result.order.inAmount) / 1e6;
        state.positions[pick.mint] = {
          amount: parseInt(result.order.outAmount),
          totalCostUsd: parseFloat(result.order.inUsdValue) || posSize,
          symbol: pick.symbol,
          entryPrice: pick.priceUsd,
          decimals: 6,
          strategy: "momentum",
          entryTime: new Date().toISOString(),
          highPnlPct: 0,
          fdvAtEntry: pick.fdv,
          scoreAtEntry: pick.score,
        };
        saveState(state);
        console.log(`    ✅ Bought ${pick.symbol} for $${posSize.toFixed(2)}`);
      } catch (e) {
        console.error(`    ❌ Buy failed:`, e.message);
      }
    }
    
    if (picks.length === 0) {
      console.log(`    ⏸️  No opportunities above score ${CONFIG.minScore}`);
    }
  } else {
    console.log(`    ⏸️  Skipping (USDC: $${state.usdcBalance.toFixed(2)}, positions: ${posCount}/${CONFIG.maxPositions})`);
  }
  
  // === Final state save & summary ===
  saveState(state);
  
  const totalSolValue = state.solBalance * sol.usd;
  let posValue = 0;
  for (const pos of Object.values(state.positions)) {
    posValue += pos.currentValue || pos.totalCostUsd;
  }
  
  const totalValue = state.usdcBalance + totalSolValue + posValue;
  const pnl = totalValue - state.startingValueUsd;
  
  console.log(`\n${"=".repeat(80)}`);
  console.log(`📊 PORTFOLIO SUMMARY`);
  console.log(`${"=".repeat(80)}`);
  console.log(`  USDC (liquid):  $${state.usdcBalance.toFixed(2)}`);
  console.log(`  SOL:            $${totalSolValue.toFixed(2)} (${state.solBalance.toFixed(4)} SOL)`);
  console.log(`  Spot Positions: $${posValue.toFixed(2)} (${Object.keys(state.positions).length} tokens)`);
  console.log(`  ─────────────────────────────`);
  console.log(`  TOTAL:          $${totalValue.toFixed(2)}`);
  console.log(`  PnL:            $${pnl.toFixed(2)} (${((pnl/state.startingValueUsd)*100).toFixed(2)}%)`);
  
  if (Object.keys(state.strategyStats).length > 0) {
    console.log(`  Strategy Stats:`);
    for (const [strat, stats] of Object.entries(state.strategyStats)) {
      const winRate = stats.wins + stats.losses > 0 ? (stats.wins / (stats.wins + stats.losses) * 100).toFixed(1) : '0.0';
      console.log(`    ${strat}: ${stats.wins}W/${stats.losses}L (${winRate}%) $${stats.totalPnl.toFixed(2)}`);
    }
  }
  
  console.log(`${"=".repeat(80)}\n`);
}

// Error handling and execution
if (require.main === module) {
  run().catch(error => {
    console.error("\n💥 Fatal Error:", error);
    process.exit(1);
  });
}

module.exports = { run, loadState, saveState };