"use client";

import { useEffect, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { TrendingUp, Clock, ExternalLink, Zap } from "lucide-react";

export interface YieldPositionData {
  protocol: string;
  token: string;
  amount: number;
  entry_apy: number;
  earned: number;
  deposited_at: number; // unix seconds
}

const PROTOCOL_URLS: Record<string, string> = {
  kamino:   "https://app.kamino.finance/lending",
  marginfi: "https://app.marginfi.com/",
  drift:    "https://app.drift.trade/earn",
};

const PROTOCOL_COLORS: Record<string, { bg: string; accent: string; text: string }> = {
  kamino:   { bg: "bg-orange-50",  accent: "bg-orange-500", text: "text-orange-600" },
  marginfi: { bg: "bg-blue-50",    accent: "bg-blue-500",   text: "text-blue-600" },
  drift:    { bg: "bg-purple-50",  accent: "bg-purple-500", text: "text-purple-600" },
};

function useLiveEarned(
  baseEarned: number,
  amount: number,
  entryAPY: number,
  depositedAt: number
) {
  const [earned, setEarned] = useState(baseEarned);
  const lastUpdateRef = useRef(Date.now() / 1000);

  useEffect(() => {
    lastUpdateRef.current = Date.now() / 1000;
    setEarned(baseEarned);
  }, [baseEarned]);

  useEffect(() => {
    const perSecond = amount * (entryAPY / 100) / (365.25 * 24 * 3600);
    const interval = setInterval(() => {
      const elapsedSinceDeposit = Date.now() / 1000 - depositedAt;
      setEarned(amount * (entryAPY / 100) / (365.25 * 24 * 3600) * elapsedSinceDeposit);
    }, 1000);
    return () => clearInterval(interval);
    // perSecond in deps to satisfy eslint but it's stable
  }, [amount, entryAPY, depositedAt]);

  return earned;
}

function formatElapsed(depositedAt: number): string {
  const s = Math.floor(Date.now() / 1000 - depositedAt);
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m ${s % 60}s`;
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  return `${h}h ${m}m`;
}

interface Props {
  position: YieldPositionData | null;
}

export function YieldPosition({ position }: Props) {
  const [elapsed, setElapsed] = useState("0s");

  useEffect(() => {
    if (!position) return;
    const t = setInterval(() => setElapsed(formatElapsed(position.deposited_at)), 1000);
    setElapsed(formatElapsed(position.deposited_at));
    return () => clearInterval(t);
  }, [position?.deposited_at]);

  const liveEarned = useLiveEarned(
    position?.earned ?? 0,
    position?.amount ?? 0,
    position?.entry_apy ?? 0,
    position?.deposited_at ?? Date.now() / 1000
  );

  const colors = PROTOCOL_COLORS[position?.protocol ?? "kamino"] ?? PROTOCOL_COLORS.kamino;
  const url = PROTOCOL_URLS[position?.protocol ?? "kamino"] ?? "#";

  return (
    <AnimatePresence mode="wait">
      {position ? (
        <motion.div
          key="active"
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -10 }}
          className={`${colors.bg} rounded-xl border border-opacity-40 p-4`}
          style={{ borderColor: colors.accent.replace("bg-", "") }}
        >
          <div className="flex items-start justify-between mb-3">
            <div className="flex items-center gap-2">
              <div className={`w-2 h-2 rounded-full ${colors.accent} animate-pulse`} />
              <span className={`text-xs font-semibold uppercase tracking-wide ${colors.text}`}>
                Active Yield Position
              </span>
            </div>
            <a
              href={url}
              target="_blank"
              rel="noopener noreferrer"
              className="text-gray-400 hover:text-gray-600 transition-colors"
            >
              <ExternalLink size={12} />
            </a>
          </div>

          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            {/* Protocol */}
            <div>
              <p className="text-[10px] text-gray-400 uppercase tracking-wide mb-0.5">Protocol</p>
              <p className="text-sm font-bold text-gray-900 capitalize">{position.protocol}</p>
            </div>

            {/* Deposited */}
            <div>
              <p className="text-[10px] text-gray-400 uppercase tracking-wide mb-0.5">Deposited</p>
              <p className="text-sm font-bold text-gray-900 font-mono-data">
                ${position.amount.toLocaleString()} {position.token}
              </p>
            </div>

            {/* APY */}
            <div>
              <p className="text-[10px] text-gray-400 uppercase tracking-wide mb-0.5">APY</p>
              <p className="text-sm font-bold text-green-600 font-mono-data">
                {position.entry_apy.toFixed(2)}%
              </p>
            </div>

            {/* Earned — live counter */}
            <div>
              <p className="text-[10px] text-gray-400 uppercase tracking-wide mb-0.5">Earned</p>
              <div className="flex items-center gap-1">
                <Zap size={11} className="text-amber-500" />
                <span className="text-sm font-bold text-amber-600 font-mono-data tabular-nums">
                  ${liveEarned.toFixed(6)}
                </span>
              </div>
            </div>
          </div>

          {/* Duration bar */}
          <div className="mt-3 flex items-center gap-2">
            <Clock size={11} className="text-gray-400" />
            <span className="text-xs text-gray-400">Running for {elapsed}</span>
            <span className="text-xs text-gray-300 mx-1">·</span>
            <span className="text-xs text-gray-400 font-mono-data">
              +${(position.amount * (position.entry_apy / 100) / (365.25 * 24)).toFixed(4)}/hr
            </span>
          </div>
        </motion.div>
      ) : (
        <motion.div
          key="empty"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="bg-gray-50 rounded-xl border border-gray-100 p-4 flex items-center gap-3"
        >
          <TrendingUp size={16} className="text-gray-300" />
          <div>
            <p className="text-sm text-gray-400">No active yield position</p>
            <p className="text-xs text-gray-300">
              Enable <code className="bg-gray-100 px-1 rounded text-gray-500">YIELD_ENABLED=true</code> to auto-deposit on OPTIMIZE signal
            </p>
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
