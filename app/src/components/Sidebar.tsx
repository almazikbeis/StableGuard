import { motion } from 'framer-motion'
import { useState } from 'react'

const menuItems = [
  { icon: 'radar', label: 'Sentinel Hub' },
  { icon: 'gpp_maybe', label: 'Risk Matrix' },
  { icon: 'swap_vert', label: 'Asset Flow' },
  { icon: 'biotech', label: 'Protocol Simulation' },
]

export default function Sidebar() {
  const [active, setActive] = useState(0)
  const [hovered, setHovered] = useState<number | null>(null)

  return (
    <motion.aside
      initial={{ x: -264, opacity: 0 }}
      animate={{ x: 0, opacity: 1 }}
      transition={{ duration: 0.7, ease: [0.22, 1, 0.36, 1], delay: 0.2 }}
      className="fixed left-0 top-16 h-[calc(100vh-64px)] z-40 hidden md:flex flex-col w-64 bg-surface-container-low/40 backdrop-blur-xl border-r border-primary-container/10"
    >
      <div className="p-8 flex items-center gap-4 border-b border-outline-variant/10">
        <motion.div
          animate={{ rotate: [0, 360] }}
          transition={{ duration: 20, repeat: Infinity, ease: 'linear' }}
          className="w-10 h-10 rounded-full bg-primary-container/20 flex items-center justify-center border border-primary-container/30"
        >
          <span className="material-symbols-outlined text-primary-container" style={{ fontVariationSettings: "'FILL' 1" }}>
            psychology
          </span>
        </motion.div>
        <div>
          <p className="text-primary-container font-black font-headline text-sm tracking-widest">SENTINEL-01</p>
          <div className="flex items-center gap-1.5">
            <motion.div
              className="w-1.5 h-1.5 rounded-full bg-primary-container"
              animate={{ opacity: [1, 0.3, 1] }}
              transition={{ duration: 1.5, repeat: Infinity }}
            />
            <p className="text-[10px] uppercase tracking-tighter text-on-surface/40">AI Status: Active</p>
          </div>
        </div>
      </div>

      <nav className="flex-1 py-8 flex flex-col relative">
        {menuItems.map((item, i) => (
          <motion.button
            key={item.label}
            onClick={() => setActive(i)}
            onHoverStart={() => setHovered(i)}
            onHoverEnd={() => setHovered(null)}
            initial={{ x: -40, opacity: 0 }}
            animate={{ x: 0, opacity: 1 }}
            transition={{ delay: 0.4 + i * 0.08, ease: [0.22, 1, 0.36, 1] }}
            className={`relative px-8 py-4 flex items-center gap-4 transition-all duration-300 text-left ${
              i === active
                ? 'text-primary-container'
                : 'text-on-surface/40 hover:text-primary-container'
            }`}
          >
            {i === active && (
              <motion.div
                layoutId="sidebar-active"
                className="absolute inset-0 bg-primary-container/10 border-r-4 border-primary-container"
                transition={{ type: 'spring', stiffness: 400, damping: 30 }}
              />
            )}
            {hovered === i && i !== active && (
              <motion.div
                layoutId="sidebar-hover"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                className="absolute inset-0 bg-surface-container"
              />
            )}
            <span className="material-symbols-outlined text-lg relative z-10">{item.icon}</span>
            <span className="font-headline uppercase tracking-widest text-xs relative z-10">{item.label}</span>
          </motion.button>
        ))}
      </nav>

      <div className="p-8 mt-auto">
        <motion.button
          whileHover={{ scale: 1.02, boxShadow: '0 0 20px rgba(147, 0, 10, 0.3)' }}
          whileTap={{ scale: 0.98 }}
          className="w-full py-3 bg-error-container/20 text-error border border-error/30 font-headline uppercase tracking-widest text-[10px] transition-all cursor-pointer"
        >
          Emergency Lock
        </motion.button>
      </div>
    </motion.aside>
  )
}
