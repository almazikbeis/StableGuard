import { motion } from 'framer-motion'

export default function AllocationBar() {
  return (
    <motion.div
      initial={{ opacity: 0, y: 30 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.7, delay: 0.7, ease: [0.22, 1, 0.36, 1] }}
      className="glass-panel rounded-2xl p-8"
    >
      <div className="flex justify-between items-center mb-6">
        <p className="font-headline text-xs tracking-[0.2em] text-on-surface-variant uppercase">
          Asset Allocation
        </p>
        <motion.span
          initial={{ opacity: 0, scale: 0.8 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ delay: 1.5 }}
          className="text-[10px] text-primary-container font-bold px-2 py-0.5 border border-primary-container/20 rounded"
          style={{ boxShadow: '0 0 10px rgba(0, 255, 163, 0.1)' }}
        >
          OPTIMIZED
        </motion.span>
      </div>

      <div className="w-full h-12 bg-surface-container-highest rounded-full flex overflow-hidden p-1">
        <motion.div
          initial={{ width: 0 }}
          animate={{ width: '60%' }}
          transition={{ duration: 1.2, delay: 0.9, ease: [0.22, 1, 0.36, 1] }}
          className="h-full bg-primary-container rounded-l-full relative"
        >
          <div className="absolute inset-0 flex items-center px-4">
            <motion.span
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ delay: 1.6 }}
              className="text-[10px] font-black text-on-primary"
            >
              USDC 60%
            </motion.span>
          </div>
        </motion.div>

        <motion.div
          initial={{ width: 0 }}
          animate={{ width: '40%' }}
          transition={{ duration: 1, delay: 1.1, ease: [0.22, 1, 0.36, 1] }}
          className="h-full bg-secondary-container relative border-l border-surface"
        >
          <div className="absolute inset-0 flex items-center px-4">
            <motion.span
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ delay: 1.8 }}
              className="text-[10px] font-black text-on-secondary"
            >
              USDT 40%
            </motion.span>
          </div>
        </motion.div>
      </div>

      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ delay: 2 }}
        className="flex gap-8 mt-6"
      >
        <div className="flex items-center gap-2">
          <div className="w-2 h-2 rounded-full bg-primary-container" />
          <span className="text-xs text-on-surface-variant">USDC (Safe)</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-2 h-2 rounded-full bg-secondary-container" />
          <span className="text-xs text-on-surface-variant">USDT (Safe)</span>
        </div>
      </motion.div>
    </motion.div>
  )
}
