"use client";

import { motion } from "framer-motion";
import Link from "next/link";
import {
  Shield,
  Zap,
  Brain,
  BarChart2,
  Bell,
  Activity,
  ArrowRight,
  ChevronRight,
  TrendingUp,
} from "lucide-react";

/* ── Data ──────────────────────────────────────────────────────────── */

const features = [
  {
    icon: Zap,
    title: "Real-time Pyth Prices",
    desc: "Sub-second price feeds for USDC, USDT, DAI, and PYUSD straight from Pyth Network oracle.",
    color: "#f59e0b",
  },
  {
    icon: Brain,
    title: "Configurable AI Autonomy",
    desc: "Choose how much authority AI gets: manual, guarded, balanced, or yield-max control modes.",
    color: "#8b5cf6",
  },
  {
    icon: Shield,
    title: "Safety-First Guardrails",
    desc: "Circuit breakers, emergency pause, and risk thresholds keep autonomy bounded by explicit policy.",
    color: "#f97316",
  },
  {
    icon: BarChart2,
    title: "Risk Engine v2",
    desc: "Windowed scoring: trend momentum + price velocity + volatility combined into a 0–100 score.",
    color: "#06b6d4",
  },
  {
    icon: Bell,
    title: "Telegram & Discord",
    desc: "Real-time alerts with configurable thresholds and smart cooldown deduplication.",
    color: "#10b981",
  },
  {
    icon: Activity,
    title: "Policy + Execution Split",
    desc: "Risk signals, AI decisions, and on-chain vault records are separated clearly for safer operation.",
    color: "#6366f1",
  },
];

const stats = [
  { value: "4",    label: "Stablecoins tracked" },
  { value: "<1s",  label: "Price latency" },
  { value: "24/7", label: "Risk monitoring" },
  { value: "4",    label: "Control modes" },
];

const pipeline = [
  { icon: Zap,       label: "Pyth Oracle",      sub: "Real-time prices",  color: "#f59e0b" },
  { icon: BarChart2, label: "Risk Engine v2",    sub: "Score + signals",   color: "#06b6d4" },
  { icon: Brain,     label: "AI Policy Layer",   sub: "Mode-aware decision", color: "#8b5cf6" },
  { icon: Shield,    label: "Vault Policy",      sub: "Record / protect",  color: "#f97316" },
];

const ticker = [
  { symbol: "USDC",  price: 1.0001, delta: +0.01 },
  { symbol: "USDT",  price: 0.9999, delta: -0.01 },
  { symbol: "DAI",   price: 1.0002, delta: +0.02 },
  { symbol: "PYUSD", price: 1.0000, delta:  0.00 },
];

/* ── Helpers ───────────────────────────────────────────────────────── */

function FeatureCard({
  icon: Icon,
  title,
  desc,
  color,
  index,
}: (typeof features)[0] & { index: number }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 24 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: "-40px" }}
      transition={{ delay: index * 0.07, type: "spring", stiffness: 100, damping: 22 }}
      whileHover={{ y: -5, boxShadow: "0 24px 48px -12px rgba(0,0,0,0.08)" }}
      className="panel-surface-soft rounded-[24px] p-6 cursor-default neon-border"
    >
      <div
        className="w-10 h-10 rounded-xl flex items-center justify-center mb-4 border border-white/10"
        style={{ background: `linear-gradient(135deg, ${color}24, rgba(255,255,255,0.04))` }}
      >
        <Icon size={18} style={{ color }} />
      </div>
      <h3 className="font-display font-semibold text-slate-50 mb-2 text-[15px]">{title}</h3>
      <p className="text-sm text-slate-300 leading-relaxed">{desc}</p>
    </motion.div>
  );
}

/* ── Page ──────────────────────────────────────────────────────────── */

