import { motion } from 'framer-motion'
import { useState } from 'react'

const tabs = [
  { icon: 'grid_view', label: 'Dashboard' },
  { icon: 'insights', label: 'Analytics' },
  { icon: 'psychology', label: 'Sentinel' },
  { icon: 'lock', label: 'Vault' },
]

export default function MobileNav() {
  const [active, setActive] = useState(0)

  return (
    <motion.nav
      initial={{ y: 80 }}
      animate={{ y: 0 }}
      transition={{ duration: 0.5, delay: 0.5, ease: [0.22, 1, 0.36, 1] }}
      className="fixed bottom-0 left-0 w-full z-50 flex justify-around items-center h-20 md:hidden px-4 bg-surface/80 backdrop-blur-lg border-t border-primary-container/15"
    >
      {tabs.map((tab, i) => (
        <button
          key={tab.label}
          onClick={() => setActive(i)}
          className={`flex flex-col items-center justify-center transition-all duration-300 ${
            i === active ? 'text-primary-container scale-110' : 'text-on-surface/40'
          }`}
        >
          <span className="material-symbols-outlined">{tab.icon}</span>
          <span className="font-body text-[10px] font-bold">{tab.label}</span>
          {i === active && (
            <motion.div
              layoutId="mobile-indicator"
              className="w-1 h-1 rounded-full bg-primary-container mt-0.5"
              style={{ boxShadow: '0 0 6px #00ffa3' }}
              transition={{ type: 'spring', stiffness: 400, damping: 30 }}
            />
          )}
        </button>
      ))}
    </motion.nav>
  )
}
