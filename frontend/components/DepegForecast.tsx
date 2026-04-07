"use client";

import { useState, useEffect, useCallback } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  ComposedChart, Area, Line, XAxis, YAxis, Tooltip,
  ReferenceLine, ResponsiveContainer, CartesianGrid,
} from "recharts";
import { Brain, TrendingDown, TrendingUp, Minus, AlertTriangle, RefreshCw, Loader2, Zap } from "lucide-react";
import { isStableSymbol } from "@/lib/assets";

const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

interface PredictionResult {
  predictions: number[];
  low: number[];
  high: number[];
  depeg_probability: number;
  severe_probability: number;
  trend: "stable" | "declining" | "recovering";
  horizon_steps: number;
  step_minutes: number;
  min_predicted: number;
  max_predicted: number;
  hours_to_warning: number | null;
  inference_ms: number;
}

interface ApiResponse {
  available: boolean;
  message?: string;
  symbol?: string;
  history?: number[];
  prediction?: PredictionResult;
}

function buildChartData(history: number[], pred: PredictionResult) {
  const data: Array<{
    label: string;
    price?: number;
    predicted?: number;
    low?: number;
    high?: number;
    type: "history" | "forecast";
  }> = [];

  // last 30 historical points
  const hist = history.slice(-30);
  hist.forEach((p, i) => {
    data.push({
      label: `-${hist.length - i}`,
      price: p,
      type: "history",
    });
  });

  // forecast
  pred.predictions.forEach((p, i) => {
    data.push({
      label: `+${(i + 1) * pred.step_minutes}m`,
      predicted: p,
      low: pred.low[i],
      high: pred.high[i],
      type: "forecast",
    });
  });

  return data;
}

function ProbBadge({ prob, label }: { prob: number; label: string }) {
  const color =
    prob < 5  ? "bg-green-50 text-green-700 border-green-200" :
    prob < 25 ? "bg-yellow-50 text-yellow-700 border-yellow-200" :
    prob < 60 ? "bg-orange-50 text-orange-700 border-orange-200" :
                "bg-red-50 text-red-700 border-red-200";
  return (
    <div className={`rounded-xl border px-3 py-2.5 flex flex-col items-center ${color}`}>
      <span className="text-2xl font-display font-extrabold">{prob.toFixed(1)}%</span>
      <span className="text-[10px] font-semibold uppercase tracking-wide mt-0.5 opacity-70">{label}</span>
    </div>
  );
}

function TrendIcon({ trend }: { trend: string }) {
  if (trend === "declining") return <TrendingDown size={13} className="text-red-500" />;
  if (trend === "recovering") return <TrendingUp size={13} className="text-green-500" />;
  return <Minus size={13} className="text-gray-400" />;
}

interface ForecastTooltipPayload {
  payload?: {
    price?: number;
    predicted?: number;
    low?: number;
    high?: number;
  };
}

interface ForecastTooltipProps {
  active?: boolean;
  payload?: ForecastTooltipPayload[];
  label?: string;
}

const CustomTooltip = ({ active, payload, label }: ForecastTooltipProps) => {
  if (!active || !payload?.length) return null;
  const d = payload[0]?.payload;
  return (
    <div className="bg-white border border-gray-200 rounded-xl shadow-lg px-3 py-2 text-xs">
      <p className="text-gray-400 mb-1">{label}</p>
      {d?.price != null && <p className="text-gray-800 font-mono">{d.price.toFixed(5)}</p>}
      {d?.predicted != null && (
        <>
          <p className="text-orange-600 font-mono font-semibold">pred: {d.predicted.toFixed(5)}</p>
          {d.low != null && d.high != null && <p className="text-gray-400 font-mono">CI: {d.low.toFixed(5)} – {d.high.toFixed(5)}</p>}
        </>
      )}
    </div>
  );
};

interface Props {
  symbol?: string;
}