export default function LandingPage() {
  return (
    <div className="min-h-screen overflow-x-hidden">

      {/* ── Background orbs ── */}
      <div className="fixed inset-0 pointer-events-none overflow-hidden" aria-hidden>
        <div
          className="absolute -top-60 right-[8%] w-[640px] h-[640px] rounded-full animate-blob"
          style={{
            background: "radial-gradient(circle, rgba(249,115,22,0.14), rgba(251,191,36,0.06))",
            filter: "blur(90px)",
          }}
        />
        <div
          className="absolute -bottom-60 -left-20 w-[560px] h-[560px] rounded-full animate-blob"
          style={{
            background: "radial-gradient(circle, rgba(99,102,241,0.09), rgba(139,92,246,0.04))",
            filter: "blur(90px)",
            animationDelay: "-11s",
          }}
        />
      </div>

      {/* ── Navbar ── */}
      <nav className="relative z-10 sticky top-0 flex items-center justify-between px-6 sm:px-12 h-16 border-b border-white/8 bg-[#08111f]/72 backdrop-blur-xl">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-xl bg-[linear-gradient(135deg,#ff7a1a,#ffb347)] flex items-center justify-center shadow-[0_0_28px_rgba(255,122,26,0.28)]">
            <Shield size={16} className="text-white" />
          </div>
          <span className="font-display font-bold text-slate-50 text-[15px] uppercase tracking-[0.08em]">StableGuard</span>
          <span className="hidden sm:inline text-xs text-slate-400 font-normal">· Configurable AI autonomy for stablecoin vaults</span>
        </div>
        <div className="flex items-center gap-2">
          <Link href="/auth/login" className="text-sm font-medium text-slate-400 hover:text-white transition-colors px-3 py-2">
            Sign in
          </Link>
          <Link
            href="/auth/register"
            className="flex items-center gap-1.5 bg-orange-500 hover:bg-orange-400 active:bg-orange-600 text-white text-sm font-semibold px-4 py-2 rounded-xl transition-colors shadow-[0_10px_30px_rgba(255,122,26,0.28)]"
          >
            Get started <ArrowRight size={14} />
          </Link>
        </div>
      </nav>

      {/* ── Hero ── */}
      <section className="relative z-10 flex flex-col items-center text-center px-6 pt-20 pb-16">
        {/* Pill badge */}
        <motion.div
          initial={{ opacity: 0, scale: 0.88 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ duration: 0.45, type: "spring" }}
          className="inline-flex items-center gap-2 glass-pill text-orange-200 text-xs font-semibold px-3.5 py-1.5 rounded-full border border-orange-300/18 mb-8"
        >
          <span className="w-1.5 h-1.5 rounded-full bg-orange-500 animate-pulse" />
          Live on Solana Devnet · Pyth Network Oracle
        </motion.div>

        {/* Headline */}
        <motion.h1
          initial={{ opacity: 0, y: 32 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1, type: "spring", stiffness: 80, damping: 20 }}
          className="font-display font-extrabold text-[52px] sm:text-[68px] lg:text-[78px] leading-[1.03] tracking-tight text-white max-w-5xl mb-6"
        >
          Stablecoin Treasury,
          <br />
          <span className="text-orange-400">Under Intelligent Command</span>
        </motion.h1>

        {/* Subtext */}
        <motion.p
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2, type: "spring", stiffness: 80, damping: 20 }}
          className="text-lg text-slate-300 max-w-2xl leading-relaxed mb-10"
        >
          Real-time Pyth prices. Explainable AI policy modes. Safety-first vault orchestration.
          <br className="hidden sm:block" />
          From manual control to emergency-only protection to active optimization.
        </motion.p>

        {/* CTA */}
        <motion.div
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.3 }}
        >
          <div className="flex flex-col sm:flex-row items-center gap-3">
            <Link
              href="/auth/register"
              className="inline-flex items-center gap-2.5 bg-orange-500 hover:bg-orange-400 active:scale-[0.98] text-white font-bold text-base px-8 py-4 rounded-2xl transition-all shadow-[0_18px_48px_rgba(255,122,26,0.26)] hover:scale-[1.02]"
            >
              <TrendingUp size={18} />
              Get started free
              <ArrowRight size={16} />
            </Link>
            <Link
              href="/dashboard"
              className="inline-flex items-center gap-1.5 text-sm text-slate-400 hover:text-white transition-colors"
            >
              View live demo →
            </Link>
          </div>
        </motion.div>

        {/* Floating mock cards */}
        <motion.div
          initial={{ opacity: 0, y: 48 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.5, type: "spring", stiffness: 70 }}
          className="mt-16 grid grid-cols-1 sm:grid-cols-3 gap-4 max-w-2xl w-full"
        >
          <div
            className="panel-surface-soft rounded-[24px] p-5 text-left animate-float"
            style={{ animationDelay: "0s" }}
          >
            <p className="text-xs text-emerald-300 font-semibold mb-2 uppercase tracking-wide">Risk Level</p>
            <p className="font-display font-extrabold text-4xl text-emerald-200 font-mono-data tabular-nums">23</p>
            <p className="text-xs text-emerald-300/80 mt-1.5 font-medium">LOW · HOLD</p>
          </div>

          <div
            className="panel-surface-soft rounded-[24px] p-5 text-left animate-float"
            style={{ animationDelay: "1.8s" }}
          >
            <p className="text-xs text-slate-400 font-semibold mb-2 uppercase tracking-wide">USDC</p>
            <p className="font-display font-extrabold text-4xl text-white font-mono-data tabular-nums">
              $1.0001
            </p>
            <p className="text-xs text-emerald-300 mt-1.5 font-medium">+0.01% vs peg</p>
          </div>

          <div
            className="panel-surface-soft rounded-[24px] p-5 text-left animate-float"
            style={{ animationDelay: "3.5s" }}
          >
            <p className="text-xs text-orange-200 font-semibold mb-2 uppercase tracking-wide">Control Mode</p>
            <p className="font-display font-extrabold text-3xl text-orange-300">GUARDED</p>
            <p className="text-xs text-orange-200/75 mt-1.5 font-medium">AI acts only in high-risk scenarios</p>
          </div>
        </motion.div>
      </section>

      {/* ── Ticker ── */}
      <div className="relative z-10 border-y border-white/8 bg-[#091523]/70 overflow-hidden py-3 select-none backdrop-blur-xl">
        <div className="flex animate-ticker whitespace-nowrap">
          {[...ticker, ...ticker, ...ticker, ...ticker].map((t, i) => (
            <span
              key={i}
              className="inline-flex items-center gap-2 px-8 text-sm font-mono-data"
            >
              <span className="font-bold text-slate-100">{t.symbol}</span>
              <span className="text-slate-300">${t.price.toFixed(4)}</span>
              <span
                className={
                  t.delta > 0
                    ? "text-green-500"
                    : t.delta < 0
                    ? "text-red-500"
                    : "text-gray-400"
                }
              >
                {t.delta > 0 ? "▲" : t.delta < 0 ? "▼" : "—"}
                {Math.abs(t.delta).toFixed(2)}%
              </span>
              <span className="text-slate-700 mx-3">·</span>
            </span>
          ))}
        </div>
      </div>

      {/* ── Stats ── */}
      <section className="relative z-10 border-b border-white/8 py-14 px-6">
        <div className="max-w-3xl mx-auto grid grid-cols-2 sm:grid-cols-4 gap-8">
          {stats.map((s, i) => (
            <motion.div
              key={s.label}
              initial={{ opacity: 0, y: 20 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ delay: i * 0.09 }}
              className="text-center panel-surface-soft rounded-[22px] px-3 py-5"
            >
              <p className="font-display font-extrabold text-[42px] text-white tracking-tight leading-none mb-2">
                {s.value}
              </p>
              <p className="text-sm text-slate-400">{s.label}</p>
            </motion.div>
          ))}
        </div>
      </section>

      {/* ── Features ── */}
      <section className="relative z-10 py-24 px-6">
        <div className="max-w-5xl mx-auto">
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            className="text-center mb-16"
          >
            <h2 className="font-display font-extrabold text-[38px] text-white tracking-tight mb-4">
              Everything to protect your vault
            </h2>
            <p className="text-slate-300 max-w-xl mx-auto text-base leading-relaxed">
              Built for DeFi teams that need programmable trust.
              StableGuard lets you choose when AI should only observe, when it should protect, and when it can optimize.
            </p>
          </motion.div>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {features.map((f, i) => (
              <FeatureCard key={f.title} {...f} index={i} />
            ))}
          </div>
        </div>
      </section>

      {/* ── Pipeline flow ── */}
      <section className="relative z-10 py-20 px-6 bg-[#091320]/72 border-y border-white/8 backdrop-blur-xl">
        <div className="max-w-4xl mx-auto">
          <motion.div
            initial={{ opacity: 0 }}
            whileInView={{ opacity: 1 }}
            viewport={{ once: true }}
            className="text-center mb-14"
          >
            <h2 className="font-display font-bold text-2xl text-white mb-2">How it works</h2>
            <p className="text-sm text-slate-400">From raw price feed to policy-aware vault orchestration</p>
          </motion.div>

          <div className="flex flex-col sm:flex-row items-center justify-center gap-4 sm:gap-6">
            {pipeline.map((step, i) => (
              <motion.div
                key={step.label}
                initial={{ opacity: 0, x: -16 }}
                whileInView={{ opacity: 1, x: 0 }}
                viewport={{ once: true }}
                transition={{ delay: i * 0.12, type: "spring" }}
                className="flex items-center gap-4 sm:gap-6"
              >
                <div className="flex flex-col items-center">
                  <motion.div
                    whileHover={{ scale: 1.08 }}
                    className="w-14 h-14 rounded-2xl flex items-center justify-center shadow-sm border border-white/10"
                    style={{ background: step.color + "18", border: `1.5px solid ${step.color}22` }}
                  >
                    <step.icon size={22} style={{ color: step.color }} />
                  </motion.div>
                  <p className="text-xs font-bold text-slate-100 mt-2.5 text-center leading-tight">{step.label}</p>
                  <p className="text-[10px] text-slate-400 text-center mt-0.5">{step.sub}</p>
                </div>
                {i < pipeline.length - 1 && (
                  <ChevronRight size={18} className="text-slate-700 flex-shrink-0 hidden sm:block" />
                )}
              </motion.div>
            ))}
          </div>
        </div>
      </section>

      {/* ── CTA ── */}
      <section className="relative z-10 py-28 px-6 text-center">
        <motion.div
          initial={{ opacity: 0, y: 24 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          className="max-w-xl mx-auto"
        >
          <h2 className="font-display font-extrabold text-[44px] leading-[1.1] text-white mb-5 tracking-tight">
            Start with the{" "}
            <span className="text-orange-400">right control mode</span>
          </h2>
          <p className="text-slate-300 mb-10 leading-relaxed text-base">
            Connect your wallet, choose your autonomy level, and let StableGuard enforce your risk policy around the clock.
          </p>
          <Link
            href="/auth/register"
            className="inline-flex items-center gap-2 bg-white hover:bg-slate-100 text-slate-950 font-bold px-9 py-4 rounded-2xl transition-all hover:scale-[1.02] active:scale-[0.98] shadow-lg"
          >
            Create free account <ArrowRight size={16} />
          </Link>
        </motion.div>
      </section>

      {/* ── Footer ── */}
      <footer className="relative z-10 border-t border-white/8 py-8 px-6">
        <div className="max-w-5xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-2.5">
            <div className="w-6 h-6 rounded-md bg-orange-500 flex items-center justify-center shadow-[0_0_18px_rgba(255,122,26,0.22)]">
              <Shield size={12} className="text-white" />
            </div>
            <span className="text-sm font-semibold text-slate-100">StableGuard</span>
          </div>
          <p className="text-xs text-slate-400">
            Solana Devnet · Pyth Network · Configurable AI Control · MIT License
          </p>
          <Link href="/dashboard" className="text-xs text-slate-400 hover:text-orange-300 transition-colors">
            Dashboard →
          </Link>
        </div>
      </footer>
    </div>
  );
}
