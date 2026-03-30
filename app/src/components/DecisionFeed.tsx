import { motion } from 'framer-motion'

const decisions = [
  {
    time: '09:42 UTC',
    text: 'Rebalancing 5% USDC to USDT to optimize yield based on liquidity variance.',
    isLatest: true,
  },
  {
    time: '09:30 UTC',
    text: 'Risk scan complete. No anomalies detected on Solana Mainnet. All protocols operating normally.',
    isLatest: false,
  },
  {
    time: '08:15 UTC',
    text: 'Sentinels deployed across Jupiter Aggregator routes. Transaction paths cleared for zero-slippage execution.',
    isLatest: false,
  },
]

export default function DecisionFeed() {
  return (
    <motion.div
      initial={{ opacity: 0, x: -30 }}
      animate={{ opacity: 1, x: 0 }}
      transition={{ duration: 0.7, delay: 0.8, ease: [0.22, 1, 0.36, 1] }}
      className="h-full"
    >
      <div className="glass-panel rounded-2xl p-6 h-full border-t-2 border-primary-container/20 relative overflow-hidden">
        {/* Scanner line */}
        <div className="absolute inset-0 overflow-hidden pointer-events-none">
          <motion.div
            className="absolute left-0 right-0 h-px bg-gradient-to-r from-transparent via-primary-container/40 to-transparent"
            animate={{ top: ['-1px', '100%'] }}
            transition={{ duration: 5, repeat: Infinity, ease: 'linear', repeatDelay: 3 }}
          />
        </div>

        <div className="flex items-center gap-3 mb-8">
          <motion.span
            className="material-symbols-outlined text-primary-container"
            style={{ fontVariationSettings: "'FILL' 1" }}
            animate={{ rotate: [0, 5, -5, 0] }}
            transition={{ duration: 4, repeat: Infinity }}
          >
            terminal
          </motion.span>
          <h3 className="font-headline font-bold text-sm tracking-widest uppercase">AI Decision Feed</h3>
        </div>

        <div className="space-y-6">
          {decisions.map((decision, i) => (
            <motion.div
              key={i}
              initial={{ opacity: 0, x: -20 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ delay: 1.2 + i * 0.2, ease: [0.22, 1, 0.36, 1] }}
              className="flex gap-3 items-start"
            >
              <motion.div
                className={`mt-1.5 w-1.5 h-1.5 rounded-full shrink-0 ${
                  decision.isLatest ? 'bg-primary-container animate-pulse-dot' : 'bg-primary-container/40'
                }`}
              />
              <div>
                <p className="text-[10px] font-mono text-on-surface-variant/60 mb-1">{decision.time}</p>
                <p
                  className={`text-xs leading-relaxed p-3 rounded-lg ${
                    decision.isLatest
                      ? 'text-primary/90 bg-surface-container-low border-l-2 border-primary-container'
                      : 'text-on-surface-variant/80 bg-surface-container-low'
                  }`}
                >
                  {decision.text}
                </p>
              </div>
            </motion.div>
          ))}
        </div>

        <motion.button
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 2 }}
          whileHover={{ backgroundColor: 'rgba(49, 53, 60, 0.6)' }}
          whileTap={{ scale: 0.98 }}
          className="w-full mt-8 py-3 bg-surface-container-highest/30 text-on-surface-variant text-[10px] font-headline uppercase tracking-widest rounded-lg transition-colors cursor-pointer"
        >
          View Full Logs
        </motion.button>
      </div>
    </motion.div>
  )
}
