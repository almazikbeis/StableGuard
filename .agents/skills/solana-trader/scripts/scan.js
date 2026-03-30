#!/usr/bin/env node
/**
 * Solana Token Opportunity Scanner
 * 
 * Standalone script that scans for trading opportunities and outputs JSON
 * Usage: node scan.js [maxResults]
 */

const https = require('https');
const http = require('http');

const MAX_RESULTS = parseInt(process.argv[2]) || 10;

async function fetchJSON(url) {
  return new Promise((resolve, reject) => {
    const client = url.startsWith('https') ? https : http;
    const req = client.get(url, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          if (res.statusCode === 429) {
            reject(new Error('Rate limited'));
          } else if (res.statusCode !== 200) {
            reject(new Error(`HTTP ${res.statusCode}: ${res.statusMessage}`));
          } else {
            resolve(JSON.parse(data));
          }
        } catch (error) {
          reject(error);
        }
      });
    });
    req.on('error', reject);
    req.setTimeout(10000, () => {
      req.destroy();
      reject(new Error('Request timeout'));
    });
  });
}

function sleep(ms) {
  return new Promise(r => setTimeout(r, ms));
}

async function getTokenInfo(mint) {
  try {
    const data = await fetchJSON(`https://api.dexscreener.com/latest/dex/tokens/${mint}`);
    const pairs = (data.pairs || []).sort((a, b) => (b.liquidity?.usd || 0) - (a.liquidity?.usd || 0));
    if (!pairs.length) return null;
    
    const p = pairs[0];
    return {
      mint,
      symbol: p.baseToken?.symbol || 'UNKNOWN',
      priceUsd: parseFloat(p.priceUsd || 0),
      priceChange: {
        m5: p.priceChange?.m5 || 0,
        h1: p.priceChange?.h1 || 0,
        h6: p.priceChange?.h6 || 0,
        h24: p.priceChange?.h24 || 0,
      },
      volume24h: p.volume?.h24 || 0,
      liquidity: p.liquidity?.usd || 0,
      fdv: p.fdv || 0,
      transactions: {
        buys24h: p.txns?.h24?.buys || 0,
        sells24h: p.txns?.h24?.sells || 0,
      },
      dexscreenerUrl: `https://dexscreener.com/solana/${p.pairAddress}`,
    };
  } catch (error) {
    return null;
  }
}

function calculateScore(info) {
  let score = 0;
  const pc = info.priceChange;
  const liq = info.liquidity;
  const vol = info.volume24h;
  const fdv = info.fdv;
  const buys = info.transactions.buys24h;
  const sells = info.transactions.sells24h;
  
  // Early stage bonus (lower FDV = more upside)
  if (fdv > 0 && fdv < 500000) score += 30;       // Micro cap
  else if (fdv < 2000000) score += 20;             // Small cap
  else if (fdv < 10000000) score += 10;            // Mid cap
  
  // Momentum (rising price)
  if (pc.m5 > 0 && pc.m5 < 15) score += pc.m5 * 3;   // Steady rise
  if (pc.h1 > 0 && pc.h1 < 30) score += pc.h1 * 2;   // Hourly momentum
  if (pc.h1 > 5 && pc.m5 > 0) score += 15;           // Accelerating
  
  // Volume/Liquidity ratio (high = hype)
  const volLiqRatio = vol / (liq || 1);
  if (volLiqRatio > 2) score += 20;
  if (volLiqRatio > 5) score += 15;
  
  // Buy pressure
  const buyRatio = buys / (sells || 1);
  if (buyRatio > 1.3) score += 15;
  if (buyRatio > 2) score += 10;
  
  // Penalties
  if (pc.m5 > 30) score -= 25;        // Already pumped
  if (pc.h1 < -15) score -= 20;       // Dumping
  if (pc.h24 < -40) score -= 20;      // Dead
  if (liq < 15000) score -= 10;       // Too thin
  if (buyRatio < 0.5) score -= 15;    // Sell pressure
  
  return {
    score: Math.max(0, score),
    details: {
      fdvBonus: fdv > 0 && fdv < 500000 ? 30 : fdv < 2000000 ? 20 : fdv < 10000000 ? 10 : 0,
      momentumScore: (pc.m5 > 0 && pc.m5 < 15 ? pc.m5 * 3 : 0) + (pc.h1 > 0 && pc.h1 < 30 ? pc.h1 * 2 : 0),
      volumeScore: volLiqRatio > 2 ? (volLiqRatio > 5 ? 35 : 20) : 0,
      buyPressureScore: buyRatio > 1.3 ? (buyRatio > 2 ? 25 : 15) : 0,
      penalties: (pc.m5 > 30 ? -25 : 0) + (pc.h1 < -15 ? -20 : 0) + (pc.h24 < -40 ? -20 : 0) + (liq < 15000 ? -10 : 0) + (buyRatio < 0.5 ? -15 : 0),
      volLiqRatio: volLiqRatio.toFixed(2),
      buyRatio: buyRatio.toFixed(2),
    }
  };
}

