# Solana Trading Strategy Guide

## Overview

This strategy focuses on **momentum-based token trading** on Solana, designed for autonomous execution with minimal manual intervention. The approach combines multiple data sources to identify trending tokens early and uses systematic entry/exit rules to manage risk.

## Core Strategy: Early Momentum Detection

### Target Assets
- **Micro/small cap tokens** (FDV < $2M preferred)
- **Recently launched** or newly trending tokens
- **High-liquidity pairs** on major DEXs (Raydium, Orca, Jupiter)
- **Boosted/promoted** tokens (attention = volume)

### Data Sources

#### 1. DexScreener Boosted Tokens
- **Why**: Paid promotion indicates serious backing/marketing budget
- **Signal**: Attention drives volume, volume drives price discovery
- **Risk**: Can be pump-and-dump schemes, requires quick exits

#### 2. GeckoTerminal Trending Pools
- **Why**: Captures tokens gaining organic momentum
- **Signal**: Rising trading activity before mainstream discovery
- **Risk**: May catch momentum late in cycle

#### 3. GeckoTerminal New Pools
- **Why**: Early access to newly launched tokens
- **Signal**: Fresh tokens with good initial liquidity
- **Risk**: High failure rate, many rugs/failed launches

## Scoring Algorithm

### Positive Factors (Entry Signals)

**Early Stage Bonus (30 pts max)**
```
FDV < $500k:  +30 pts  (micro cap, highest upside)
FDV < $2M:    +20 pts  (small cap, good upside)
FDV < $10M:   +10 pts  (mid cap, moderate upside)
```

**Momentum Score (45 pts max)**
```
5-minute rise (0-15%):  +3 pts per %
1-hour rise (0-30%):    +2 pts per %
Acceleration bonus:     +15 pts (if 1h > 5% AND 5m > 0%)
```

**Volume Hype (35 pts max)**
```
Volume/Liquidity > 2:   +20 pts
Volume/Liquidity > 5:   +15 pts additional
```

**Buy Pressure (25 pts max)**
```
Buy/Sell ratio > 1.3:   +15 pts
Buy/Sell ratio > 2.0:   +10 pts additional
```

### Negative Factors (Risk Penalties)

**Overextension Penalties**
```
5m price > +30%:    -25 pts  (already pumped, rug risk)
1h price < -15%:    -20 pts  (momentum broken, selling)
24h price < -40%:   -20 pts  (dead token, avoid)
```

**Liquidity/Market Penalties**
```
Liquidity < $15k:       -10 pts  (too thin, slippage risk)
Buy/Sell ratio < 0.5:   -15 pts  (sell pressure dominates)
```

### Score Thresholds
- **Entry threshold**: 25+ points
- **Premium targets**: 40+ points
- **Avoid**: < 15 points

## Entry Rules

### Position Sizing
- **Base size**: $5-10 per position
- **Adaptive sizing**: 20-30% of available USDC
- **Maximum single position**: $15
- **Portfolio limit**: 4 concurrent positions

### Entry Conditions
1. **Score ≥ 25 points**
2. **Minimum liquidity**: $10k
3. **Portfolio slots available**: < 4 positions
4. **Available capital**: ≥ $5 USDC

### Timing
- **Scan frequency**: Every 15 minutes
- **Entry execution**: Immediate when conditions met
- **Slippage tolerance**: 3-5% (handled by Jupiter)

## Exit Strategy

### Take Profit Levels
1. **Primary target**: +50% gain (lock in winner)
2. **Early exit**: +30% if momentum weakening

### Stop Loss Levels
1. **Hard stop**: -25% loss (cut losers quickly)
2. **Momentum stop**: 5m <-8% AND 1h <-15% (technical breakdown)

### Trailing Stop
- **Activation**: After +30% unrealized gain
- **Trail distance**: 15% from peak PnL
- **Purpose**: Let winners run, but protect gains

