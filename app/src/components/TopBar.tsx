import { motion } from 'framer-motion'
import { useState } from 'react'

const navItems = ['Sentinel Hub', 'Risk Matrix', 'Asset Flow', 'Protocol Simulation']

export default function TopBar() {
  const [active, setActive] = useState(0)

  return (
    <motion.header
      initial={{ y: -64, opacity: 0 }}
      animate={{ y: 0, opacity: 1 }}
      transition={{ duration: 0.6, ease: [0.22, 1, 0.36, 1] }}
      className="fixed top-0 w-full z-50 flex justify-between items-center px-8 h-16 bg-surface/90 backdrop-blur-xl"
    >
      <div className="flex items-center gap-4">
        <span className="text-2xl font-bold tracking-tighter text-primary-container font-headline">
          StableGuard
        </span>
      </div>

      <nav className="hidden md:flex gap-8 items-center h-full">
        {navItems.map((item, i) => (
          <button
            key={item}
            onClick={() => setActive(i)}
            className={`relative h-full flex items-center px-1 font-headline tracking-wider text-xs uppercase transition-colors duration-300 ${
              i === active ? 'text-primary-container' : 'text-on-surface/40 hover:text-on-surface/70'
            }`}
          >
            {item}
            {i === active && (
              <motion.div
                layoutId="nav-indicator"
                className="absolute bottom-0 left-0 right-0 h-0.5 bg-primary-container"
                style={{ boxShadow: '0 0 10px #00ffa3, 0 0 20px #00ffa3' }}
                transition={{ type: 'spring', stiffness: 400, damping: 30 }}
              />
            )}
          </button>
        ))}
      </nav>

      <div className="flex items-center gap-5">
        {['sensors', 'account_balance_wallet', 'smart_toy'].map((icon, i) => (
          <motion.span
            key={icon}
            initial={{ scale: 0, rotate: -180 }}
            animate={{ scale: 1, rotate: 0 }}
            transition={{ delay: 0.3 + i * 0.1, type: 'spring', stiffness: 260 }}
            className="material-symbols-outlined text-primary-container cursor-pointer hover:scale-110 transition-transform text-xl"
          >
            {icon}
          </motion.span>
        ))}
      </div>
    </motion.header>
  )
}
