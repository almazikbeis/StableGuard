import { motion, animate } from 'framer-motion'
import { useEffect, useState } from 'react'

export default function RiskGauge() {
  const riskValue = 0.02
  const circumference = 2 * Math.PI * 100
  const offset = circumference - (riskValue / 100) * circumference

  const [displayText, setDisplayText] = useState('0.00%')

  useEffect(() => {
    const controls = animate(0, riskValue, {
      duration: 2,
      ease: [0.22, 1, 0.36, 1],
      onUpdate: (v) => setDisplayText(v.toFixed(2) + '%'),
    })
    return () => controls.stop()
  }, [])

  return (
    <motion.div
      initial={{ opacity: 0, scale: 0.9 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ duration: 0.8, delay: 0.4, ease: [0.22, 1, 0.36, 1] }}
      className="glass-panel rounded-2xl p-6 flex flex-col items-center justify-center relative h-full min-h-[340px] overflow-hidden"
    >
      {/* Grid overlay */}
      <div className="absolute inset-0 grid-bg opacity-30" />

      {/* Shield icon */}
      <motion.div
        className="absolute top-4 right-4"
        animate={{ y: [0, -4, 0] }}
        transition={{ duration: 4, repeat: Infinity, ease: 'easeInOut' }}
      >
        <span className="material-symbols-outlined text-primary-container/30 text-4xl">shield_with_heart</span>
      </motion.div>

      {/* Circular Gauge */}
      <div className="relative w-44 h-44 flex items-center justify-center">
        <svg className="w-full h-full -rotate-90" viewBox="0 0 220 220">
          {/* Track */}
          <circle
            cx="110" cy="110" r="100"
            fill="transparent"
            stroke="#31353c"
            strokeWidth="7"
          />
          {/* Value ring */}
          <motion.circle
            cx="110" cy="110" r="100"
            fill="transparent"
            stroke="#00ffa3"
            strokeWidth="7"
            strokeDasharray={circumference}
            initial={{ strokeDashoffset: circumference }}
            animate={{ strokeDashoffset: offset }}
            transition={{ duration: 2.5, ease: [0.22, 1, 0.36, 1], delay: 0.6 }}
            style={{
              filter: 'drop-shadow(0 0 8px rgba(0, 255, 163, 0.5)) drop-shadow(0 0 16px rgba(0, 255, 163, 0.2))',
            }}
            strokeLinecap="round"
          />
          {/* Particle dot at top */}
          <motion.circle
            cx="110" cy="10" r="3.5"
            fill="#52ffac"
            initial={{ opacity: 0 }}
            animate={{ opacity: [0, 1, 0.5] }}
            transition={{ duration: 2.5, delay: 0.6 }}
            style={{ filter: 'blur(1px) drop-shadow(0 0 6px #00ffa3)' }}
          />
        </svg>

        {/* Center text */}
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <motion.span
            className="font-headline text-4xl font-light text-primary"
            initial={{ opacity: 0, scale: 0.5 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ duration: 0.8, delay: 1 }}
            style={{ textShadow: '0 0 30px rgba(0, 255, 163, 0.2)' }}
          >
            {displayText}
          </motion.span>
          <motion.span
            className="font-headline text-[10px] tracking-widest text-primary-container mt-1 uppercase"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ delay: 1.5 }}
          >
            Risk Index
          </motion.span>
        </div>
      </div>

      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ delay: 2 }}
        className="mt-6 text-center"
      >
        <p className="text-on-surface-variant text-sm font-medium">
          System Status: <span className="text-primary-container" style={{ textShadow: '0 0 12px rgba(0,255,163,0.3)' }}>Stable & Neutralized</span>
        </p>
      </motion.div>
    </motion.div>
  )
}
