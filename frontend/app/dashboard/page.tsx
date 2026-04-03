"use client";

import { startTransition, useCallback, useEffect, useRef, useState } from "react";
import { motion } from "framer-motion";
import { useRealtime, FeedMessage } from "@/lib/useRealtime";
import { api, TokensResponse, VaultState, DecisionRow, HistoryStats, SettingsResponse } from "@/lib/api";
import { toast } from "@/lib/toast";
import { Header } from "@/components/Header";
import { Card } from "@/components/Card";
import { RiskGauge } from "@/components/RiskGauge";
import { TokenPrices } from "@/components/TokenPrices";
import { AllocationPie } from "@/components/AllocationPie";
import { DecisionFeed } from "@/components/DecisionFeed";
import { StatCard } from "@/components/StatCard";
import { PriceChart } from "@/components/PriceChart";
import { AnimatedNumber } from "@/components/AnimatedNumber";
import { YieldOpportunities } from "@/components/YieldOpportunities";
import { YieldPosition } from "@/components/YieldPosition";
import { DAOPayments } from "@/components/DAOPayments";
import { AIChat } from "@/components/AIChat";
import { AutopilotIntent } from "@/components/AutopilotIntent";
import { SlippageAnalysis } from "@/components/SlippageAnalysis";
import { WhaleIntelligence } from "@/components/WhaleIntelligence";
import { PipelineVisualizer } from "@/components/PipelineVisualizer";
import {
  BarChart2,
  Zap,
  Shield,
  TrendingUp,
  Activity,
  AlertTriangle,
  Settings,
  Bot,
  Lock,
  Sparkles,
  Radar,
  Orbit,
} from "lucide-react";
import Link from "next/link";

const STRATEGY_NAMES: Record<number, string> = { 0: "SAFE", 1: "BALANCED", 2: "YIELD" };
const CONTROL_MODE_META: Record<string, { label: string; tone: string; blurb: string }> = {
  MANUAL: {
    label: "MANUAL",
    tone: "text-slate-200",
    blurb: "AI monitors, explains, and alerts. Human stays fully in control.",
  },
  GUARDED: {
    label: "GUARDED",
    tone: "text-emerald-300",
    blurb: "AI only steps in for extreme-risk protection and depeg defense.",
  },
  BALANCED: {
    label: "BALANCED",
    tone: "text-cyan-300",
    blurb: "AI runs moderate automation with protection-first policy limits.",
  },
  YIELD_MAX: {
    label: "YIELD MAX",
    tone: "text-orange-300",
    blurb: "AI takes the most initiative while keeping circuit breakers active.",
  },
  UNKNOWN: {
    label: "UNKNOWN",
    tone: "text-slate-400",
    blurb: "Control profile unavailable.",
  },
};

/* ── Animation variants ─────────────────────────────────────────────── */
const container = {
  hidden: { opacity: 0 },
  show: { opacity: 1, transition: { staggerChildren: 0.07, delayChildren: 0.05 } },
} as const;
const card = {
  hidden: { opacity: 0, y: 18 },
  show: { opacity: 1, y: 0, transition: { type: "spring" as const, stiffness: 110, damping: 22 } },
} as const;