### Position Monitoring
- **Check frequency**: Every 15 minutes
- **Exit execution**: Immediate when triggers hit
- **State persistence**: Track highest PnL for trailing stops

## Risk Management

### Portfolio Level
- **Max positions**: 4 concurrent trades
- **Reserve capital**: Always keep $5+ liquid for exits/gas
- **Correlation limit**: Avoid similar tokens in same sector
- **Exposure limit**: Never risk more than starting portfolio value

### Trade Level
- **Position sizing**: Risk 2-8% of portfolio per trade
- **Time limit**: No position held > 48 hours (momentum trades)
- **Profit taking**: Scale out at resistance levels
- **Loss cutting**: Quick exits on breakdown

### Market Conditions
- **Bull market**: Increase position sizes, longer holds
- **Bear market**: Smaller sizes, quicker exits
- **High volatility**: Tighter stops, faster execution
- **Low volume**: Avoid trading, wait for setups

## SOL Swing Component

### SOL Accumulation Strategy
```
Daily SOL drop > -7%:     Buy dip (up to $15)
Daily SOL pump > +6%:     Take profit (up to 30% of holdings)
Base position:            Maintain 0.2+ SOL minimum
```

### Rationale
- SOL provides base yield and stability
- Swing trades capture major moves
- Maintains exposure to Solana ecosystem growth
- Provides hedge against alt-token volatility

## Performance Tracking

### Key Metrics
- **Win rate**: Target 40-60% (momentum trading is volatile)
- **Average winner**: Target 20-50% gains
- **Average loser**: Keep under -15% losses
- **Profit factor**: Win $ / Loss $ ratio > 1.5

### Strategy Stats
```json
{
  "momentum": {
    "wins": 12,
    "losses": 8, 
    "totalPnl": 45.67,
    "winRate": 60,
    "avgWin": 15.2,
    "avgLoss": -8.3
  }
}
```

### Optimization Triggers
- **Win rate < 35%**: Lower entry threshold
- **Avg loss > -20%**: Tighter stops
- **Profit factor < 1.2**: Review exit timing
- **Max drawdown > -30%**: Reduce position sizes

## Market Cycle Adaptations

### Bull Market (Risk On)
- Increase position sizes to $15-20
- Extend holding periods to 24-48h
- Raise entry threshold to 30+ points
- Target +75% take profits

### Bear Market (Risk Off)
- Reduce position sizes to $3-8
- Faster exits (6-12h max hold)
- Lower entry threshold to 20+ points
- Quick +25% profit taking

### Choppy/Sideways
- Minimal trading, wait for clear trends
- Focus on SOL swing trades only
- Tight risk controls
- Preserve capital for next cycle

## Common Pitfalls & Solutions

### Pitfall: FOMO Chasing
**Solution**: Stick to systematic scoring, no manual overrides

### Pitfall: Holding Losers Too Long
**Solution**: Automated stop losses, no emotional attachment

### Pitfall: Taking Profits Too Early
**Solution**: Trailing stops let winners run while protecting gains

### Pitfall: Overtrading in Low-Quality Setups
**Solution**: Maintain minimum score requirements, quality over quantity

### Pitfall: Ignoring Market Context
**Solution**: Adjust parameters based on overall market conditions

## Technology Stack

### Execution
- **Jupiter Aggregator**: Best price execution, minimal slippage
- **Solana RPC**: Fast transaction confirmation
- **Rate Limiting**: Respect API limits to avoid bans

### Data Sources
- **DexScreener**: Token metadata, price changes, volume
- **GeckoTerminal**: Pool discovery, trending analysis
- **CoinGecko**: SOL price for swing trading

### Risk Controls
- **Position Tracking**: JSON state files for persistence
- **Trade Logging**: Complete audit trail
- **Error Handling**: Graceful degradation on API failures

This strategy balances aggressive opportunity seeking with systematic risk management, designed for consistent profitability in the volatile Solana token ecosystem.