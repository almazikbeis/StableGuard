import { motion } from 'framer-motion'

export default function StatusBanner() {
  return (
    <motion.div
      initial={{ opacity: 0, y: 20, scale: 0.98 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      transition={{ duration: 0.6, delay: 0.3 }}
      className="mb-8 w-full glass-panel rounded-xl p-4 flex items-center justify-between border-l-4 border-primary-container overflow-hidden relative"
    >
      {/* Scanner animation */}
      <div className="absolute inset-0 overflow-hidden pointer-events-none">
        <motion.div
          className="absolute w-full h-8 bg-gradient-to-b from-transparent via-primary-container/10 to-transparent"
          animate={{ top: ['-2rem', '100%'] }}
          transition={{ duration: 3, repeat: Infinity, ease: 'linear', repeatDelay: 2 }}
        />
      </div>

      {/* Pulse BG */}
      <motion.div
        className="absolute inset-0 bg-primary-container/5"
        animate={{ opacity: [0.03, 0.08, 0.03] }}
        transition={{ duration: 3, repeat: Infinity }}
      />

      <div className="flex items-center gap-4 relative z-10">
        <div className="w-2 h-2 rounded-full bg-primary-container animate-pulse-dot" />
        <h2 className="font-headline font-bold text-sm tracking-[0.2em] text-primary-container uppercase">
          AI Sentinel Status: FULL PROTECTION ACTIVE
        </h2>
      </div>

      <div className="hidden sm:flex items-center gap-2 relative z-10">
        <motion.span
          className="text-[10px] font-mono text-primary-container/60"
          animate={{ opacity: [0.6, 1, 0.6] }}
          transition={{ duration: 2, repeat: Infinity }}
        >
          LATENCY: 12ms
        </motion.span>
        <span className="text-[10px] font-mono text-primary-container/60 px-2 border-l border-primary-container/20">
          UPTIME: 99.99%
        </span>
      </div>
    </motion.div>
  )
}
