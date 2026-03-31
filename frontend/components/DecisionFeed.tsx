"use client";

import { DecisionRow } from "@/lib/api";
import { Shield, TrendingUp, Minus, ExternalLink } from "lucide-react";

function ActionBadge({ action }: { action: string }) {
  const map: Record<string, { color: string; bg: string; icon: typeof Shield }> = {
    HOLD:     { color: "#6b7280", bg: "#f3f4f6", icon: Minus },
    PROTECT:  { color: "#dc2626", bg: "#fef2f2", icon: Shield },
    OPTIMIZE: { color: "#16a34a", bg: "#f0fdf4", icon: TrendingUp },
  };
  const cfg = map[action] ?? map.HOLD;
  const Icon = cfg.icon;
  return (
    <span
      className="inline-flex items-center gap-1 text-xs font-semibold px-2 py-0.5 rounded-full"
      style={{ color: cfg.color, background: cfg.bg }}
    >
      <Icon size={10} />
      {action}
    </span>
  );
}

interface Props {
  decisions: DecisionRow[];
}

export function DecisionFeed({ decisions }: Props) {
  if (!decisions || decisions.length === 0) {
    return (
      <div className="text-sm text-gray-400 py-6 text-center">
        No AI decisions yet — pipeline is warming up
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {decisions.map((d) => (
        <div key={d.id} className="p-3 rounded-lg bg-gray-50 border border-gray-100">
          <div className="flex items-start justify-between gap-2 mb-1.5">
            <ActionBadge action={d.action} />
            <div className="flex items-center gap-2 flex-shrink-0">
              <span className="text-xs text-gray-400">
                conf {d.confidence}%
              </span>
              <span className="text-xs text-gray-300">·</span>
              <span className="text-xs text-gray-400">
                risk {d.risk_level.toFixed(0)}
              </span>
              {d.exec_sig && (
                <a
                  href={`https://explorer.solana.com/tx/${d.exec_sig}?cluster=devnet`}
                  target="_blank"
                  rel="noreferrer"
                  className="text-blue-500 hover:text-blue-600"
                >
                  <ExternalLink size={11} />
                </a>
              )}
            </div>
          </div>
          <p className="text-xs text-gray-700 leading-relaxed">{d.rationale}</p>
          <p className="text-[11px] text-gray-400 mt-1.5">
            {new Date(d.ts * 1000).toLocaleString()}
            {d.action !== "HOLD" && ` · slot ${d.from_index} → ${d.to_index}`}
          </p>
        </div>
      ))}
    </div>
  );
}
