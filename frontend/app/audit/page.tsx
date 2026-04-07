"use client";

import { useEffect, useState, useCallback } from "react";
import { motion } from "framer-motion";
import { Header } from "@/components/Header";
import {
  Bot,
  ExternalLink,
  Shield,
  TrendingUp,
  Activity,
  CheckCircle2,
  Clock,
  Zap,
  Brain,
  RefreshCw,
} from "lucide-react";

const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

interface DecisionRow {
  id: number;
  ts: number;
  action: string;
  from_index: number;
  to_index: number;
  suggested_fraction: number;
  confidence: number;
  rationale: string;
  risk_analysis: string;
  yield_analysis: string;
  risk_level: number;
  exec_sig: string;
}

interface HistoryStats {
  total_decisions: number;
  total_rebalances: number;
  total_risk_events: number;
  avg_risk_level: number;
  last_decision_ts?: number;
}

function explorerUrl(sig: string, rpc = "") {
  if (!sig) return "";
  const isMain = rpc.includes("mainnet") || rpc.includes("helius") || rpc.includes("quiknode");
  return `https://explorer.solana.com/tx/${sig}${isMain ? "" : "?cluster=devnet"}`;
}

function actionMeta(action: string) {
  if (action === "PROTECT")
    return { label: "PROTECT", bg: "bg-red-400/10", border: "border-red-400/20", text: "text-red-300", icon: <Shield size={12} /> };
  if (action === "OPTIMIZE")
    return { label: "OPTIMIZE", bg: "bg-cyan-400/10", border: "border-cyan-400/20", text: "text-cyan-300", icon: <TrendingUp size={12} /> };
  return { label: "HOLD", bg: "bg-slate-400/8", border: "border-white/10", text: "text-slate-400", icon: <Activity size={12} /> };
}

function riskColor(level: number) {
  if (level >= 80) return "text-red-400";
  if (level >= 50) return "text-orange-400";
  if (level >= 30) return "text-amber-300";
  return "text-emerald-400";
}

function timeAgo(ts: number) {
  const diff = Math.floor(Date.now() / 1000 - ts);
  if (diff < 60) return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return new Date(ts * 1000).toLocaleDateString();
}

