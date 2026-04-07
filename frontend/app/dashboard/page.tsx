"use client";

import { startTransition, useCallback, useEffect, useRef, useState } from "react";
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
import { LiveDemoFlow } from "@/components/LiveDemoFlow";
import { MarketTape } from "@/components/MarketTape";
import { Goals } from "@/components/Goals";
import { DepegForecast } from "@/components/DepegForecast";
import { OnChainRebalance } from "@/components/OnChainRebalance";
import { ASSET_OPTIONS, getAssetMeta } from "@/lib/assets";
import {
  LayoutDashboard,
  ShieldAlert,
  TrendingUp,
  Bot,
  Link2,
  Activity,
  Zap,
  BarChart2,
  Settings,
  RefreshCw,
  AlertTriangle,
  ChevronRight,
  Orbit,
  ExternalLink,
} from "lucide-react";
import Link from "next/link";

/* ─── Constants ──────────────────────────────────────────────────────── */
const CONTROL_MODE_META: Record<string, { label: string; tone: string; blurb: string; color: string }> = {
  MANUAL:    { label: "MANUAL",    tone: "text-slate-200",   color: "bg-slate-500/20 text-slate-200",   blurb: "AI monitors only. You decide." },
  GUARDED:   { label: "GUARDED",   tone: "text-emerald-300", color: "bg-emerald-500/20 text-emerald-300", blurb: "AI intervenes on extreme risk." },
  BALANCED:  { label: "BALANCED",  tone: "text-cyan-300",    color: "bg-cyan-500/20 text-cyan-300",    blurb: "Moderate automation with protection." },
  YIELD_MAX: { label: "YIELD MAX", tone: "text-orange-300",  color: "bg-orange-500/20 text-orange-300",  blurb: "Maximize APY with circuit breakers." },
  UNKNOWN:   { label: "UNKNOWN",   tone: "text-slate-400",   color: "bg-slate-600/20 text-slate-400",   blurb: "Control profile unavailable." },
};

/* ─── Sidebar tabs ───────────────────────────────────────────────────── */
type Tab = "overview" | "risk" | "yield" | "ai" | "onchain";

const TABS: { id: Tab; label: string; icon: React.ReactNode; badge?: string }[] = [
  { id: "overview", label: "Overview",     icon: <LayoutDashboard size={16} /> },
  { id: "risk",     label: "Risk & Intel", icon: <ShieldAlert size={16} /> },
  { id: "yield",    label: "Yield",        icon: <TrendingUp size={16} /> },
  { id: "ai",       label: "AI Agent",     icon: <Bot size={16} />, badge: "LIVE" },
  { id: "onchain",  label: "On-Chain",     icon: <Link2 size={16} /> },
];

/* ─── Animations ─────────────────────────────────────────────────────── */