async function scanOpportunities() {
  const startTime = Date.now();
  const results = {
    timestamp: new Date().toISOString(),
    scanDuration: null,
    sources: [],
    opportunities: [],
    summary: {
      totalScanned: 0,
      validTokens: 0,
      topScore: 0,
    }
  };
  
  const candidates = new Set();
  
  // === Source 1: DexScreener Boosted ===
  try {
    const boosted = await fetchJSON('https://api.dexscreener.com/token-boosts/latest/v1');
    const solTokens = (boosted || []).filter(t => t.chainId === 'solana');
    
    for (const token of solTokens.slice(0, 15)) {
      candidates.add(token.tokenAddress);
    }
    
    results.sources.push({
      name: 'dexscreener-boosted',
      tokens: solTokens.length,
      status: 'success'
    });
  } catch (error) {
    results.sources.push({
      name: 'dexscreener-boosted',
      tokens: 0,
      status: 'error',
      error: error.message
    });
  }
  
  await sleep(1500);
  
  // === Source 2: GeckoTerminal Trending ===
  try {
    const trending = await fetchJSON('https://api.geckoterminal.com/api/v2/networks/solana/trending_pools?include=base_token');
    
    for (const pool of (trending.data || []).slice(0, 15)) {
      const baseTokenId = pool.relationships?.base_token?.data?.id || '';
      const tokenAddr = baseTokenId.replace('solana_', '');
      if (tokenAddr && tokenAddr.length > 10) {
        candidates.add(tokenAddr);
      }
    }
    
    results.sources.push({
      name: 'geckoterminal-trending',
      pools: trending.data?.length || 0,
      status: 'success'
    });
  } catch (error) {
    results.sources.push({
      name: 'geckoterminal-trending',
      pools: 0,
      status: 'error',
      error: error.message
    });
  }
  
  await sleep(1500);
  
  // === Source 3: GeckoTerminal New Pools ===
  try {
    const newPools = await fetchJSON('https://api.geckoterminal.com/api/v2/networks/solana/new_pools?page=1&include=base_token');
    
    for (const pool of (newPools.data || []).slice(0, 20)) {
      const attr = pool.attributes;
      const liq = parseFloat(attr.reserve_in_usd || 0);
      const ageHours = (Date.now() - new Date(attr.pool_created_at).getTime()) / 3600000;
      
      // Only include pools with good liquidity and recent creation
      if (liq >= 10000 && ageHours < 12) {
        const baseTokenId = pool.relationships?.base_token?.data?.id || '';
        const tokenAddr = baseTokenId.replace('solana_', '');
        if (tokenAddr && tokenAddr.length > 10) {
          candidates.add(tokenAddr);
        }
      }
    }
    
    results.sources.push({
      name: 'geckoterminal-new',
      pools: newPools.data?.length || 0,
      status: 'success'
    });
  } catch (error) {
    results.sources.push({
      name: 'geckoterminal-new',
      pools: 0,
      status: 'error',
      error: error.message
    });
  }
  
  results.summary.totalScanned = candidates.size;
  
  // === Score all candidates ===
  const opportunities = [];
  let lookupCount = 0;
  
  for (const mint of candidates) {
    if (lookupCount > 0 && lookupCount % 5 === 0) {
      await sleep(1000); // Rate limiting
    }
    lookupCount++;
    
    const info = await getTokenInfo(mint);
    if (!info) continue;
    
    // Filter minimum requirements
    if (info.liquidity < 10000) continue;
    if (info.volume24h < 5000 && info.liquidity < 50000) continue;
    
    const scoring = calculateScore(info);
    
    opportunities.push({
      ...info,
      score: scoring.score,
      scoreDetails: scoring.details,
    });
    
    results.summary.validTokens++;
  }
  
  // Sort by score and limit results
  opportunities.sort((a, b) => b.score - a.score);
  results.opportunities = opportunities.slice(0, MAX_RESULTS);
  results.summary.topScore = opportunities[0]?.score || 0;
  
  results.scanDuration = Date.now() - startTime;
  
  return results;
}

// Main execution
async function main() {
  try {
    console.error(`🔍 Scanning for Solana trading opportunities (max: ${MAX_RESULTS})...`);
    const results = await scanOpportunities();
    console.error(`✅ Scan complete: ${results.summary.validTokens} tokens, top score: ${results.summary.topScore}`);
    console.log(JSON.stringify(results, null, 2));
  } catch (error) {
    console.error('❌ Scan failed:', error.message);
    process.exit(1);
  }
}

if (require.main === module) {
  main();
}

module.exports = { scanOpportunities, calculateScore };