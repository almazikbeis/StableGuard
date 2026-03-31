"use client";

import { useEffect, useState, useCallback } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { TrendingUp, ExternalLink, RefreshCw, Wifi, WifiOff, Trophy } from "lucide-react";
import { api, YieldOpportunity } from "@/lib/api";

const PROTOCOL_COLORS: Record<string, { bg: string; text: string; dot: string }> = {
  kamino:   { bg: "bg-orange-50",  text: "text-orange-600",  dot: "bg-orange-400" },
  marginfi: { bg: "bg-blue-50",    text: "text-blue-600",    dot: "bg-blue-400" },
  drift:    { bg: "bg-purple-50",  text: "text-purple-600",  dot: "bg-purple-400" },
};

function APYBar({ value, max }: { value: number; max: number }) {
  const pct = Math.min(100, (value / max) * 100);
  return (
    <div className="w-24 h-1.5 bg-gray-100 rounded-full overflow-hidden">
      <motion.div
        initial={{ width: 0 }}
        animate={{ width: `${pct}%` }}
        transition={{ duration: 0.6, ease: "easeOut" }}
        className="h-full bg-green-400 rounded-full"
      />
    </div>
  );
}

interface Props {
  className?: string;
}

export function YieldOpportunities({ className = "" }: Props) {
  const [opps, setOpps] = useState<YieldOpportunity[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [hasLive, setHasLive] = useState(false);
  const [filter, setFilter] = useState<string>("ALL");

  const load = useCallback(async (isRefresh = false) => {
    if (isRefresh) setRefreshing(true);
    try {
      const data = await api.yieldOpportunities();
      setOpps(data.opportunities ?? []);
      setHasLive((data.opportunities ?? []).some((o) => o.is_live));
    } catch {
      // backend not running — keep existing data or empty
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  useEffect(() => {
    load();
    const t = setInterval(() => load(), 5 * 60 * 1000); // refresh every 5 min
    return () => clearInterval(t);
  }, [load]);

  const tokens = ["ALL", ...Array.from(new Set(opps.map((o) => o.token)))];
  const filtered = filter === "ALL" ? opps : opps.filter((o) => o.token === filter);
  const maxAPY = Math.max(...opps.map((o) => o.supply_apy), 1);

  return (
    <div className={`bg-white rounded-xl border border-gray-200 ${className}`}>
      {/* Header */}
      <div className="px-4 pt-4 pb-3 border-b border-gray-100 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <TrendingUp size={14} className="text-green-500" />
          <span className="text-sm font-semibold text-gray-900">Yield Opportunities</span>
          <span className="text-xs text-gray-400">Kamino · Marginfi · Drift</span>
        </div>
        <div className="flex items-center gap-2">
          {/* Live / estimate indicator */}
          <span className="flex items-center gap-1 text-[10px]">
            {hasLive ? (
              <>
                <Wifi size={10} className="text-green-500" />
                <span className="text-green-600 font-medium">Live</span>
              </>
            ) : (
              <>
                <WifiOff size={10} className="text-gray-400" />
                <span className="text-gray-400">Estimates</span>
              </>
            )}
          </span>
          <button
            onClick={() => load(true)}
            disabled={refreshing}
            className="p-1 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors disabled:opacity-40"
            title="Refresh rates"
          >
            <RefreshCw size={12} className={refreshing ? "animate-spin" : ""} />
          </button>
        </div>
      </div>

      {/* Token filter tabs */}
      <div className="px-4 pt-2.5 pb-1 flex gap-1.5 overflow-x-auto">
        {tokens.map((t) => (
          <button
            key={t}
            onClick={() => setFilter(t)}
            className={`text-xs px-2.5 py-1 rounded-full whitespace-nowrap transition-colors ${
              filter === t
                ? "bg-gray-900 text-white"
                : "text-gray-500 hover:bg-gray-100"
            }`}
          >
            {t}
          </button>
        ))}
      </div>

      {/* Table */}
      <div className="px-4 pb-4">
        {loading ? (
          <div className="py-8 text-center text-sm text-gray-400">Loading rates…</div>
        ) : filtered.length === 0 ? (
          <div className="py-8 text-center text-sm text-gray-400">
            Backend offline — rates unavailable
          </div>
        ) : (
          <div className="mt-2 space-y-1">
            {/* Column headers */}
            <div className="grid grid-cols-[1fr_auto_auto_auto_auto] gap-3 px-2 mb-1">
              <span className="text-[10px] text-gray-400 uppercase tracking-wide">Protocol</span>
              <span className="text-[10px] text-gray-400 uppercase tracking-wide text-right">Token</span>
              <span className="text-[10px] text-gray-400 uppercase tracking-wide text-right">Supply APY</span>
              <span className="text-[10px] text-gray-400 uppercase tracking-wide text-right hidden sm:block">TVL</span>
              <span className="text-[10px] text-gray-400 uppercase tracking-wide text-right hidden sm:block">Util</span>
            </div>

            <AnimatePresence>
              {filtered.map((opp, i) => {
                const colors = PROTOCOL_COLORS[opp.protocol] ?? PROTOCOL_COLORS.kamino;
                const isBest = i === 0 && filter === "ALL";

                return (
                  <motion.div
                    key={`${opp.protocol}-${opp.token}`}
                    initial={{ opacity: 0, x: -8 }}
                    animate={{ opacity: 1, x: 0 }}
                    exit={{ opacity: 0, x: -8 }}
                    transition={{ delay: i * 0.04 }}
                    className={`grid grid-cols-[1fr_auto_auto_auto_auto] gap-3 items-center px-2 py-2 rounded-lg hover:bg-gray-50 transition-colors ${
                      isBest ? "ring-1 ring-green-200 bg-green-50/50" : ""
                    }`}
                  >
                    {/* Protocol name */}
                    <div className="flex items-center gap-2 min-w-0">
                      {isBest && <Trophy size={11} className="text-amber-500 flex-shrink-0" />}
                      <span className={`inline-flex items-center gap-1.5 text-xs px-2 py-0.5 rounded-full font-medium ${colors.bg} ${colors.text}`}>
                        <span className={`w-1.5 h-1.5 rounded-full ${colors.dot}`} />
                        {opp.display_name}
                      </span>
                      <a
                        href={opp.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-gray-300 hover:text-gray-500 transition-colors flex-shrink-0"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <ExternalLink size={10} />
                      </a>
                    </div>

                    {/* Token */}
                    <span className="text-xs font-mono font-semibold text-gray-700 text-right">
                      {opp.token}
                    </span>

                    {/* APY */}
                    <div className="flex items-center gap-2 justify-end">
                      <APYBar value={opp.supply_apy} max={maxAPY} />
                      <span
                        className={`text-sm font-bold font-mono-data tabular-nums min-w-[46px] text-right ${
                          opp.supply_apy >= 8
                            ? "text-green-600"
                            : opp.supply_apy >= 6
                            ? "text-yellow-600"
                            : "text-gray-700"
                        }`}
                      >
                        {opp.supply_apy.toFixed(2)}%
                      </span>
                    </div>

                    {/* TVL */}
                    <span className="text-xs text-gray-400 text-right hidden sm:block font-mono-data">
                      ${opp.tvl_millions >= 1000
                        ? (opp.tvl_millions / 1000).toFixed(1) + "B"
                        : opp.tvl_millions.toFixed(0) + "M"}
                    </span>

                    {/* Utilization */}
                    <span className="text-xs text-gray-400 text-right hidden sm:block font-mono-data">
                      {(opp.util_rate * 100).toFixed(0)}%
                    </span>
                  </motion.div>
                );
              })}
            </AnimatePresence>
          </div>
        )}
      </div>

      {/* Footer note */}
      {!loading && opps.length > 0 && (
        <div className="px-4 pb-3 border-t border-gray-50 pt-2.5">
          <p className="text-[10px] text-gray-400">
            {hasLive
              ? "Live rates from Kamino, Marginfi & Drift mainnet APIs · refreshed every 5 min"
              : "Estimated rates based on typical mainnet APYs · connect backend for live data"}
          </p>
        </div>
      )}
    </div>
  );
}
