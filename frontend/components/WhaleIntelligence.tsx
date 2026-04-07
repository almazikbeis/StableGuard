"use client";

import { useState, useEffect, useCallback } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  Fish,
  TrendingDown,
  BarChart2,
  Droplets,
  Zap,
  RefreshCw,
  Loader2,
  ChevronDown,
  Brain,
} from "lucide-react";

const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

interface WhaleAlert {
  dex_id: string;
  token: string;
  signal: "sell_pressure" | "low_liquidity" | "price_drop" | "volume_spike";
  severity: "low" | "medium" | "high";
  value: number;
  description: string;
}

interface WhaleData {
  score: number;
  alerts: WhaleAlert[];
  summary: string;
  updated_at: number;
}

const SIGNAL_META = {
  sell_pressure: { icon: TrendingDown, label: "Sell Pressure", color: "text-red-400" },
  low_liquidity: { icon: Droplets, label: "Low Liquidity", color: "text-amber-400" },
  price_drop: { icon: TrendingDown, label: "Price Drop", color: "text-red-500" },
  volume_spike: { icon: BarChart2, label: "Volume Spike", color: "text-orange-400" },
} as const;

const SEV_STYLE = {
  low: "border-amber-400/20 bg-amber-400/6 text-amber-300",
  medium: "border-orange-400/20 bg-orange-400/8 text-orange-300",
  high: "border-red-400/25 bg-red-400/10 text-red-300",
} as const;

function ScoreRing({ score }: { score: number }) {
  const color =
    score < 20 ? "#22c55e" :
    score < 45 ? "#f59e0b" :
    score < 70 ? "#f97316" :
                 "#ef4444";
  const r = 22;
  const circ = 2 * Math.PI * r;
  const fill = (score / 100) * circ;

  return (
    <div className="relative w-14 h-14 flex items-center justify-center flex-shrink-0">
      <svg width="56" height="56" className="-rotate-90">
        <circle cx="28" cy="28" r={r} fill="none" stroke="rgba(255,255,255,0.06)" strokeWidth="4" />
        <motion.circle
          cx="28" cy="28" r={r}
          fill="none"
          stroke={color}
          strokeWidth="4"
          strokeLinecap="round"
          strokeDasharray={circ}
          initial={{ strokeDashoffset: circ }}
          animate={{ strokeDashoffset: circ - fill }}
          transition={{ duration: 0.9, ease: "easeOut" }}
        />
      </svg>
      <span className="absolute text-sm font-display font-bold" style={{ color }}>
        {score.toFixed(0)}
      </span>
    </div>
  );
}

export function WhaleIntelligence() {
  const [data, setData] = useState<WhaleData | null>(null);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState(false);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch(`${BASE}/onchain/whales`);
      if (res.ok) setData(await res.json());
    } catch {
      // silent
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
    const id = setInterval(fetchData, 2 * 60 * 1000);
    return () => clearInterval(id);
  }, [fetchData]);

  const highAlerts = data?.alerts.filter((a) => a.severity === "high").length ?? 0;
  const visibleAlerts = expanded ? data?.alerts : data?.alerts.slice(0, 3);

  return (
    <div className="panel-surface rounded-[24px] p-5">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <div className="relative">
            <Fish size={14} className="text-cyan-400" />
            {highAlerts > 0 && (
              <span className="absolute -top-1.5 -right-1.5 w-3.5 h-3.5 rounded-full bg-red-500 text-white text-[7px] font-bold flex items-center justify-center">
                {highAlerts}
              </span>
            )}
          </div>
          <div>
            <h3 className="font-display font-bold text-slate-50 text-sm">Whale Intelligence</h3>
            <p className="text-[10px] text-slate-500">DexScreener · Solana mainnet · feeds AI agents</p>
          </div>
        </div>
        <button
          onClick={fetchData}
          disabled={loading}
          className="p-1.5 rounded-lg text-slate-500 hover:text-slate-300 transition-colors disabled:opacity-40"
        >
          {loading ? <Loader2 size={12} className="animate-spin" /> : <RefreshCw size={12} />}
        </button>
      </div>

      {/* AI integration badge */}
      <div className="flex items-center gap-1.5 mb-4 bg-purple-400/8 border border-purple-400/15 rounded-xl px-3 py-2">
        <Brain size={10} className="text-purple-400 flex-shrink-0" />
        <span className="text-[10px] text-purple-300">
          Whale score feeds the Strategy AI agent — high sell pressure → higher PROTECT urgency
        </span>
      </div>

      {/* Score + summary */}
      {data && (
        <div className="flex items-center gap-3 mb-4">
          <ScoreRing score={data.score} />
          <div className="flex-1 min-w-0">
            <p className="text-xs font-semibold text-slate-200 mb-0.5">On-chain risk score</p>
            <p className="text-xs text-slate-400 leading-relaxed">{data.summary}</p>
            {data.updated_at > 0 && (
              <p className="text-[10px] text-slate-600 mt-1 font-mono">
                {new Date(data.updated_at * 1000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
              </p>
            )}
          </div>
        </div>
      )}

      {loading && !data && (
        <div className="h-16 flex items-center justify-center">
          <Loader2 size={14} className="animate-spin text-slate-600" />
        </div>
      )}

      {/* No alerts */}
      {data && data.alerts.length === 0 && (
        <div className="rounded-xl border border-emerald-400/15 bg-emerald-400/6 px-3 py-2.5 flex items-center gap-2">
          <Zap size={12} className="text-emerald-400 flex-shrink-0" />
          <p className="text-xs text-emerald-300">No whale activity — liquidity stable</p>
        </div>
      )}

      {/* Alert feed */}
      <AnimatePresence>
        {visibleAlerts && visibleAlerts.length > 0 && (
          <motion.div className="space-y-2">
            {visibleAlerts.map((a, i) => {
              const meta = SIGNAL_META[a.signal];
              const Icon = meta.icon;
              return (
                <motion.div
                  key={`${a.dex_id}-${a.signal}-${i}`}
                  layout
                  initial={{ opacity: 0, x: -6 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ delay: i * 0.04 }}
                  className={`rounded-xl border px-3 py-2.5 flex items-start gap-2 ${SEV_STYLE[a.severity]}`}
                >
                  <Icon size={12} className={`flex-shrink-0 mt-0.5 ${meta.color}`} />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5 mb-0.5 flex-wrap">
                      <span className="text-[9px] font-bold uppercase tracking-wide opacity-70">{meta.label}</span>
                      <span className="text-[9px] font-mono bg-white/8 rounded px-1">{a.dex_id}</span>
                      <span className={`text-[8px] font-bold uppercase px-1 py-0.5 rounded ${
                        a.severity === "high" ? "bg-red-400/20" :
                        a.severity === "medium" ? "bg-orange-400/20" : "bg-amber-400/20"
                      }`}>{a.severity}</span>
                    </div>
                    <p className="text-[11px] leading-snug">{a.description}</p>
                  </div>
                </motion.div>
              );
            })}
          </motion.div>
        )}
      </AnimatePresence>

      {/* Expand toggle */}
      {data && data.alerts.length > 3 && (
        <button
          onClick={() => setExpanded(!expanded)}
          className="w-full flex items-center justify-center gap-1 text-xs text-slate-500 hover:text-slate-300 transition-colors py-2 mt-1"
        >
          <motion.div animate={{ rotate: expanded ? 180 : 0 }} transition={{ duration: 0.2 }}>
            <ChevronDown size={12} />
          </motion.div>
          {expanded ? "Show less" : `+${data.alerts.length - 3} more signals`}
        </button>
      )}
    </div>
  );
}