/* ─── Page ───────────────────────────────────────────────────────────── */
export default function Dashboard() {
  const [activeTab, setActiveTab] = useState<Tab>("overview");
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [liveData, setLiveData] = useState<FeedMessage | null>(null);
  const prevRiskRef = useRef<number | null>(null);

  const handleMessage = useCallback((msg: FeedMessage) => {
    setLiveData(msg);
    const r = msg.risk?.risk_level;
    const prev = prevRiskRef.current;
    if (prev !== null && r !== undefined) {
      if (prev < 80 && r >= 80) toast.show("danger", "High Risk Alert", `Risk jumped to ${r.toFixed(0)}`);
      else if (prev < 60 && r >= 60) toast.show("warning", "Risk Elevated", `Score rose to ${r.toFixed(0)}`);
    }
    if (r !== undefined) prevRiskRef.current = r;
  }, []);

  const { connected, mode } = useRealtime({ onMessage: handleMessage });

  const [tokens,       setTokens]       = useState<TokensResponse | null>(null);
  const [vault,        setVault]         = useState<VaultState | null>(null);
  const [decisions,    setDecisions]     = useState<DecisionRow[]>([]);
  const [stats,        setStats]         = useState<HistoryStats | null>(null);
  const [settings,     setSettings]      = useState<SettingsResponse | null>(null);
  const [priceHistory, setPriceHistory]  = useState<{ ts: number; price: number; conf: number }[]>([]);
  const [selectedSymbol, setSelectedSymbol] = useState("USDC");
  const [refreshing,   setRefreshing]    = useState(false);
  const [lastUpdate,   setLastUpdate]    = useState("");
  const [error,        setError]         = useState<string | null>(null);

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
    const t0 = setTimeout(() => void loadStatic(), 0);
    const t1 = setInterval(() => void loadStatic(), 30_000);
    return () => { clearTimeout(t0); clearInterval(t1); };
  }, [loadStatic]);

  /* ─── Derived state ─── */
  const risk          = liveData?.risk ?? null;
  const riskLevel     = risk?.risk_level ?? 0;
  const livePrices    = liveData?.prices ?? {};
  const maxDeviation  = liveData?.max_deviation ?? tokens?.max_deviation ?? 0;
  const controlMode   = settings?.control_mode ?? "UNKNOWN";
  const controlMeta   = CONTROL_MODE_META[controlMode] ?? CONTROL_MODE_META.UNKNOWN;
  const executionMode = settings?.execution_mode ?? "record_only";
  const aiModel       = settings?.ai_agent_model ?? "claude-haiku-4-5";
  const yieldLiveMode = settings?.yield_live_mode ?? "disabled";
  const execStatus    = liveData?.exec_status ?? "warming_up";
  const policyEval    = liveData?.policy;
  const selectedAssetMeta = getAssetMeta(selectedSymbol);

  const enrichedTokens = tokens ? {
    ...tokens,
    max_deviation: maxDeviation,
    tokens: tokens.tokens.map((t) => ({ ...t, price: livePrices[t.symbol] ?? t.price })),
  } : null;

  function riskColor(r: number) {
    if (r < 30) return "text-emerald-300";
    if (r < 60) return "text-amber-300";
    if (r < 80) return "text-orange-300";
    return "text-rose-400";
  }
  function riskBadge(r: number) {
    if (r < 30) return "bg-emerald-500/15 text-emerald-300 border-emerald-400/20";
    if (r < 60) return "bg-amber-500/15 text-amber-300 border-amber-400/20";
    if (r < 80) return "bg-orange-500/15 text-orange-300 border-orange-400/20";
    return "bg-rose-500/15 text-rose-400 border-rose-400/20";
  }

  /* ─── Render ─────────────────────────────────────────────────────── */
  return (
    <div className="app-shell min-h-screen flex flex-col relative overflow-x-hidden">
      {/* Ambient blobs */}
      <div className="pointer-events-none fixed inset-0 overflow-hidden" aria-hidden>
        <div className="absolute -top-40 left-[10%] h-80 w-80 rounded-full bg-cyan-400/8 blur-[100px]" />
        <div className="absolute top-32 right-[4%] h-[26rem] w-[26rem] rounded-full bg-orange-500/10 blur-[120px]" />
        <div className="absolute bottom-0 left-1/2 h-64 w-64 rounded-full bg-violet-500/8 blur-[100px]" />
      </div>

      <Header lastUpdate={lastUpdate} onRefresh={loadStatic} refreshing={refreshing} connected={connected} streamMode={mode} />

      {/* Market tape — always visible */}
      {enrichedTokens?.tokens?.length ? (
        <div className="border-b border-white/6 bg-[#07111f]/60">
          <MarketTape tokens={enrichedTokens.tokens} />
        </div>
      ) : null}

      {/* Error banner */}
      {error && (
        <div className="mx-4 mt-3 rounded-2xl px-4 py-3 bg-amber-400/10 border border-amber-300/20 flex items-center gap-2">
          <AlertTriangle size={14} className="text-amber-300 flex-shrink-0" />
          <p className="text-xs text-amber-100/80">
            Backend unreachable — <code className="font-mono">cd backend && go run main.go</code>
          </p>
        </div>
      )}

      {/* Main layout: sidebar + content */}
      <div className="flex flex-1 max-w-[1440px] w-full mx-auto px-4 sm:px-6 py-5 gap-5 min-h-0">

        {/* ── Sidebar (inline — no inner component) ── */}
        <aside className={`flex-shrink-0 flex flex-col transition-all duration-300 ${sidebarCollapsed ? "w-[60px]" : "w-[220px]"} min-h-0`}>
          {!sidebarCollapsed && (
            <div className="mb-4 rounded-[16px] border border-white/8 bg-white/[0.03] p-3">
              <div className="flex items-center gap-2 mb-2">
                <span className="relative flex h-2 w-2">
                  <span className="absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75 animate-ping" />
                  <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-300" />
                </span>
                <span className="text-[10px] uppercase tracking-[0.2em] text-emerald-200">Agent Online</span>
              </div>
              <div className={`text-xs font-semibold ${controlMeta.tone}`}>{controlMeta.label}</div>
              <div className="mt-1 text-[10px] text-slate-500 leading-tight">{controlMeta.blurb}</div>
              <div className={`mt-2 inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-mono ${riskBadge(riskLevel)}`}>
                Risk {riskLevel.toFixed(0)}
              </div>
            </div>
          )}
          <nav className="flex flex-col gap-1">
            {TABS.map((tab) => {
              const active = activeTab === tab.id;
              return (
                <button key={tab.id} onClick={() => setActiveTab(tab.id)}
                  className={`group relative flex items-center gap-3 rounded-[14px] px-3 py-2.5 text-left transition-all
                    ${active ? "bg-white/8 border border-white/12 text-white" : "text-slate-400 hover:text-slate-200 hover:bg-white/4"}`}>
                  {active && <span className="absolute left-0 top-1/2 -translate-y-1/2 h-4 w-0.5 rounded-r-full bg-cyan-400" />}
                  <span className={active ? "text-cyan-300" : "text-slate-500 group-hover:text-slate-300"}>{tab.icon}</span>
                  {!sidebarCollapsed && (
                    <>
                      <span className="text-[11px] uppercase tracking-[0.14em] font-medium flex-1">{tab.label}</span>
                      {tab.badge && (
                        <span className="rounded-full bg-orange-500/20 px-1.5 py-0.5 text-[9px] uppercase tracking-[0.14em] text-orange-300 border border-orange-400/20">{tab.badge}</span>
                      )}
                    </>
                  )}
                </button>
              );
            })}
          </nav>
          <div className="mt-auto pt-4 flex flex-col gap-1">
            <Link href="/settings" className="flex items-center gap-3 rounded-[14px] px-3 py-2.5 text-slate-500 hover:text-slate-200 hover:bg-white/4 transition-all">
              <Settings size={16} />
              {!sidebarCollapsed && <span className="text-[11px] uppercase tracking-[0.14em]">Settings</span>}
            </Link>
            <button onClick={() => setSidebarCollapsed(!sidebarCollapsed)}
              className="flex items-center gap-3 rounded-[14px] px-3 py-2.5 text-slate-600 hover:text-slate-300 hover:bg-white/4 transition-all">
              <ChevronRight size={16} className={`transition-transform ${sidebarCollapsed ? "" : "rotate-180"}`} />
              {!sidebarCollapsed && <span className="text-[11px] uppercase tracking-[0.14em]">Collapse</span>}
            </button>
          </div>
        </aside>

        {/* ── Content area ── */}
        <main className="flex-1 min-w-0">
          {/* Tab header */}
          <div className="flex items-center justify-between mb-5">
            <div>
              <h2 className="font-display text-lg font-semibold text-white tracking-[-0.02em]">
                {TABS.find((t) => t.id === activeTab)?.label}
              </h2>
              <p className="text-[11px] uppercase tracking-[0.16em] text-slate-500 mt-0.5">
                {activeTab === "overview" && "Full system snapshot"}
                {activeTab === "risk"     && "Threat intelligence & pipeline"}
                {activeTab === "yield"    && "Live APY from DeFiLlama"}
                {activeTab === "ai"       && "Autonomous agent workspace"}
                {activeTab === "onchain"  && "Solana program state & execution"}
              </p>
            </div>
            <div className="flex items-center gap-2">
              <div className="hidden sm:flex items-center gap-1 rounded-[14px] border border-white/8 bg-white/[0.025] p-1">
                {TABS.map((tab) => (
                  <button key={tab.id} onClick={() => setActiveTab(tab.id)}
                    className={`rounded-[10px] px-3 py-1.5 text-[10px] uppercase tracking-[0.14em] transition-all
                      ${activeTab === tab.id ? "bg-white/10 text-white" : "text-slate-500 hover:text-slate-200"}`}>
                    {tab.label}
                  </button>
                ))}
              </div>
              <button onClick={() => void loadStatic()} disabled={refreshing}
                className="p-2 rounded-[12px] border border-white/8 bg-white/[0.025] text-slate-400 hover:text-white transition-colors disabled:opacity-40">
                <RefreshCw size={13} className={refreshing ? "animate-spin" : ""} />
              </button>
            </div>
          </div>

          {/* ── All tabs rendered, inactive hidden via CSS — no remount on data update ── */}

          {/* OVERVIEW */}
          <div className={activeTab === "overview" ? "space-y-4" : "hidden"}>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
              <StatCard label="Risk Level" value={<AnimatedNumber value={riskLevel} decimals={0} className="font-mono-data" />} sub={risk?.action ?? "Warming up…"} icon={<Activity size={15} className="text-orange-400" />} accent={riskColor(riskLevel)} />
              <StatCard label="Max Deviation" value={<AnimatedNumber value={maxDeviation} decimals={4} suffix="%" className="font-mono-data" />} sub="worst stablecoin drift" icon={<BarChart2 size={15} className="text-cyan-300" />} />
              <StatCard label="AI Decisions" value={stats?.total_decisions ?? "—"} sub={stats?.total_rebalances ? `${stats.total_rebalances} rebalances` : "loading…"} icon={<Zap size={15} className="text-violet-300" />} />
              <StatCard label="Control Mode" value={controlMeta.label} sub={controlMeta.blurb} icon={<Bot size={15} className="text-slate-300" />} accent={controlMeta.tone} />
            </div>
            {vault && (
              <div className="flex flex-wrap items-center gap-3 rounded-[16px] border border-white/8 bg-white/[0.025] px-4 py-3">
                <div className="flex items-center gap-2">
                  <span className="relative flex h-2 w-2"><span className="absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75 animate-ping" /><span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-300" /></span>
                  <span className="text-[11px] uppercase tracking-[0.18em] text-slate-300 font-semibold">Vault On-Chain</span>
                </div>
                <span className="h-3 w-px bg-white/10" /><span className="text-[11px] text-slate-400">{vault.num_tokens} tokens registered</span>
                <span className="h-3 w-px bg-white/10" /><span className="text-[11px] text-slate-400">Deposited: <span className="text-slate-200 font-mono">{(vault.total_deposited / 1e6).toLocaleString()}</span></span>
                <span className="h-3 w-px bg-white/10" /><span className="text-[11px] text-slate-400">Rebalances: <span className="text-cyan-300 font-mono">{vault.total_rebalances}</span></span>
                <span className="h-3 w-px bg-white/10" /><span className="text-[11px] text-slate-400">AI decisions: <span className="text-violet-300 font-mono">{vault.decision_count}</span></span>
                {vault.is_paused && <span className="text-[11px] text-amber-300 font-semibold">⚠ PAUSED</span>}
              </div>
            )}
            {/* Volatile Markets strip */}
            {liveData?.risk?.volatile_prices && Object.keys(liveData.risk.volatile_prices).length > 0 && (
              <div className="flex flex-wrap gap-3 rounded-[16px] border border-white/8 bg-white/[0.025] px-4 py-3">
                <span className="text-[10px] uppercase tracking-[0.18em] text-orange-300 font-semibold self-center flex items-center gap-1.5">
                  <Orbit size={11} />Volatile Markets
                </span>
                <span className="h-3 w-px bg-white/10 self-center" />
                {Object.entries(liveData.risk.volatile_prices).map(([sym, price]) => (
                  <div key={sym} className="flex items-center gap-1.5">
                    <span className="text-[10px] text-slate-500 uppercase">{sym}</span>
                    <span className="text-[11px] font-mono font-semibold text-slate-200">
                      ${price >= 1000 ? price.toLocaleString("en-US", { maximumFractionDigits: 0 }) : price.toFixed(2)}
                    </span>
                  </div>
                ))}
                <span className="h-3 w-px bg-white/10 self-center" />
                <span className="text-[10px] text-slate-600">Volatile crash risk: <span className={`font-semibold ${(liveData.risk.volatile_risk ?? 0) > 50 ? "text-rose-400" : "text-emerald-300"}`}>{(liveData.risk.volatile_risk ?? 0).toFixed(0)}/100</span></span>
              </div>
            )}

            <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
              <div className="space-y-4">
                <Card title="Risk Score" subtitle="Live AI-computed threat level"><RiskGauge value={riskLevel} /></Card>
                <Card title="Allocation" subtitle="Current portfolio split">{vault && <AllocationPie balances={vault.balances} numTokens={vault.num_tokens} />}</Card>
              </div>
              <div className="lg:col-span-2 space-y-4">
                <Card title="Price History" subtitle={`${selectedSymbol} live feed`}>
                  <div className="flex gap-2 mb-3 flex-wrap">
                    {ASSET_OPTIONS.map((asset) => (
                      <button key={asset.symbol} onClick={() => setSelectedSymbol(asset.symbol)}
                        className={`rounded-full px-3 py-1 text-[10px] uppercase tracking-[0.16em] border transition-all ${
                          selectedSymbol === asset.symbol
                            ? asset.assetType === "volatile"
                              ? "bg-orange-500/20 border-orange-400/30 text-orange-200"
                              : "bg-cyan-500/20 border-cyan-400/30 text-cyan-200"
                            : "border-white/8 text-slate-500 hover:text-slate-200"
                        }`}
                      >
                        {asset.symbol}
                      </button>
                    ))}
                  </div>
                  <PriceChart
                    data={priceHistory}
                    symbol={selectedSymbol}
                    assetType={selectedAssetMeta?.assetType}
                    color={selectedAssetMeta?.color}
                  />
                </Card>
                <Card title="Token Prices" subtitle="Pyth oracle stream">{enrichedTokens && <TokenPrices tokens={enrichedTokens.tokens} fetchedAt={enrichedTokens.fetched_at} />}</Card>
              </div>
            </div>
            <Card title="AI Decision Feed" subtitle="Recent agent verdicts"><DecisionFeed decisions={decisions} /></Card>
            <YieldPosition position={liveData?.yield_position ?? null} />
          </div>

          {/* RISK & INTEL */}
          <div className={activeTab === "risk" ? "space-y-4" : "hidden"}>
            <Card title="AI Pipeline" subtitle="Live signal flow: Oracle → Scorer → Agents → Executor">
              <PipelineVisualizer liveData={liveData} connected={connected} />
            </Card>
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              <Card title="Risk Gauge" subtitle="Windowed scorer v2 — velocity + trend + volatility">
                <RiskGauge value={riskLevel} />
                <div className="mt-4 grid grid-cols-2 gap-2">
                  {[
                    { label: "Velocity",   value: risk?.velocity?.toFixed(3) ?? "—",   color: "text-cyan-300" },
                    { label: "Trend",      value: risk?.trend?.toFixed(3) ?? "—",       color: "text-violet-300" },
                    { label: "Volatility", value: risk?.volatility?.toFixed(3) ?? "—",  color: "text-orange-300" },
                    { label: "Action",     value: risk?.action ?? "—",                  color: "text-emerald-300" },
                  ].map((item) => (
                    <div key={item.label} className="rounded-[12px] border border-white/6 bg-white/[0.02] px-3 py-2">
                      <div className="text-[10px] uppercase tracking-[0.16em] text-slate-500">{item.label}</div>
                      <div className={`mt-1 text-sm font-mono font-semibold ${item.color}`}>{item.value}</div>
                    </div>
                  ))}
                </div>
                {/* Risk breakdown: stable vs volatile */}
                <div className="mt-3 space-y-2">
                  <div className="text-[10px] uppercase tracking-[0.16em] text-slate-500 mb-1">Risk Breakdown</div>
                  {[
                    { label: "Stablecoin Peg Risk", value: risk?.stable_risk ?? 0, color: "bg-cyan-400" },
                    { label: "Volatile Crash Risk",  value: risk?.volatile_risk ?? 0, color: "bg-orange-400" },
                  ].map((item) => (
                    <div key={item.label}>
                      <div className="flex justify-between mb-0.5">
                        <span className="text-[10px] text-slate-500">{item.label}</span>
                        <span className="text-[10px] font-mono text-slate-300">{item.value.toFixed(0)}/100</span>
                      </div>
                      <div className="h-1.5 bg-white/5 rounded-full overflow-hidden">
                        <div className={`h-full rounded-full ${item.color} transition-all duration-700`} style={{ width: `${Math.min(100, item.value)}%` }} />
                      </div>
                    </div>
                  ))}
                </div>
                {/* Volatile prices */}
                {risk?.volatile_prices && Object.keys(risk.volatile_prices).length > 0 && (
                  <div className="mt-3 pt-3 border-t border-white/6">
                    <div className="text-[10px] uppercase tracking-[0.16em] text-slate-500 mb-2">Live Volatile Prices</div>
                    <div className="grid grid-cols-3 gap-2">
                      {Object.entries(risk.volatile_prices).map(([sym, price]) => (
                        <div key={sym} className="rounded-[10px] border border-white/6 bg-white/[0.02] px-2 py-1.5 text-center">
                          <div className="text-[9px] uppercase text-slate-500">{sym}</div>
                          <div className="text-xs font-mono font-semibold text-orange-300">
                            ${price >= 1000 ? (price / 1000).toFixed(1) + "k" : price.toFixed(2)}
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </Card>
              <Card title="Policy Engine" subtitle="Execution verdict">
                <div className="space-y-3">
                  {[
                    { label: "Control Mode",   value: controlMeta.label, tone: controlMeta.tone },
                    { label: "Policy Verdict", value: policyEval?.verdict ?? "pending", tone: policyEval?.verdict === "allowed" ? "text-emerald-300" : "text-amber-300" },
                    { label: "Exec Status",    value: execStatus, tone: execStatus === "executed" ? "text-emerald-300" : "text-amber-300" },
                    { label: "Yield Mode",     value: yieldLiveMode, tone: yieldLiveMode === "live" ? "text-emerald-300" : "text-slate-400" },
                    { label: "AI Model",       value: aiModel.replaceAll("-", " "), tone: "text-violet-300" },
                    { label: "Execution",      value: settings?.execution_auto_enabled ? "autonomous" : executionMode, tone: settings?.execution_auto_enabled ? "text-orange-300" : "text-slate-400" },
                  ].map((item) => (
                    <div key={item.label} className="flex items-center justify-between rounded-[12px] border border-white/6 bg-white/[0.02] px-3 py-2.5">
                      <span className="text-[11px] uppercase tracking-[0.14em] text-slate-500">{item.label}</span>
                      <span className={`text-xs font-semibold font-mono ${item.tone}`}>{item.value}</span>
                    </div>
                  ))}
                </div>
              </Card>
            </div>
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              <Card
                title={selectedAssetMeta?.assetType === "volatile" ? "Market Forecast" : "Peg Forecast"}
                subtitle={selectedAssetMeta?.assetType === "volatile" ? `${selectedSymbol} downside risk over the next 4h` : `${selectedSymbol} peg stability over the next 4h`}
              >
                <DepegForecast symbol={selectedSymbol} />
              </Card>
              <Card title="Whale Intelligence" subtitle="Large flow detection"><WhaleIntelligence /></Card>
            </div>
          </div>

          {/* YIELD */}
          <div className={activeTab === "yield" ? "space-y-4" : "hidden"}>
            <Card title="Live Yield Opportunities" subtitle="Real APY from DeFiLlama — Kamino · Marginfi · Drift"><YieldOpportunities /></Card>
            <YieldPosition position={liveData?.yield_position ?? null} />
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              <Card title="Slippage Analysis" subtitle={`Cost model for rotating ${selectedSymbol}`}>
                <SlippageAnalysis symbol={selectedSymbol} />
              </Card>
              <Card title="Treasury Payments" subtitle="Send supported treasury assets on-chain"><DAOPayments /></Card>
            </div>
          </div>

          {/* AI AGENT */}
          <div className={activeTab === "ai" ? "space-y-4" : "hidden"}>
            <Card title="Live Demo — Watch AI Decide" subtitle="Simulate a stablecoin shock or crypto crash and watch the full AI → on-chain loop">
              <LiveDemoFlow />
            </Card>
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              <Card title="AI Financial Advisor" subtitle="Portfolio-aware Claude — Cmd+K for quick actions">
                <div className="min-h-[160px] flex items-center justify-center"><AIChat /></div>
              </Card>
              <div className="space-y-4">
                <Card title="Autopilot Intent" subtitle="Natural language → vault config"><AutopilotIntent /></Card>
                <Card title="Financial Goals" subtitle="Track yield targets"><Goals /></Card>
              </div>
            </div>
            <Card title="AI Decision History" subtitle="Every agent verdict"><DecisionFeed decisions={decisions} /></Card>
          </div>

          {/* ON-CHAIN */}
          <div className={activeTab === "onchain" ? "space-y-4" : "hidden"}>
            <Card title="Vault State" subtitle="Live on-chain VaultState PDA">
              {vault ? (
                <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
                  {[
                    { label: "Tokens",      value: vault.num_tokens,                                                                               color: "text-cyan-300" },
                    { label: "Deposited",   value: `${(vault.total_deposited / 1e6).toLocaleString()} units`,                                      color: "text-white" },
                    { label: "Rebalances",  value: vault.total_rebalances,                                                                         color: "text-violet-300" },
                    { label: "AI Decisions",value: vault.decision_count,                                                                           color: "text-orange-300" },
                    { label: "Strategy",    value: vault.strategy_mode === 0 ? "SAFE" : vault.strategy_mode === 1 ? "BALANCED" : "YIELD",          color: "text-emerald-300" },
                    { label: "Status",      value: vault.is_paused ? "⚠ PAUSED" : "Active",                                                       color: vault.is_paused ? "text-amber-300" : "text-emerald-300" },
                  ].map((item) => (
                    <div key={item.label} className="rounded-[14px] border border-white/8 bg-white/[0.03] p-3">
                      <div className="text-[10px] uppercase tracking-[0.16em] text-slate-500 mb-1">{item.label}</div>
                      <div className={`text-lg font-semibold font-mono ${item.color}`}>{item.value}</div>
                    </div>
                  ))}
                </div>
              ) : <p className="text-sm text-slate-500">Loading vault state…</p>}
            </Card>
            <Card title="Execute Rebalance" subtitle="Move funds between vault token slots on-chain"><OnChainRebalance /></Card>
            <Card title="Execution Pipeline" subtitle="Stage → Swap → Settle — devnet custody flow">
              <PipelineVisualizer liveData={liveData} connected={connected} />
            </Card>
            <Card title="AI Audit Trail" subtitle="Every decision recorded on-chain via record_decision">
              <DecisionFeed decisions={decisions} />
              <div className="mt-3">
                <Link href="/audit" className="inline-flex items-center gap-1.5 rounded-full border border-white/10 px-3 py-1.5 text-[10px] uppercase tracking-[0.16em] text-slate-400 hover:text-white transition-all">
                  <ExternalLink size={11} />Full Audit Log
                </Link>
              </div>
            </Card>
          </div>

        </main>
      </div>
    </div>
  );
}