export function DepegForecast({ symbol = "USDC" }: Props) {
  const [data, setData] = useState<ApiResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const stableAsset = isStableSymbol(symbol);

  const fetch = useCallback(async () => {
    setLoading(true);
    try {
      const res = await window.fetch(`${BASE}/prediction/depeg?symbol=${symbol}&limit=200`);
      const json = await res.json();
      setData(json);
    } catch {
      setData({ available: false, message: "Backend offline" });
    } finally {
      setLoading(false);
    }
  }, [symbol]);

  useEffect(() => {
    fetch();
    const id = setInterval(fetch, 5 * 60 * 1000); // refresh every 5 min
    return () => clearInterval(id);
  }, [fetch]);

  const chartData = data?.available && data.history && data.prediction
    ? buildChartData(data.history, data.prediction)
    : [];

  const pred = data?.prediction;

  return (
    <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
      {/* Header */}
      <div className="px-4 pt-4 pb-3 border-b border-gray-100 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <div className="w-7 h-7 rounded-lg bg-purple-50 border border-purple-100 flex items-center justify-center">
            <Brain size={13} className="text-purple-500" />
          </div>
          <div>
            <span className="text-sm font-semibold text-gray-900">
              {stableAsset ? "Depeg Forecast" : "Market Forecast"}
            </span>
            <span className="text-xs text-gray-400 ml-2">Chronos T5 · 4h horizon</span>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {pred && (
            <span className="text-[10px] text-gray-400 font-mono">{pred.inference_ms}ms</span>
          )}
          <button
            onClick={fetch}
            disabled={loading}
            className="p-1.5 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors disabled:opacity-40"
          >
            {loading ? <Loader2 size={13} className="animate-spin" /> : <RefreshCw size={13} />}
          </button>
        </div>
      </div>

      <div className="p-4">
        {loading && !data && (
          <div className="h-48 flex items-center justify-center">
            <div className="flex flex-col items-center gap-2">
              <Loader2 size={20} className="animate-spin text-purple-400" />
              <p className="text-xs text-gray-400">Running Chronos T5…</p>
            </div>
          </div>
        )}

        {!loading && data && !data.available && (
          <div className="rounded-xl bg-gray-50 border border-gray-200 p-4 flex items-start gap-3">
            <Zap size={14} className="text-yellow-500 flex-shrink-0 mt-0.5" />
            <div>
              <p className="text-xs font-semibold text-gray-700">ML Service Offline</p>
              <p className="text-xs text-gray-400 mt-0.5">{data.message}</p>
              <code className="text-[10px] bg-gray-100 border border-gray-200 rounded px-2 py-1 mt-2 block text-gray-600">
                cd ml-service && pip install -r requirements.txt && python main.py
              </code>
            </div>
          </div>
        )}

        <AnimatePresence>
          {!loading && data?.available && pred && (
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              className="space-y-4"
            >
              {/* Probability badges */}
              <div className="grid grid-cols-3 gap-2">
                <ProbBadge prob={pred.depeg_probability} label={stableAsset ? "Depeg risk" : "Downside risk"} />
                <ProbBadge prob={pred.severe_probability} label={stableAsset ? "Severe risk" : "Severe downside"} />
                <div className="rounded-xl border border-gray-100 bg-gray-50 px-3 py-2.5 flex flex-col items-center">
                  <div className="flex items-center gap-1 mt-0.5">
                    <TrendIcon trend={pred.trend} />
                    <span className="text-sm font-bold font-display text-gray-700 capitalize">{pred.trend}</span>
                  </div>
                  <span className="text-[10px] font-semibold uppercase tracking-wide mt-0.5 text-gray-400">Trend</span>
                </div>
              </div>

              {/* Hours to warning */}
              {pred.hours_to_warning != null && (
                <motion.div
                  initial={{ opacity: 0, x: -8 }}
                  animate={{ opacity: 1, x: 0 }}
                  className="flex items-center gap-2 bg-orange-50 border border-orange-200 rounded-xl px-3 py-2"
                >
                  <AlertTriangle size={13} className="text-orange-500 flex-shrink-0" />
                  <p className="text-xs text-orange-700">
                    Model predicts {stableAsset ? "peg stress" : "market weakness"} in{" "}
                    <span className="font-bold">{pred.hours_to_warning}h</span> — monitoring closely
                  </p>
                </motion.div>
              )}

              {/* Chart */}
              <div className="h-48">
                <ResponsiveContainer width="100%" height="100%">
                  <ComposedChart data={chartData} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#f3f4f6" />
                    <XAxis
                      dataKey="label"
                      tick={{ fontSize: 9, fill: "#9ca3af" }}
                      tickLine={false}
                      axisLine={false}
                      interval={4}
                    />
                    <YAxis
                      domain={["auto", "auto"]}
                      tick={{ fontSize: 9, fill: "#9ca3af" }}
                      tickLine={false}
                      axisLine={false}
                      tickFormatter={v => v.toFixed(3)}
                    />
                    <Tooltip content={<CustomTooltip />} />

                    {stableAsset && (
                      <ReferenceLine y={1.0} stroke="#d1d5db" strokeDasharray="4 2" strokeWidth={1} />
                    )}
                    {stableAsset && (
                      <ReferenceLine y={0.998} stroke="#f97316" strokeDasharray="3 2" strokeWidth={1} label={{ value: "0.998", fill: "#f97316", fontSize: 8 }} />
                    )}

                    {/* Confidence band */}
                    <Area
                      dataKey="high"
                      stroke="none"
                      fill="#a855f720"
                      fillOpacity={1}
                      legendType="none"
                      dot={false}
                      activeDot={false}
                    />
                    <Area
                      dataKey="low"
                      stroke="none"
                      fill="#ffffff"
                      fillOpacity={1}
                      legendType="none"
                      dot={false}
                      activeDot={false}
                    />

                    {/* Historical prices */}
                    <Line
                      dataKey="price"
                      stroke="#f97316"
                      strokeWidth={2}
                      dot={false}
                      activeDot={{ r: 3, fill: "#f97316" }}
                      connectNulls={false}
                    />

                    {/* Predicted prices */}
                    <Line
                      dataKey="predicted"
                      stroke="#a855f7"
                      strokeWidth={1.5}
                      strokeDasharray="5 3"
                      dot={false}
                      activeDot={{ r: 3, fill: "#a855f7" }}
                      connectNulls={false}
                    />
                  </ComposedChart>
                </ResponsiveContainer>
              </div>

              {/* Legend */}
              <div className="flex items-center gap-4 text-[10px] text-gray-400">
                <span className="flex items-center gap-1.5">
                  <span className="w-4 h-0.5 bg-orange-400 inline-block rounded" />
                  Historical
                </span>
                <span className="flex items-center gap-1.5">
                  <span className="w-4 h-0.5 bg-purple-400 inline-block rounded" style={{ borderTop: "1.5px dashed #a855f7", background: "none" }} />
                  Forecast (median)
                </span>
                <span className="flex items-center gap-1.5">
                  <span className="w-4 h-3 bg-purple-100 inline-block rounded" />
                  90% confidence band
                </span>
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  );
}
