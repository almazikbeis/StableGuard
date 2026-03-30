import { motion } from 'framer-motion'
import { useEffect, useRef, useState } from 'react'

interface Particle {
  x: number
  y: number
  vx: number
  vy: number
  size: number
  alpha: number
}

function ParticleCanvas() {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const resize = () => {
      canvas.width = canvas.offsetWidth * 2
      canvas.height = canvas.offsetHeight * 2
      ctx.scale(2, 2)
    }
    resize()
    window.addEventListener('resize', resize)

    const particles: Particle[] = Array.from({ length: 60 }, () => ({
      x: Math.random() * canvas.offsetWidth,
      y: Math.random() * canvas.offsetHeight,
      vx: (Math.random() - 0.5) * 0.4,
      vy: (Math.random() - 0.5) * 0.4,
      size: Math.random() * 2 + 1,
      alpha: Math.random() * 0.5 + 0.2,
    }))

    let animId: number
    const draw = () => {
      const w = canvas.offsetWidth
      const h = canvas.offsetHeight
      ctx.clearRect(0, 0, w, h)

      particles.forEach((p) => {
        p.x += p.vx
        p.y += p.vy
        if (p.x < 0 || p.x > w) p.vx *= -1
        if (p.y < 0 || p.y > h) p.vy *= -1
      })

      // Draw connections
      for (let i = 0; i < particles.length; i++) {
        for (let j = i + 1; j < particles.length; j++) {
          const dx = particles[i].x - particles[j].x
          const dy = particles[i].y - particles[j].y
          const dist = Math.sqrt(dx * dx + dy * dy)
          if (dist < 120) {
            ctx.beginPath()
            ctx.strokeStyle = `rgba(0, 255, 163, ${0.15 * (1 - dist / 120)})`
            ctx.lineWidth = 0.5
            ctx.moveTo(particles[i].x, particles[i].y)
            ctx.lineTo(particles[j].x, particles[j].y)
            ctx.stroke()
          }
        }
      }

      // Draw particles
      particles.forEach((p) => {
        ctx.beginPath()
        ctx.fillStyle = `rgba(0, 255, 163, ${p.alpha})`
        ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2)
        ctx.fill()

        // Glow
        ctx.beginPath()
        const gradient = ctx.createRadialGradient(p.x, p.y, 0, p.x, p.y, p.size * 4)
        gradient.addColorStop(0, `rgba(0, 255, 163, ${p.alpha * 0.3})`)
        gradient.addColorStop(1, 'rgba(0, 255, 163, 0)')
        ctx.fillStyle = gradient
        ctx.arc(p.x, p.y, p.size * 4, 0, Math.PI * 2)
        ctx.fill()
      })

      animId = requestAnimationFrame(draw)
    }
    draw()

    return () => {
      cancelAnimationFrame(animId)
      window.removeEventListener('resize', resize)
    }
  }, [])

  return <canvas ref={canvasRef} className="absolute inset-0 w-full h-full" />
}

const features = [
  { icon: 'security_update_good', title: 'Multi-Sig Guard', desc: '4/5 Active Sentinels' },
  { icon: 'stream', title: 'Liquidity Stream', desc: 'Stable across 14 DEXs' },
  { icon: 'network_intelligence', title: 'AI Prediction', desc: '0.001% Volatility forecast' },
]

export default function NetworkViz() {
  return (
    <motion.div
      initial={{ opacity: 0, y: 30 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.8, delay: 1, ease: [0.22, 1, 0.36, 1] }}
      className="glass-panel rounded-2xl p-8 min-h-[400px] h-full flex flex-col justify-end relative overflow-hidden"
    >
      {/* Particle network background */}
      <ParticleCanvas />

      {/* Gradient overlay at bottom for readability */}
      <div className="absolute bottom-0 left-0 right-0 h-48 bg-gradient-to-t from-surface-container/90 to-transparent pointer-events-none" />

      {/* Action buttons */}
      <div className="relative z-10 flex items-center justify-center gap-4 mb-10">
        <motion.button
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 1.5 }}
          whileHover={{ scale: 1.05, boxShadow: '0 10px 40px rgba(0, 255, 163, 0.35)' }}
          whileTap={{ scale: 0.95 }}
          className="px-12 py-4 bg-primary-container text-on-primary-container font-headline font-bold uppercase tracking-widest text-xs rounded-full cursor-pointer"
          style={{ boxShadow: '0 10px 30px rgba(0, 255, 163, 0.3)' }}
        >
          Deposit
        </motion.button>
        <motion.button
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 1.6 }}
          whileHover={{ scale: 1.05, backgroundColor: '#353940' }}
          whileTap={{ scale: 0.95 }}
          className="px-12 py-4 bg-surface-container-highest text-on-surface font-headline font-bold uppercase tracking-widest text-xs rounded-full border border-outline-variant/30 cursor-pointer transition-colors"
        >
          Withdraw
        </motion.button>
      </div>

      {/* Feature badges */}
      <div className="relative z-10 grid grid-cols-1 md:grid-cols-3 gap-8">
        {features.map((f, i) => (
          <motion.div
            key={f.title}
            initial={{ opacity: 0, y: 15 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 1.8 + i * 0.15 }}
            whileHover={{ x: 4 }}
            className="flex items-center gap-4"
          >
            <motion.span
              className="material-symbols-outlined text-primary-container text-3xl"
              animate={{ rotate: [0, 3, -3, 0] }}
              transition={{ duration: 6, repeat: Infinity, delay: i * 0.5 }}
            >
              {f.icon}
            </motion.span>
            <div>
              <h4 className="font-headline text-xs font-bold uppercase">{f.title}</h4>
              <p className="text-[10px] text-on-surface-variant">{f.desc}</p>
            </div>
          </motion.div>
        ))}
      </div>
    </motion.div>
  )
}