/* ── Page ───────────────────────────────────────────────────────────── */
export default function Dashboard() {
  const [liveData, setLiveData] = useState<FeedMessage | null>(null);
  const prevRiskRef = useRef<number | null>(null);

  const handleMessage = useCallback((msg: FeedMessage) => {
    setLiveData(msg);
    const r = msg.risk?.risk_level;
    const prev = prevRiskRef.current;
    if (prev !== null && r !== undefined) {
      if (prev < 80 && r >= 80) {
        toast.show("danger", "High Risk Alert", `Risk jumped to ${r.toFixed(0)} — review AI analysis`);
      } else if (prev < 60 && r >= 60) {
        toast.show("warning", "Risk Elevated", `Score rose to ${r.toFixed(0)}`);
      }
    }
    if (r !== undefined) prevRiskRef.current = r;
  }, []);

  const { connected, mode } = useRealtime({ onMessage: handleMessage });

  const [tokens, setTokens] = useState<TokensResponse | null>(null);
  const [vault, setVault] = useState<VaultState | null>(null);
  const [decisions, setDecisions] = useState<DecisionRow[]>([]);
  const [stats, setStats] = useState<HistoryStats | null>(null);
  const [settings, setSettings] = useState<SettingsResponse | null>(null);
  const [priceHistory, setPriceHistory] = useState<{ ts: number; price: number; conf: number }[]>([]);
  const [selectedSymbol, setSelectedSymbol] = useState("USDC");
  const [refreshing, setRefreshing] = useState(false);
  const [lastUpdate, setLastUpdate] = useState("");
  const [error, setError] = useState<string | null>(null);

  const loadStatic = useCallback(async () => {
    setRefreshing(true);
    const [t, v, d, s, ph, st] = await Promise.allSettled([
      api.tokens(),
      api.vault(),
      api.decisions(10),
      api.stats(),
      api.priceHistory(selectedSymbol, 120),
      api.settings(),
    ]);
    startTransition(() => {
      if (t.status === "fulfilled") { setTokens(t.value); setError(null); }
      else setError(String((t as PromiseRejectedResult).reason));
      if (v.status === "fulfilled") setVault(v.value);
      if (d.status === "fulfilled") setDecisions(d.value.decisions ?? []);
      if (s.status === "fulfilled") setStats(s.value);
      if (ph.status === "fulfilled") setPriceHistory(ph.value.data ?? []);
      if (st.status === "fulfilled") setSettings(st.value);
      setLastUpdate(new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" }));
    });
    setRefreshing(false);
  }, [selectedSymbol]);

  useEffect(() => {
    const initial = setTimeout(() => {
      void loadStatic();
    }, 0);
    const t = setInterval(() => {
      void loadStatic();
    }, 30_000);
    return () => {
      clearTimeout(initial);
      clearInterval(t);
    };
  }, [loadStatic]);

  const risk = liveData?.risk ?? null;
  const riskLevel = risk?.risk_level ?? 0;
  const livePrices = liveData?.prices ?? {};
  const maxDeviation = liveData?.max_deviation ?? tokens?.max_deviation ?? 0;

  const enrichedTokens = tokens
    ? {
        ...tokens,
        max_deviation: maxDeviation,
        tokens: tokens.tokens.map((t) => ({
          ...t,
          price: livePrices[t.symbol] ?? t.price,
        })),
      }
    : null;

  function riskBg(r: number) {
    if (r < 30) return "bg-emerald-400/6 border-emerald-300/18";
    if (r < 60) return "bg-yellow-400/6 border-yellow-300/18";
    if (r < 80) return "bg-orange-400/8 border-orange-300/18";
    return "bg-red-400/8 border-red-300/18";
  }

  const currentDecision = liveData?.decision ?? null;
  const controlMode = settings?.control_mode ?? "UNKNOWN";
  const controlMeta = CONTROL_MODE_META[controlMode] ?? CONTROL_MODE_META.UNKNOWN;
  const executionMode = settings?.execution_mode ?? "record_only";
  const policyEval = liveData?.policy;
  const yieldLiveMode = settings?.yield_live_mode ?? "disabled";
  const execStatus = liveData?.exec_status ?? "warming_up";
  const execStatusLabel =
    execStatus === "signal_only"
      ? "Signal Only"
      : execStatus === "executed"
      ? "Executed"
      : execStatus === "failed"
      ? "Failed"
      : "Standby";
  const execStatusTone =
    execStatus === "signal_only"
      ? "text-amber-300"
      : execStatus === "executed"
      ? "text-emerald-300"
      : execStatus === "failed"
      ? "text-red-300"
      : "text-slate-300";
  const policyVerdictLabel =
    policyEval?.verdict === "allowed"
      ? "Allowed"
      : policyEval?.verdict === "blocked"
      ? "Blocked"
      : policyEval?.verdict === "requires_approval"
      ? "Approval"
      : "Pending";
  const policyVerdictTone =
    policyEval?.verdict === "allowed"
      ? "text-emerald-300"
      : policyEval?.verdict === "blocked"
      ? "text-red-300"
      : policyEval?.verdict === "requires_approval"
      ? "text-amber-300"
      : "text-slate-300";

  return (
    <div className="min-h-screen relative overflow-x-hidden">
      <div className="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden>
        <div className="absolute -top-40 left-[10%] h-80 w-80 rounded-full bg-cyan-400/10 blur-[100px]" />
        <div className="absolute top-32 right-[4%] h-[26rem] w-[26rem] rounded-full bg-orange-500/12 blur-[120px]" />
        <div className="absolute inset-x-0 top-0 h-[32rem] opacity-40 animate-grid-drift bg-[linear-gradient(transparent_96%,rgba(255,255,255,0.06)_100%),linear-gradient(90deg,transparent_96%,rgba(255,255,255,0.06)_100%)] bg-[size:64px_64px]" />
      </div>
      <Header
        lastUpdate={lastUpdate}
        onRefresh={loadStatic}
        refreshing={refreshing}
        connected={connected}
        streamMode={mode}
      />

      <main className="relative max-w-7xl mx-auto px-4 sm:px-6 py-6 space-y-6">
        <motion.section
          initial={{ opacity: 0, y: 18 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.35 }}
          className="panel-surface neon-border rounded-[28px] px-5 py-5 sm:px-7 sm:py-6 overflow-hidden"
        >
          <div className="absolute inset-0 pointer-events-none" aria-hidden>
            <div className="absolute inset-y-0 left-0 w-1/2 bg-[radial-gradient(circle_at_top_left,rgba(79,227,255,0.14),transparent_55%)]" />
            <div className="absolute inset-y-0 right-0 w-1/2 bg-[radial-gradient(circle_at_top_right,rgba(255,122,26,0.18),transparent_52%)]" />
            <div className="absolute top-1/2 left-[-12%] h-px w-[50%] bg-gradient-to-r from-transparent via-cyan-300/30 to-transparent animate-scanner" />
          </div>
          <div className="relative grid grid-cols-1 xl:grid-cols-[1.4fr_0.9fr] gap-6 items-start">
            <div>
              <div className="flex flex-wrap items-center gap-2 mb-4">
                <span className="glass-pill rounded-full px-3 py-1 text-[10px] uppercase tracking-[0.24em] text-cyan-200">Autonomous Treasury</span>
                <span className="glass-pill rounded-full px-3 py-1 text-[10px] uppercase tracking-[0.24em] text-orange-200">Stablecoin Defense Grid</span>
              </div>
              <h1 className="font-display text-3xl sm:text-4xl lg:text-5xl leading-[0.98] tracking-[-0.04em] text-white max-w-3xl">
                AI modes, policy firewalls, and yield execution in one Solana control surface.
              </h1>
              <p className="mt-4 max-w-2xl text-sm sm:text-base text-slate-300 leading-relaxed">
                StableGuard turns treasury automation into something visible and bounded: market signals in, AI proposals evaluated, policy verdicts enforced, and trusted execution paths exposed live.
              </p>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-3 xl:grid-cols-1 gap-3">
              {[
                { icon: <Radar size={15} className="text-cyan-300" />, label: "Execution", value: execStatusLabel, tone: execStatusTone },
                { icon: <Sparkles size={15} className="text-orange-300" />, label: "Policy", value: policyVerdictLabel, tone: policyVerdictTone },
                { icon: <Orbit size={15} className="text-emerald-300" />, label: "Yield", value: yieldLiveMode === "live" ? "Live" : yieldLiveMode === "strategy_only" ? "Strategy" : "Disabled", tone: yieldLiveMode === "live" ? "text-emerald-300" : yieldLiveMode === "strategy_only" ? "text-amber-300" : "text-slate-300" },
              ].map((item) => (
                <div key={item.label} className="glass-pill rounded-[18px] px-4 py-3">
                  <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-slate-400">
                    {item.icon}
                    {item.label}
                  </div>
                  <div className={`mt-2 text-lg font-semibold ${item.tone}`}>{item.value}</div>
                </div>
              ))}
            </div>
          </div>
        </motion.section>

        {/* ── Pipeline Architecture Visualizer ── */}
        <motion.div
          initial={{ opacity: 0, y: -8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.35 }}
        >
          <PipelineVisualizer liveData={liveData} connected={connected} />
        </motion.div>

        {/* Error banner */}
        {error && (
          <motion.div
            initial={{ opacity: 0, y: -8 }}
            animate={{ opacity: 1, y: 0 }}
            className="rounded-2xl px-4 py-3 bg-amber-400/10 border border-amber-300/20"
          >
            <div className="flex items-center gap-2 text-sm font-semibold text-amber-200 mb-1">
              <AlertTriangle size={14} />
              Backend connection error
            </div>
            <p className="text-xs text-amber-100/80">
              {error.includes("404") || error.includes("Cannot GET")
                ? "Backend running with old code — restart: kill process → cd backend → go run main.go"
                : "Backend unreachable. Start: cd backend && go run main.go"}
            </p>
          </motion.div>
        )}

        {/* ── Top stats ── */}
        <motion.div
          variants={container}
          initial="hidden"
          animate="show"
          className="grid grid-cols-2 sm:grid-cols-4 gap-3"
        >
          <motion.div variants={card}>
            <StatCard
              label="Risk Level"
              value={
                <AnimatedNumber
                  value={riskLevel}
                  decimals={0}
                  className="font-mono-data"
                />
              }
              sub={risk?.action ?? "Warming up…"}
              icon={<Activity size={15} className="text-orange-500" />}
              accent={
                riskLevel >= 60
                  ? "text-red-600"
                  : riskLevel >= 30
                  ? "text-yellow-600"
                  : "text-green-600"
              }
            />
          </motion.div>

          <motion.div variants={card}>
            <StatCard
              label="Max Deviation"
              value={
                <AnimatedNumber
                  value={maxDeviation}
                  decimals={4}
                  suffix="%"
                  className="font-mono-data"
                />
              }
              sub="vs USDC"
              icon={<BarChart2 size={15} className="text-blue-500" />}
            />
          </motion.div>

          <motion.div variants={card}>
            <StatCard
              label="AI Decisions"
              value={stats?.total_decisions ?? "—"}
              sub={stats?.total_rebalances ? `${stats.total_rebalances} rebalances` : "loading…"}
              icon={<Zap size={15} className="text-purple-500" />}
            />
          </motion.div>

          <motion.div variants={card}>
            <StatCard
              label="Control Mode"
              value={controlMeta.label}
              sub={controlMeta.blurb}
              icon={<Bot size={15} className="text-gray-400" />}
              accent={controlMeta.tone}
            />
          </motion.div>
        </motion.div>

        {/* ── Main grid ── */}
        <motion.div
          variants={container}
          initial="hidden"
          animate="show"
          className="grid grid-cols-1 lg:grid-cols-3 gap-4"
        >
          <motion.div variants={card} className="lg:col-span-3">
            <Card title="AI Control Layer" subtitle="Configurable autonomy instead of one fixed autopilot">
              <div className="grid grid-cols-1 lg:grid-cols-3 gap-3">
                <div className="panel-surface-soft rounded-[22px] p-4">
                  <div className="flex items-center gap-2 mb-2">
                    <Bot size={15} className={controlMeta.tone} />
                    <p className={`text-sm font-semibold ${controlMeta.tone}`}>{controlMeta.label}</p>
                  </div>
                  <p className="text-sm text-slate-300 leading-relaxed">{controlMeta.blurb}</p>
                  <div className="mt-3 grid grid-cols-2 gap-2">
                    <div className="rounded-xl bg-white/5 p-2.5 border border-white/8">
                      <div className="text-[10px] uppercase tracking-[0.16em] text-slate-500">Auto execute</div>
                      <div className="text-sm font-semibold text-slate-100">{settings?.auto_execute ? "Enabled" : "Disabled"}</div>
                    </div>
                    <div className="rounded-xl bg-white/5 p-2.5 border border-white/8">
                      <div className="text-[10px] uppercase tracking-[0.16em] text-slate-500">Yield layer</div>
                      <div className="text-sm font-semibold text-slate-100">
                        {yieldLiveMode === "live" ? "Live" : yieldLiveMode === "strategy_only" ? "Strategy only" : settings?.yield_enabled ? "Enabled" : "Disabled"}
                      </div>
                    </div>
                    <div className="rounded-xl bg-white/5 p-2.5 border border-white/8">
                      <div className="text-[10px] uppercase tracking-[0.16em] text-slate-500">Policy verdict</div>
                      <div className={`text-sm font-semibold ${policyVerdictTone}`}>{policyVerdictLabel}</div>
                    </div>
                    <div className="rounded-xl bg-white/5 p-2.5 border border-white/8">
                      <div className="text-[10px] uppercase tracking-[0.16em] text-slate-500">Action class</div>
                      <div className="text-sm font-semibold text-slate-100">{policyEval?.action_class ?? "observe"}</div>
                    </div>
                  </div>
                  <div className="mt-3 rounded-xl border border-amber-300/20 bg-amber-400/10 px-3 py-2">
                    <div className="text-[10px] font-semibold uppercase tracking-wide text-amber-200">
                      Custody Boundary
                    </div>
                    <p className="mt-1 text-xs leading-relaxed text-amber-100/80">
                      {executionMode === "record_only"
                        ? "AI can score, propose, and record allocation shifts, but current custody keeps market swaps unavailable."
                        : settings?.execution_note}
                    </p>
                  </div>
                  {policyEval?.reason && (
                    <div className="mt-3 rounded-xl border border-white/8 bg-white/5 px-3 py-2">
                      <div className="text-[10px] font-semibold uppercase tracking-wide text-slate-400">
                        Policy Reason
                      </div>
                      <p className="mt-1 text-xs leading-relaxed text-slate-200">{policyEval.reason}</p>
                    </div>
                  )}
                  {settings?.yield_live_note && (
                    <div className="mt-3 rounded-xl border border-cyan-300/20 bg-cyan-400/8 px-3 py-2">
                      <div className="text-[10px] font-semibold uppercase tracking-wide text-cyan-200">
                        Yield Execution
                      </div>
                      <p className="mt-1 text-xs leading-relaxed text-cyan-100/85">{settings.yield_live_note}</p>
                    </div>
                  )}
                </div>

                <div className="panel-surface-soft rounded-[22px] p-4">
                  <div className="flex items-center gap-2 mb-2">
                    <Lock size={15} className={execStatusTone} />
                    <p className={`text-sm font-semibold ${execStatusTone}`}>Execution Status</p>
                  </div>
                  <p className={`text-lg font-bold ${execStatusTone}`}>{execStatusLabel}</p>
                  <p className="text-sm text-slate-300 leading-relaxed mt-2">
                    {liveData?.exec_note ?? "Waiting for the next pipeline decision to report execution status."}
                  </p>
                </div>

                <div className="panel-surface-soft rounded-[22px] p-4">
                  <div className="flex items-center gap-2 mb-2">
                    <Shield size={15} className="text-slate-300" />
                    <p className="text-sm font-semibold text-slate-100">Policy Envelope</p>
                  </div>
                  <div className="space-y-2 text-sm text-slate-300">
                    <div className="flex items-center justify-between gap-3">
                      <span>Risk threshold</span>
                      <span className="font-semibold text-slate-50">{settings?.alert_risk_threshold ?? "—"}</span>
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <span>Yield entry risk</span>
                      <span className="font-semibold text-slate-50">{settings?.yield_entry_risk ?? "—"}</span>
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <span>Circuit breaker</span>
                      <span className="font-semibold text-slate-50">{settings?.circuit_breaker_pause_pct ?? "—"}%</span>
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <span>On-chain strategy</span>
                      <span className="font-semibold text-slate-50">{vault ? STRATEGY_NAMES[vault.strategy_mode] ?? "—" : "—"}</span>
                    </div>
                  </div>
                </div>
              </div>
            </Card>
          </motion.div>

          <motion.div variants={card}>
            <Card className={`border ${riskBg(riskLevel)}`}>
              <div className="flex flex-col items-center gap-3">
                <RiskGauge value={riskLevel} size={180} />
                {risk?.summary && (
                  <p className="text-xs text-slate-300 text-center leading-relaxed px-2">
                    {risk.summary}
                  </p>
                )}
                {risk && (
                  <div className="w-full grid grid-cols-3 gap-2 mt-1">
                    {[
                      { label: "Trend",      value: risk.trend?.toFixed(5)      ?? "—" },
                      { label: "Velocity",   value: risk.velocity?.toFixed(5)   ?? "—" },
                      { label: "Volatility", value: risk.volatility?.toFixed(5) ?? "—" },
                    ].map((m) => (
                      <div
                        key={m.label}
                        className="rounded-xl p-2 text-center border border-white/10 bg-white/6"
                      >
                        <div className="text-[10px] text-slate-400">{m.label}</div>
                        <div className="text-xs font-mono font-medium text-slate-100 mt-0.5 tabular-nums">
                          {m.value}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
                {!risk && (
                  <p className="text-xs text-slate-400 text-center">
                    {connected ? "Waiting for first price update…" : "Connecting to backend…"}
                  </p>
                )}
              </div>
            </Card>
          </motion.div>

          <motion.div variants={card} className="lg:col-span-2">
            <Card title="Live Prices" subtitle="Pyth Network · real-time">
              {enrichedTokens ? (
                <TokenPrices tokens={enrichedTokens.tokens} fetchedAt={enrichedTokens.fetched_at} />
              ) : (
                <div className="h-32 flex items-center justify-center text-sm text-gray-400">
                  {error ? "Failed to load — check backend" : "Connecting…"}
                </div>
              )}
            </Card>
          </motion.div>
        </motion.div>

        {/* ── Second row ── */}
        <motion.div
          variants={container}
          initial="hidden"
          animate="show"
          className="grid grid-cols-1 lg:grid-cols-2 gap-4"
        >
          <motion.div variants={card}>
            <Card
              title="Price History"
              action={
                <div className="flex gap-1">
                  {["USDC", "USDT", "DAI", "PYUSD"].map((s) => (
                    <button
                      key={s}
                      onClick={() => setSelectedSymbol(s)}
                      className={`text-xs px-2 py-0.5 rounded-full transition-colors ${
                        selectedSymbol === s
                          ? "bg-white text-slate-950"
                          : "text-slate-400 hover:bg-white/6"
                      }`}
                    >
                      {s}
                    </button>
                  ))}
                </div>
              }
            >
              <PriceChart
                data={priceHistory}
                symbol={selectedSymbol}
                color={
                  selectedSymbol === "USDT"
                    ? "#16a34a"
                    : selectedSymbol === "DAI"
                    ? "#d97706"
                    : "#2563eb"
                }
              />
            </Card>
          </motion.div>

          <motion.div variants={card}>
            <Card
              title="Vault Allocation"
              subtitle={vault ? `${vault.num_tokens} tokens registered` : undefined}
            >
              {vault ? (
                <>
                  <AllocationPie
                    balances={vault.balances}
                    mints={vault.mints}
                    numTokens={vault.num_tokens}
                  />
                  <div className="mt-4 grid grid-cols-2 gap-2">
                    {[
                      { label: "Total deposited", value: (vault.total_deposited / 1e6).toFixed(2) + "M" },
                      { label: "Rebalances",       value: vault.total_rebalances },
                    ].map((s) => (
                      <div key={s.label} className="bg-white/5 rounded-xl p-2.5 border border-white/8">
                        <div className="text-[10px] text-slate-400">{s.label}</div>
                        <div className="text-sm font-semibold text-slate-100">{s.value}</div>
                      </div>
                    ))}
                  </div>
                </>
              ) : (
                <div className="h-32 flex items-center justify-center text-sm text-gray-400">
                  {error ? "Vault unavailable" : "Connecting to vault…"}
                </div>
              )}
            </Card>
          </motion.div>
        </motion.div>

        {/* ── Yield Optimizer ── */}
        <motion.div
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.38 }}
          className="space-y-3"
        >
          <YieldPosition position={liveData?.yield_position ?? null} />
          <YieldOpportunities />
        </motion.div>

        {/* ── Slippage Analysis + Whale Intelligence ── */}
        <motion.div
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.38 }}
          className="grid grid-cols-1 lg:grid-cols-2 gap-4"
        >
          <SlippageAnalysis symbol="USDC" />
          <WhaleIntelligence />
        </motion.div>

        {/* ── Autopilot ── */}
        <motion.div
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.40 }}
        >
          <AutopilotIntent />
        </motion.div>

        {/* ── DAO Treasury Payments ── */}
        <motion.div initial={{ opacity: 0, y: 16 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.42 }}>
          <DAOPayments />
        </motion.div>

        {/* ── AI Decision feed ── */}
        <motion.div initial={{ opacity: 0, y: 16 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.45 }}>
          <Card
            title="AI Decisions"
            subtitle="Multi-agent pipeline · Claude"
            action={
              currentDecision && (
                <div className="flex items-center gap-1.5">
                  <TrendingUp size={12} className="text-slate-400" />
                  <span className="text-xs text-slate-400">
                    Latest:{" "}
                    <span className="font-semibold text-slate-100">{currentDecision.action}</span>
                  </span>
                </div>
              )
            }
          >
            <div className="content-auto">
              <DecisionFeed decisions={decisions} />
            </div>
          </Card>
        </motion.div>

        {/* ── Current AI analysis ── */}
        {currentDecision && (
          <motion.div
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.55 }}
          >
            <Card title="Current AI Analysis" subtitle="From the last pipeline run">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <p className="text-xs font-semibold text-slate-400 mb-1.5">Risk Analysis</p>
                  <p className="text-sm text-slate-200 leading-relaxed bg-white/5 rounded-xl p-3 border border-white/8">
                    {currentDecision.risk_analysis}
                  </p>
                </div>
                <div>
                  <p className="text-xs font-semibold text-slate-400 mb-1.5">Yield Analysis</p>
                  <p className="text-sm text-slate-200 leading-relaxed bg-white/5 rounded-xl p-3 border border-white/8">
                    {currentDecision.yield_analysis}
                  </p>
                </div>
                <div className="md:col-span-2">
                  <p className="text-xs font-semibold text-slate-400 mb-1.5">Strategy Rationale</p>
                  <p className="text-sm text-slate-200 leading-relaxed bg-white/5 rounded-xl p-3 border border-white/8">
                    {currentDecision.rationale}
                  </p>
                </div>
              </div>
            </Card>
          </motion.div>
        )}

        {/* ── Footer ── */}
        <div className="flex items-center justify-between py-2">
          <p className="text-xs text-slate-500">StableGuard v3 · Solana Devnet</p>
          <Link
            href="/settings"
            className="flex items-center gap-1.5 text-xs text-slate-400 hover:text-white transition-colors"
          >
            <Settings size={12} />
            Settings &amp; Alerts
          </Link>
        </div>

      </main>

      {/* ── Floating AI Chat ── */}
      <AIChat />
    </div>
  );
}
