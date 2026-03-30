import { motion, animate } from 'framer-motion'
import { useEffect, useState } from 'react'

const stats = [
  { label: 'Yield APY', value: '8.42%' },
  { label: 'Peg Stability', value: '1.0001' },
  { label: 'Net Gain', value: '+$842.10' },
  { label: 'Gas Saved', value: '0.45 SOL' },
]

export default function PortfolioCard() {
  const [balanceText, setBalanceText] = useState('$0.00')

  useEffect(() => {
    const controls = animate(0, 124502.80, {
      duration: 2,
      ease: [0.22, 1, 0.36, 1],
      onUpdate: (v) =>
        setBalanceText('$' + v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })),
    })
    return () => controls.stop()
  }, [])

  return (
    <motion.div
      initial={{ opacity: 0, y: 30 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.7, delay: 0.5, ease: [0.22, 1, 0.36, 1] }}
      className="glass-panel rounded-2xl p-6 relative group"
    >
      {/* Gradient overlay */}
      <div className="absolute inset-0 bg-gradient-to-br from-primary-container/10 to-transparent opacity-30" />

      <div className="relative z-10 flex flex-col sm:flex-row justify-between items-start sm:items-end">
        <div>
          <p className="font-headline text-[10px] tracking-[0.3em] text-on-surface-variant uppercase mb-3">
            Secured Portfolio Balance
          </p>
          <motion.h1
            className="font-headline text-4xl lg:text-5xl font-bold text-primary tracking-tighter"
            style={{ textShadow: '0 0 40px rgba(0, 255, 163, 0.1)' }}
          >
            {balanceText}
          </motion.h1>
        </div>

        <motion.div
          initial={{ opacity: 0, x: 20 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ delay: 1.2 }}
          className="mt-6 sm:mt-0 flex items-center gap-2 px-3 py-1 bg-primary-container/10 rounded-full border border-primary-container/20"
        >
          <span className="material-symbols-outlined text-primary-container text-sm">trending_up</span>
          <span className="text-xs font-bold text-primary-container">+1.2% (24h)</span>
        </motion.div>
      </div>

      <div className="mt-8 grid grid-cols-2 md:grid-cols-4 gap-4 relative z-10">
        {stats.map((stat, i) => (
          <motion.div
            key={stat.label}
            initial={{ opacity: 0, y: 15 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.8 + i * 0.1, ease: [0.22, 1, 0.36, 1] }}
            whileHover={{ scale: 1.05, backgroundColor: 'rgba(10, 14, 20, 0.7)' }}
            className="bg-surface-container-lowest/50 p-4 rounded-xl transition-colors"
          >
            <p className="text-[10px] text-on-surface-variant uppercase">{stat.label}</p>
            <p className="text-lg font-headline font-bold text-primary">{stat.value}</p>
          </motion.div>
        ))}
      </div>
    </motion.div>
  )
}
