"use client";

import { useCallback, useEffect, useRef, useState } from "react";
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
} from "lucide-react";
import Link from "next/link";

const STRATEGY_NAMES: Record<number, string> = { 0: "SAFE", 1: "BALANCED", 2: "YIELD" };
const CONTROL_MODE_META: Record<string, { label: string; tone: string; blurb: string }> = {
  MANUAL: {
    label: "MANUAL",
    tone: "text-gray-700",
    blurb: "AI monitors, explains, and alerts. Human stays fully in control.",
  },
  GUARDED: {
    label: "GUARDED",
    tone: "text-green-700",
    blurb: "AI only steps in for extreme-risk protection and depeg defense.",
  },
  BALANCED: {
    label: "BALANCED",
    tone: "text-blue-700",
    blurb: "AI runs moderate automation with protection-first policy limits.",
  },
  YIELD_MAX: {
    label: "YIELD MAX",
    tone: "text-orange-700",
    blurb: "AI takes the most initiative while keeping circuit breakers active.",
  },
  UNKNOWN: {
    label: "UNKNOWN",
    tone: "text-gray-500",
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
    if (t.status === "fulfilled") { setTokens(t.value); setError(null); }
    else setError(String((t as PromiseRejectedResult).reason));
    if (v.status === "fulfilled") setVault(v.value);
    if (d.status === "fulfilled") setDecisions(d.value.decisions ?? []);
    if (s.status === "fulfilled") setStats(s.value);
    if (ph.status === "fulfilled") setPriceHistory(ph.value.data ?? []);
    if (st.status === "fulfilled") setSettings(st.value);
    setLastUpdate(new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" }));
    setRefreshing(false);
  }, [selectedSymbol]);

  useEffect(() => {
    loadStatic();
    const t = setInterval(loadStatic, 30_000);
    return () => clearInterval(t);
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
    if (r < 30) return "bg-green-50 border-green-200";
    if (r < 60) return "bg-yellow-50 border-yellow-200";
    if (r < 80) return "bg-orange-50 border-orange-200";
    return "bg-red-50 border-red-200";
  }

  const currentDecision = liveData?.decision ?? null;
  const controlMode = settings?.control_mode ?? "UNKNOWN";
  const controlMeta = CONTROL_MODE_META[controlMode] ?? CONTROL_MODE_META.UNKNOWN;
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
      ? "text-amber-700"
      : execStatus === "executed"
      ? "text-green-700"
      : execStatus === "failed"
      ? "text-red-700"
      : "text-gray-600";

  return (
    <div className="min-h-screen bg-gray-50">
      <Header
        lastUpdate={lastUpdate}
        onRefresh={loadStatic}
        refreshing={refreshing}
        connected={connected}
        streamMode={mode}
      />

      <main className="max-w-7xl mx-auto px-4 sm:px-6 py-6 space-y-6">

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
            className="bg-amber-50 border border-amber-200 rounded-xl px-4 py-3"
          >
            <div className="flex items-center gap-2 text-sm font-semibold text-amber-800 mb-1">
              <AlertTriangle size={14} />
              Backend connection error
            </div>
            <p className="text-xs text-amber-700">
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
                <div className="rounded-xl border border-gray-200 bg-white p-4">
                  <div className="flex items-center gap-2 mb-2">
                    <Bot size={15} className={controlMeta.tone} />
                    <p className={`text-sm font-semibold ${controlMeta.tone}`}>{controlMeta.label}</p>
                  </div>
                  <p className="text-sm text-gray-600 leading-relaxed">{controlMeta.blurb}</p>
                  <div className="mt-3 grid grid-cols-2 gap-2">
                    <div className="rounded-lg bg-gray-50 p-2 border border-gray-100">
                      <div className="text-[10px] text-gray-500">Auto execute</div>
                      <div className="text-sm font-semibold text-gray-800">{settings?.auto_execute ? "Enabled" : "Disabled"}</div>
                    </div>
                    <div className="rounded-lg bg-gray-50 p-2 border border-gray-100">
                      <div className="text-[10px] text-gray-500">Yield layer</div>
                      <div className="text-sm font-semibold text-gray-800">{settings?.yield_enabled ? "Enabled" : "Disabled"}</div>
                    </div>
                  </div>
                </div>

                <div className="rounded-xl border border-gray-200 bg-white p-4">
                  <div className="flex items-center gap-2 mb-2">
                    <Lock size={15} className={execStatusTone} />
                    <p className={`text-sm font-semibold ${execStatusTone}`}>Execution Status</p>
                  </div>
                  <p className={`text-lg font-bold ${execStatusTone}`}>{execStatusLabel}</p>
                  <p className="text-sm text-gray-600 leading-relaxed mt-2">
                    {liveData?.exec_note ?? "Waiting for the next pipeline decision to report execution status."}
                  </p>
                </div>

                <div className="rounded-xl border border-gray-200 bg-white p-4">
                  <div className="flex items-center gap-2 mb-2">
                    <Shield size={15} className="text-gray-600" />
                    <p className="text-sm font-semibold text-gray-800">Policy Envelope</p>
                  </div>
                  <div className="space-y-2 text-sm text-gray-600">
                    <div className="flex items-center justify-between gap-3">
                      <span>Risk threshold</span>
                      <span className="font-semibold text-gray-900">{settings?.alert_risk_threshold ?? "—"}</span>
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <span>Yield entry risk</span>
                      <span className="font-semibold text-gray-900">{settings?.yield_entry_risk ?? "—"}</span>
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <span>Circuit breaker</span>
                      <span className="font-semibold text-gray-900">{settings?.circuit_breaker_pause_pct ?? "—"}%</span>
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <span>On-chain strategy</span>
                      <span className="font-semibold text-gray-900">{vault ? STRATEGY_NAMES[vault.strategy_mode] ?? "—" : "—"}</span>
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
                  <p className="text-xs text-gray-600 text-center leading-relaxed px-2">
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
                        className="bg-white/60 rounded-lg p-2 text-center border border-white"
                      >
                        <div className="text-[10px] text-gray-500">{m.label}</div>
                        <div className="text-xs font-mono font-medium text-gray-800 mt-0.5 tabular-nums">
                          {m.value}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
                {!risk && (
                  <p className="text-xs text-gray-400 text-center">
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
                          ? "bg-gray-900 text-white"
                          : "text-gray-500 hover:bg-gray-100"
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
                      <div key={s.label} className="bg-gray-50 rounded-lg p-2">
                        <div className="text-[10px] text-gray-500">{s.label}</div>
                        <div className="text-sm font-semibold text-gray-800">{s.value}</div>
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
                  <TrendingUp size={12} className="text-gray-400" />
                  <span className="text-xs text-gray-500">
                    Latest:{" "}
                    <span className="font-semibold text-gray-700">{currentDecision.action}</span>
                  </span>
                </div>
              )
            }
          >
            <DecisionFeed decisions={decisions} />
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
                  <p className="text-xs font-semibold text-gray-500 mb-1.5">Risk Analysis</p>
                  <p className="text-sm text-gray-700 leading-relaxed bg-gray-50 rounded-xl p-3">
                    {currentDecision.risk_analysis}
                  </p>
                </div>
                <div>
                  <p className="text-xs font-semibold text-gray-500 mb-1.5">Yield Analysis</p>
                  <p className="text-sm text-gray-700 leading-relaxed bg-gray-50 rounded-xl p-3">
                    {currentDecision.yield_analysis}
                  </p>
                </div>
                <div className="md:col-span-2">
                  <p className="text-xs font-semibold text-gray-500 mb-1.5">Strategy Rationale</p>
                  <p className="text-sm text-gray-700 leading-relaxed bg-gray-50 rounded-xl p-3">
                    {currentDecision.rationale}
                  </p>
                </div>
              </div>
            </Card>
          </motion.div>
        )}

        {/* ── Footer ── */}
        <div className="flex items-center justify-between py-2">
          <p className="text-xs text-gray-400">StableGuard v3 · Solana Devnet</p>
          <Link
            href="/settings"
            className="flex items-center gap-1.5 text-xs text-gray-500 hover:text-gray-800 transition-colors"
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
