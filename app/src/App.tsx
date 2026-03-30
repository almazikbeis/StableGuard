import { motion } from 'framer-motion'
import TopBar from './components/TopBar'
import Sidebar from './components/Sidebar'
import StatusBanner from './components/StatusBanner'
import RiskGauge from './components/RiskGauge'
import PortfolioCard from './components/PortfolioCard'
import AllocationBar from './components/AllocationBar'
import DecisionFeed from './components/DecisionFeed'
import NetworkViz from './components/NetworkViz'
import MobileNav from './components/MobileNav'

export default function App() {
  return (
    <div className="min-h-screen bg-surface text-on-surface">
      <TopBar />
      <Sidebar />

      <main className="pt-20 pb-24 md:ml-64 md:px-8 px-4 min-h-screen">
        <StatusBanner />

        {/* Row 1: Risk Gauge + Financials */}
        <div className="flex flex-col lg:flex-row gap-6 mb-6">
          <div className="lg:w-[360px] shrink-0">
            <RiskGauge />
          </div>
          <div className="flex-1 flex flex-col gap-6 min-w-0">
            <PortfolioCard />
            <AllocationBar />
          </div>
        </div>

        {/* Row 2: Decision Feed + Network Viz */}
        <div className="flex flex-col lg:flex-row gap-6">
          <div className="lg:w-[320px] shrink-0">
            <DecisionFeed />
          </div>
          <div className="flex-1 min-w-0">
            <NetworkViz />
          </div>
        </div>
      </main>

      {/* Footer */}
      <footer className="md:ml-64 md:px-8 px-4 pb-28 text-center md:text-right">
        <motion.button
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 2.5 }}
          whileHover={{ color: 'rgba(255, 180, 171, 0.6)' }}
          className="text-[9px] font-headline uppercase tracking-widest text-on-surface-variant/40 transition-colors flex items-center justify-center md:justify-end gap-2 ml-auto cursor-pointer"
        >
          <span className="material-symbols-outlined text-[12px]">warning</span>
          Simulate Depeg Event (DEMO ONLY)
        </motion.button>
      </footer>

      <MobileNav />
    </div>
  )
}
