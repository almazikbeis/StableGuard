"use client";

import { useState, useEffect, useCallback } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Fish, TrendingDown, BarChart2, Droplets, Zap, RefreshCw, Loader2, ChevronDown } from "lucide-react";

const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

interface WhaleAlert {
  dex_id:      string;
  token:       string;
  signal:      "sell_pressure" | "low_liquidity" | "price_drop" | "volume_spike";
  severity:    "low" | "medium" | "high";
  value:       number;
  description: string;
}

interface WhaleData {
  score:      number;
  alerts:     WhaleAlert[];
  summary:    string;
  updated_at: number;
}

const SIGNAL_META: Record<WhaleAlert["signal"], { icon: React.ElementType; label: string; color: string }> = {
  sell_pressure: { icon: TrendingDown, label: "Sell Pressure", color: "text-red-500"    },
  low_liquidity: { icon: Droplets,    label: "Low Liquidity",  color: "text-yellow-500" },
  price_drop:    { icon: TrendingDown, label: "Price Drop",    color: "text-red-600"    },
  volume_spike:  { icon: BarChart2,   label: "Volume Spike",  color: "text-orange-500" },
};

const SEVERITY_STYLE: Record<WhaleAlert["severity"], string> = {
  low:    "bg-yellow-50 border-yellow-200 text-yellow-700",
  medium: "bg-orange-50 border-orange-200 text-orange-700",
  high:   "bg-red-50   border-red-200    text-red-700",
};

function ScoreRing({ score }: { score: number }) {
  const color =
    score < 20  ? "#22c55e" :
    score < 45  ? "#f59e0b" :
    score < 70  ? "#f97316" :
                  "#ef4444";
  const r = 28;
  const circ = 2 * Math.PI * r;
  const fill = (score / 100) * circ;

  return (
    <div className="relative w-16 h-16 flex items-center justify-center">
      <svg width="64" height="64" className="-rotate-90">
        <circle cx="32" cy="32" r={r} fill="none" stroke="#f3f4f6" strokeWidth="5" />
        <motion.circle
          cx="32" cy="32" r={r}
          fill="none"
          stroke={color}
          strokeWidth="5"
          strokeLinecap="round"
          strokeDasharray={circ}
          initial={{ strokeDashoffset: circ }}
          animate={{ strokeDashoffset: circ - fill }}
          transition={{ duration: 0.8, ease: "easeOut" }}
        />
      </svg>
      <span className="absolute text-sm font-display font-extrabold" style={{ color }}>
        {score.toFixed(0)}
      </span>
    </div>
  );
}

function AlertCard({ alert }: { alert: WhaleAlert }) {
  const meta = SIGNAL_META[alert.signal];
  const Icon = meta.icon;
  const sev = SEVERITY_STYLE[alert.severity];

  return (
    <motion.div
      layout
      initial={{ opacity: 0, x: -8 }}
      animate={{ opacity: 1, x: 0 }}
      className={`rounded-lg border px-3 py-2.5 flex items-start gap-2.5 ${sev}`}
    >
      <Icon size={13} className="flex-shrink-0 mt-0.5" />
      <div className="min-w-0">
        <div className="flex items-center gap-1.5 mb-0.5">
          <span className="text-[10px] font-bold uppercase tracking-wide opacity-70">{meta.label}</span>
          <span className="text-[10px] font-mono bg-white/40 rounded px-1">{alert.dex_id}</span>
          <span className={`text-[9px] font-bold uppercase px-1 py-0.5 rounded ${
            alert.severity === "high" ? "bg-red-200" : alert.severity === "medium" ? "bg-orange-200" : "bg-yellow-200"
          }`}>{alert.severity}</span>
        </div>
        <p className="text-[11px] leading-snug">{alert.description}</p>
      </div>
    </motion.div>
  );
}

export function WhaleIntelligence() {
  const [data, setData]       = useState<WhaleData | null>(null);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState(false);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res  = await window.fetch(`${BASE}/onchain/whales`);
      const json = await res.json();
      setData(json);
    } catch {
      setData(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
    const id = setInterval(fetchData, 2 * 60 * 1000); // refresh every 2 min
    return () => clearInterval(id);
  }, [fetchData]);

  const highAlerts = data?.alerts.filter(a => a.severity === "high").length ?? 0;
  const visibleAlerts = expanded ? data?.alerts : data?.alerts.slice(0, 3);

  return (
    <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
      {/* Header */}
      <div className="px-4 pt-4 pb-3 border-b border-gray-100 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <div className="w-7 h-7 rounded-lg bg-blue-50 border border-blue-100 flex items-center justify-center relative">
            <Fish size={13} className="text-blue-500" />
            {highAlerts > 0 && (
              <span className="absolute -top-1 -right-1 w-3.5 h-3.5 rounded-full bg-red-500 text-white text-[8px] font-bold flex items-center justify-center">
                {highAlerts}
              </span>
            )}
          </div>
          <div>
            <span className="text-sm font-semibold text-gray-900">Whale Intelligence</span>
            <span className="text-xs text-gray-400 ml-2">DexScreener · Solana</span>
          </div>
        </div>
        <button
          onClick={fetchData}
          disabled={loading}
          className="p-1.5 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors disabled:opacity-40"
        >
          {loading ? <Loader2 size={13} className="animate-spin" /> : <RefreshCw size={13} />}
        </button>
      </div>

      <div className="p-4 space-y-4">
        {/* Score + summary */}
        {data && (
          <div className="flex items-center gap-4">
            <ScoreRing score={data.score} />
            <div className="flex-1">
              <p className="text-xs font-semibold text-gray-700 mb-0.5">Whale Risk Score</p>
              <p className="text-xs text-gray-500 leading-relaxed">{data.summary}</p>
              {data.updated_at > 0 && (
                <p className="text-[10px] text-gray-400 mt-1">
                  Updated {new Date(data.updated_at * 1000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
                </p>
              )}
            </div>
          </div>
        )}

        {loading && !data && (
          <div className="h-20 flex items-center justify-center">
            <Loader2 size={16} className="animate-spin text-gray-300" />
          </div>
        )}

        {/* Alert feed */}
        {data && data.alerts.length === 0 && (
          <div className="rounded-xl bg-green-50 border border-green-200 px-3 py-3 flex items-center gap-2">
            <Zap size={13} className="text-green-500 flex-shrink-0" />
            <p className="text-xs text-green-700">No whale activity detected — liquidity stable</p>
          </div>
        )}

        <AnimatePresence>
          {visibleAlerts && visibleAlerts.length > 0 && (
            <motion.div className="space-y-2">
              {visibleAlerts.map((a, i) => (
                <AlertCard key={`${a.dex_id}-${a.signal}-${i}`} alert={a} />
              ))}
            </motion.div>
          )}
        </AnimatePresence>

        {/* Expand toggle */}
        {data && data.alerts.length > 3 && (
          <button
            onClick={() => setExpanded(!expanded)}
            className="w-full flex items-center justify-center gap-1 text-xs text-gray-400 hover:text-gray-600 transition-colors py-1"
          >
            <motion.div animate={{ rotate: expanded ? 180 : 0 }} transition={{ duration: 0.2 }}>
              <ChevronDown size={13} />
            </motion.div>
            {expanded ? "Show less" : `Show ${data.alerts.length - 3} more signals`}
          </button>
        )}
      </div>
    </div>
  );
}
