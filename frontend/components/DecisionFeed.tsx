"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { DecisionRow } from "@/lib/api";
import {
  Shield, TrendingUp, Minus, ExternalLink,
  ChevronDown, Brain, BarChart2, Zap,
} from "lucide-react";

function ActionBadge({ action }: { action: string }) {
  const map: Record<string, { color: string; bg: string; border: string; icon: React.ElementType; dot: string }> = {
    HOLD:     { color: "#6b7280", bg: "#f3f4f6", border: "#e5e7eb", icon: Minus,      dot: "bg-gray-400" },
    PROTECT:  { color: "#dc2626", bg: "#fef2f2", border: "#fecaca", icon: Shield,     dot: "bg-red-500" },
    OPTIMIZE: { color: "#16a34a", bg: "#f0fdf4", border: "#bbf7d0", icon: TrendingUp, dot: "bg-green-500" },
  };
  const cfg = map[action] ?? map.HOLD;
  const Icon = cfg.icon;
  return (
    <span
      className="inline-flex items-center gap-1.5 text-xs font-semibold px-2.5 py-1 rounded-full border"
      style={{ color: cfg.color, background: cfg.bg, borderColor: cfg.border }}
    >
      <span className={`w-1.5 h-1.5 rounded-full ${cfg.dot}`} />
      <Icon size={10} />
      {action}
    </span>
  );
}

function ConfidenceBar({ value }: { value: number }) {
  const color = value >= 75 ? "bg-green-400" : value >= 50 ? "bg-yellow-400" : "bg-red-400";
  return (
    <div className="flex items-center gap-2">
      <div className="flex-1 h-1 bg-gray-100 rounded-full overflow-hidden">
        <motion.div
          initial={{ width: 0 }}
          animate={{ width: `${value}%` }}
          transition={{ duration: 0.6, ease: "easeOut" }}
          className={`h-full rounded-full ${color}`}
        />
      </div>
      <span className="text-[10px] font-mono text-gray-500 w-8 text-right">{value}%</span>
    </div>
  );
}

function DecisionCard({ d }: { d: DecisionRow }) {
  const [expanded, setExpanded] = useState(false);

  const isAction = d.action !== "HOLD";
  const ts = new Date(d.ts * 1000);
  const timeStr = ts.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
  const dateStr = ts.toLocaleDateString([], { month: "short", day: "numeric" });

  return (
    <motion.div
      layout
      className={`rounded-xl border overflow-hidden transition-colors ${
        isAction ? "border-orange-100 bg-orange-50/30" : "border-gray-100 bg-gray-50"
      }`}
    >
      {/* Top row */}
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full text-left px-3.5 py-3"
      >
        <div className="flex items-start justify-between gap-2 mb-2">
          <ActionBadge action={d.action} />
          <div className="flex items-center gap-2 flex-shrink-0">
            {d.exec_sig && (
              <a
                href={`https://explorer.solana.com/tx/${d.exec_sig}?cluster=devnet`}
                target="_blank" rel="noreferrer"
                onClick={e => e.stopPropagation()}
                className="text-blue-400 hover:text-blue-600 transition-colors"
              >
                <ExternalLink size={11} />
              </a>
            )}
            <span className="text-[10px] text-gray-400 font-mono">{timeStr}</span>
            <motion.div animate={{ rotate: expanded ? 180 : 0 }} transition={{ duration: 0.2 }}>
              <ChevronDown size={13} className="text-gray-400" />
            </motion.div>
          </div>
        </div>

        {/* Rationale preview */}
        <p className="text-xs text-gray-700 leading-relaxed line-clamp-2">{d.rationale}</p>

        {/* Confidence + meta */}
        <div className="mt-2 flex items-center gap-3">
          <div className="flex-1">
            <ConfidenceBar value={d.confidence} />
          </div>
          <span className="text-[10px] text-gray-400 flex-shrink-0">
            risk {d.risk_level.toFixed(0)}
            {isAction && ` · slot ${d.from_index}→${d.to_index}`}
          </span>
        </div>
      </button>

      {/* Expanded detail */}
      <AnimatePresence initial={false}>
        {expanded && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.22, ease: "easeInOut" }}
            className="overflow-hidden"
          >
            <div className="px-3.5 pb-3.5 space-y-2.5 border-t border-gray-100 pt-3">
              {/* Risk analysis */}
              <div className="flex gap-2">
                <div className="w-5 h-5 rounded bg-red-50 border border-red-100 flex items-center justify-center flex-shrink-0 mt-0.5">
                  <BarChart2 size={10} className="text-red-400" />
                </div>
                <div>
                  <p className="text-[10px] font-semibold text-gray-400 uppercase tracking-wide mb-0.5">Risk Analysis</p>
                  <p className="text-xs text-gray-700 leading-relaxed">{d.risk_analysis}</p>
                </div>
              </div>

              {/* Yield analysis */}
              <div className="flex gap-2">
                <div className="w-5 h-5 rounded bg-green-50 border border-green-100 flex items-center justify-center flex-shrink-0 mt-0.5">
                  <Zap size={10} className="text-green-500" />
                </div>
                <div>
                  <p className="text-[10px] font-semibold text-gray-400 uppercase tracking-wide mb-0.5">Yield Analysis</p>
                  <p className="text-xs text-gray-700 leading-relaxed">{d.yield_analysis}</p>
                </div>
              </div>

              {/* Strategy rationale */}
              <div className="flex gap-2">
                <div className="w-5 h-5 rounded bg-purple-50 border border-purple-100 flex items-center justify-center flex-shrink-0 mt-0.5">
                  <Brain size={10} className="text-purple-400" />
                </div>
                <div>
                  <p className="text-[10px] font-semibold text-gray-400 uppercase tracking-wide mb-0.5">Strategy Rationale</p>
                  <p className="text-xs text-gray-700 leading-relaxed">{d.rationale}</p>
                </div>
              </div>

              {/* Footer */}
              <div className="flex items-center justify-between pt-1 border-t border-gray-100">
                <span className="text-[10px] text-gray-400">{dateStr} · {timeStr}</span>
                {d.exec_sig && d.exec_sig !== "" ? (
                  <a
                    href={`https://explorer.solana.com/tx/${d.exec_sig}?cluster=devnet`}
                    target="_blank" rel="noreferrer"
                    className="flex items-center gap-1 text-[10px] text-blue-500 hover:text-blue-700 font-mono transition-colors"
                  >
                    <ExternalLink size={9} />
                    {d.exec_sig.slice(0, 16)}…
                  </a>
                ) : (
                  <span className="text-[10px] text-gray-300">No on-chain tx</span>
                )}
              </div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  );
}

interface Props {
  decisions: DecisionRow[];
}

export function DecisionFeed({ decisions }: Props) {
  if (!decisions || decisions.length === 0) {
    return (
      <div className="text-sm text-gray-400 py-8 text-center flex flex-col items-center gap-2">
        <Brain size={24} className="text-gray-200" />
        No AI decisions yet — pipeline is warming up
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {decisions.map(d => <DecisionCard key={d.id} d={d} />)}
    </div>
  );
}