export default function AuditPage() {
  const [decisions, setDecisions] = useState<DecisionRow[]>([]);
  const [stats, setStats] = useState<HistoryStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState<number | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [dRes, sRes] = await Promise.all([
        fetch(`${BASE}/history/decisions?limit=50`),
        fetch(`${BASE}/history/stats`),
      ]);
      if (dRes.ok) {
        const d = await dRes.json();
        setDecisions(d.decisions ?? []);
      }
      if (sRes.ok) {
        setStats(await sRes.json());
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const onChainCount = decisions.filter((d) => !!d.exec_sig).length;

  return (
    <div className="app-shell min-h-screen">
      <Header connected={false} />

      <main className="max-w-5xl mx-auto px-4 sm:px-6 py-8 space-y-6">

        {/* Page title */}
        <motion.div
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          className="space-y-1"
        >
          <div className="flex items-center justify-between">
            <div>
              <h1 className="font-display text-2xl sm:text-3xl font-bold text-slate-50 flex items-center gap-2">
                <Brain size={22} className="text-purple-400" />
                AI Audit Trail
              </h1>
              <p className="text-sm text-slate-400 mt-1">
                Every AI decision — verifiable on-chain via Solana Explorer
              </p>
            </div>
            <button
              onClick={load}
              disabled={loading}
              className="flex items-center gap-1.5 rounded-full border border-white/10 px-3 py-1.5 text-xs text-slate-400 hover:text-slate-200 hover:border-white/20 transition-all"
            >
              <RefreshCw size={12} className={loading ? "animate-spin" : ""} />
              Refresh
            </button>
          </div>
        </motion.div>

        {/* Stats row */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.05 }}
          className="grid grid-cols-2 sm:grid-cols-4 gap-3"
        >
          {[
            {
              icon: <Bot size={14} className="text-purple-400" />,
              label: "Total Decisions",
              value: stats?.total_decisions ?? decisions.length,
            },
            {
              icon: <CheckCircle2 size={14} className="text-emerald-400" />,
              label: "On-Chain Proofs",
              value: onChainCount,
            },
            {
              icon: <Zap size={14} className="text-orange-400" />,
              label: "Avg Risk Level",
              value: stats?.avg_risk_level ? `${stats.avg_risk_level.toFixed(0)}/100` : "—",
            },
            {
              icon: <Activity size={14} className="text-cyan-400" />,
              label: "Risk Events",
              value: stats?.total_risk_events ?? "—",
            },
          ].map((s) => (
            <div key={s.label} className="panel-surface rounded-[18px] p-4">
              <div className="flex items-center gap-1.5 text-[10px] uppercase tracking-[0.18em] text-slate-500 mb-2">
                {s.icon}
                {s.label}
              </div>
              <div className="text-2xl font-display text-white">{s.value}</div>
            </div>
          ))}
        </motion.div>

        {/* AI→Solana chain explanation */}
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 0.1 }}
          className="panel-surface rounded-[18px] p-4"
        >
          <div className="flex items-center gap-2 mb-3">
            <Zap size={13} className="text-orange-400" />
            <span className="text-[10px] font-bold uppercase tracking-[0.22em] text-orange-300">
              How AI decisions reach Solana
            </span>
          </div>
          <div className="flex items-center gap-2 overflow-x-auto pb-1 text-[11px] font-mono text-slate-400">
            {[
              "Pyth price feed",
              "WindowedScorer (v2)",
              "3 Claude agents",
              "FinalDecision",
              "record_decision ix",
              "PDA on Solana",
            ].map((step, i, arr) => (
              <span key={step} className="flex items-center gap-2 flex-shrink-0">
                <span className="text-slate-200">{step}</span>
                {i < arr.length - 1 && <span className="text-white/20">→</span>}
              </span>
            ))}
          </div>
          <p className="text-[10px] text-slate-500 mt-2">
            Each non-HOLD decision creates a <code className="text-cyan-400">decision_log</code> PDA seeded by{" "}
            <code className="text-cyan-400">[&quot;decision&quot;, vault, sequence]</code>.
            All decisions are immutable and publicly verifiable.
          </p>
        </motion.div>

        {/* Decision list */}
        <div className="space-y-2">
          {loading && decisions.length === 0 && (
            <div className="text-center py-12 text-slate-500 text-sm">Loading decisions…</div>
          )}
          {!loading && decisions.length === 0 && (
            <div className="text-center py-12 space-y-3">
              <Bot size={32} className="text-slate-600 mx-auto" />
              <p className="text-slate-500 text-sm">No decisions recorded yet.</p>
              <p className="text-slate-600 text-xs">Use the Depeg Simulator on the dashboard to trigger the AI pipeline.</p>
            </div>
          )}

          {decisions.map((d, idx) => {
            const meta = actionMeta(d.action);
            const isOpen = expanded === d.id;

            return (
              <motion.div
                key={d.id}
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: idx * 0.03 }}
                className={`panel-surface rounded-[18px] border overflow-hidden ${meta.border} cursor-pointer`}
                onClick={() => setExpanded(isOpen ? null : d.id)}
              >
                {/* Row header */}
                <div className="flex items-center gap-3 px-4 py-3">
                  {/* Sequence badge */}
                  <div className="text-[10px] font-mono text-slate-500 w-8 flex-shrink-0">
                    #{d.id}
                  </div>

                  {/* Action badge */}
                  <div className={`flex items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide flex-shrink-0 ${meta.bg} ${meta.border} ${meta.text}`}>
                    {meta.icon}
                    {meta.label}
                  </div>

                  {/* Risk level */}
                  <div className={`text-xs font-mono font-semibold flex-shrink-0 ${riskColor(d.risk_level)}`}>
                    Risk {d.risk_level.toFixed(0)}
                  </div>

                  {/* Confidence */}
                  <div className="text-xs text-slate-500 flex-shrink-0">
                    conf {d.confidence}%
                  </div>

                  {/* Rationale (truncated) */}
                  <div className="text-xs text-slate-400 truncate flex-1 min-w-0">
                    {d.rationale}
                  </div>

                  {/* Time + on-chain indicator */}
                  <div className="flex items-center gap-2 flex-shrink-0">
                    {d.exec_sig && (
                      <CheckCircle2 size={12} className="text-emerald-400" />
                    )}
                    <div className="flex items-center gap-1 text-[10px] text-slate-500">
                      <Clock size={10} />
                      {timeAgo(d.ts)}
                    </div>
                  </div>
                </div>

                {/* Expanded detail */}
                {isOpen && (
                  <motion.div
                    initial={{ opacity: 0, height: 0 }}
                    animate={{ opacity: 1, height: "auto" }}
                    className="px-4 pb-4 space-y-3 border-t border-white/6"
                  >
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-3">
                      {d.risk_analysis && (
                        <div className="bg-white/3 rounded-xl p-3">
                          <div className="text-[9px] uppercase tracking-[0.18em] text-red-400 mb-1.5 flex items-center gap-1">
                            <Shield size={9} /> Risk Agent
                          </div>
                          <p className="text-xs text-slate-300 leading-relaxed">{d.risk_analysis}</p>
                        </div>
                      )}
                      {d.yield_analysis && (
                        <div className="bg-white/3 rounded-xl p-3">
                          <div className="text-[9px] uppercase tracking-[0.18em] text-cyan-400 mb-1.5 flex items-center gap-1">
                            <TrendingUp size={9} /> Yield Agent
                          </div>
                          <p className="text-xs text-slate-300 leading-relaxed">{d.yield_analysis}</p>
                        </div>
                      )}
                    </div>

                    {d.from_index >= 0 && d.to_index >= 0 && d.action !== "HOLD" && (
                      <div className="text-[10px] text-slate-500 font-mono">
                        Rebalance: slot {d.from_index} → slot {d.to_index} · fraction {(d.suggested_fraction * 100).toFixed(1)}%
                      </div>
                    )}

                    {d.exec_sig ? (
                      <a
                        href={explorerUrl(d.exec_sig)}
                        target="_blank"
                        rel="noopener noreferrer"
                        onClick={(e) => e.stopPropagation()}
                        className="flex items-center gap-2 bg-emerald-400/8 border border-emerald-400/20 rounded-xl px-3 py-2 text-xs font-mono text-emerald-300 hover:text-emerald-200 transition-colors"
                      >
                        <CheckCircle2 size={12} />
                        <span className="truncate">On-chain proof: {d.exec_sig}</span>
                        <ExternalLink size={10} className="flex-shrink-0 ml-auto" />
                      </a>
                    ) : (
                      <div className="text-[10px] text-slate-600 font-mono italic">
                        No on-chain signature — decision recorded in DB only (HOLD or auto_execute=false)
                      </div>
                    )}
                  </motion.div>
                )}
              </motion.div>
            );
          })}
        </div>
      </main>
    </div>
  );
}
