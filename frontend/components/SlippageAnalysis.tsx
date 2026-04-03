"use client";

import { useEffect, useState } from "react";
import { motion } from "framer-motion";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  Cell,
} from "recharts";
import { Activity, AlertTriangle, CheckCircle2, TrendingDown } from "lucide-react";

const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

interface SlippageMeasurement {
  impact_10k: number;
  impact_100k: number;
  impact_1m: number;
  liquidity_score: number;
  drain_detected: boolean;
  input_mint: string;
  output_mint: string;
  measured_at: string;
}

interface SlippageResponse {
  current: SlippageMeasurement;
  window: SlippageMeasurement[];
  note: string;
  updated_at: number;
}

export function SlippageAnalysis({ symbol = "USDC" }: { symbol?: string }) {
  const [data, setData] = useState<SlippageResponse | null>(null);
  const [loading, setLoading] = useState(true);

  async function fetchSlippage() {
    try {
      const res = await fetch(`${BASE}/onchain/slippage`);
      if (!res.ok) return;
      const json = await res.json();
      setData(json);
    } catch {
      // silently fail — devnet will return zeros
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    fetchSlippage();
    const t = setInterval(fetchSlippage, 60_000);
    return () => clearInterval(t);
  }, []);

  const current = data?.current;
  const score = current?.liquidity_score ?? 100;
  const drain = current?.drain_detected ?? false;

  const barData = current
    ? [
        { size: "$10K", impact: +(current.impact_10k * 100).toFixed(4) },
        { size: "$100K", impact: +(current.impact_100k * 100).toFixed(4) },
        { size: "$1M", impact: +(current.impact_1m * 100).toFixed(4) },
      ]
    : [];

  const sparkData =
    data?.window?.map((m, i) => ({
      i,
      v: +(m.impact_100k * 100).toFixed(4),
    })) ?? [];

  function scoreColor(s: number) {
    if (s >= 80) return "text-green-600";
    if (s >= 50) return "text-yellow-600";
    return "text-red-600";
  }

  function scoreBg(s: number) {
    if (s >= 80) return "bg-green-50 border-green-200";
    if (s >= 50) return "bg-yellow-50 border-yellow-200";
    return "bg-red-50 border-red-200";
  }

  function barColor(impact: number) {
    if (impact < 0.01) return "#22c55e";
    if (impact < 0.1) return "#eab308";
    return "#ef4444";
  }

  return (
    <div className="bg-white rounded-2xl border border-gray-100 shadow-sm p-5">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div>
          <h3 className="font-display font-bold text-gray-900 text-sm flex items-center gap-1.5">
            <Activity size={14} className="text-purple-500" />
            Liquidity Depth
          </h3>
          <p className="text-xs text-gray-400 mt-0.5">Jupiter mainnet · {symbol}/USDT</p>
        </div>

        {!loading && current && (
          <div className={`flex items-center gap-1.5 px-3 py-1.5 rounded-full border text-xs font-bold ${scoreBg(score)}`}>
            {score >= 80 ? <CheckCircle2 size={12} className="text-green-500" /> : <TrendingDown size={12} className="text-yellow-500" />}
            <span className={scoreColor(score)}>Score {score}</span>
          </div>
        )}
      </div>

      {/* Drain alert */}
      {drain && (
        <motion.div
          initial={{ opacity: 0, y: -4 }}
          animate={{ opacity: 1, y: 0 }}
          className="flex items-center gap-2 bg-red-50 border border-red-200 rounded-xl px-3 py-2 mb-4 text-xs text-red-700 font-semibold"
        >
          <AlertTriangle size={12} />
          Liquidity drain detected — 3x impact increase vs 45 min ago
        </motion.div>
      )}

      {/* Bar chart */}
      {loading ? (
        <div className="h-32 flex items-center justify-center text-sm text-gray-400">
          Loading liquidity data…
        </div>
      ) : barData.length > 0 ? (
        <div className="h-32">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={barData} barSize={28}>
              <XAxis
                dataKey="size"
                tick={{ fontSize: 10, fill: "#9ca3af" }}
                axisLine={false}
                tickLine={false}
              />
              <YAxis hide />
              <Tooltip
                formatter={(v) => [`${v}%`, "Price Impact"]}
                contentStyle={{ fontSize: 11, borderRadius: 8 }}
              />
              <Bar dataKey="impact" radius={[4, 4, 0, 0]}>
                {barData.map((entry, i) => (
                  <Cell key={i} fill={barColor(entry.impact)} />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </div>
      ) : (
        <div className="h-32 flex items-center justify-center">
          <div className="text-center">
            <CheckCircle2 size={20} className="text-green-500 mx-auto mb-1" />
            <p className="text-xs text-gray-500">Impact = 0% (devnet)</p>
            <p className="text-xs text-gray-400">Score: 100 — max liquidity</p>
          </div>
        </div>
      )}

      {/* Sparkline window */}
      {sparkData.length > 1 && (
        <div className="mt-3 pt-3 border-t border-gray-50">
          <p className="text-[10px] text-gray-400 mb-1.5">Last {sparkData.length} measurements (100K impact %)</p>
          <div className="h-8">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={sparkData} barSize={6}>
                <Bar dataKey="v" fill="#a78bfa" radius={[2, 2, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      )}

      {data?.note && (
        <p className="text-[10px] text-gray-400 mt-2">{data.note}</p>
      )}
    </div>
  );
}
