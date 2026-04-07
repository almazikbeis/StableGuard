"use client";

import {
  Area,
  AreaChart,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
  ReferenceLine,
} from "recharts";
import { PricePoint } from "@/lib/api";

interface Props {
  data: PricePoint[];
  symbol: string;
  color?: string;
  assetType?: string; // "stable" | "volatile"
}

const VOLATILE_SYMBOLS = new Set(["BTC", "ETH", "SOL"]);

function fmtPrice(v: number, symbol: string): string {
  if (VOLATILE_SYMBOLS.has(symbol)) {
    if (v >= 10000) return `$${v.toLocaleString("en-US", { maximumFractionDigits: 0 })}`;
    if (v >= 100)   return `$${v.toFixed(2)}`;
    return `$${v.toFixed(4)}`;
  }
  return `$${v.toFixed(6)}`;
}

function fmtAxis(v: number, symbol: string): string {
  if (VOLATILE_SYMBOLS.has(symbol)) {
    if (v >= 1000) return `$${(v / 1000).toFixed(1)}k`;
    return `$${v.toFixed(0)}`;
  }
  return v.toFixed(4);
}

export function PriceChart({ data, symbol, color = "#2563eb", assetType }: Props) {
  const isVolatile = assetType === "volatile" || VOLATILE_SYMBOLS.has(symbol);
  if (!data || data.length === 0) {
    return (
      <div className="h-52 flex items-center justify-center rounded-[20px] border border-dashed border-white/10 bg-white/[0.03] text-sm text-slate-400">
        No price data yet
      </div>
    );
  }

  const formatted = data.map((p) => ({
    time: new Date(p.ts * 1000).toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
    }),
    price: p.price,
  }));

  const prices = data.map((p) => p.price);
  const latest = prices[prices.length - 1] ?? 0;
  const first = prices[0] ?? latest;
  const deltaPct = first === 0 ? 0 : ((latest - first) / first) * 100;
  const minP = Math.min(...prices);
  const maxP = Math.max(...prices);
  const pad = (maxP - minP) * 0.1 || 0.001;
  const chartId = `price-${symbol.toLowerCase()}`;
  const glow = `${color}44`;
  const trendTone = deltaPct >= 0 ? "text-emerald-300" : "text-rose-300";

  return (
    <div className="rounded-[22px] border border-white/8 bg-[linear-gradient(180deg,rgba(255,255,255,0.04),rgba(255,255,255,0.02))] p-4">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <div className="text-[11px] uppercase tracking-[0.22em] text-slate-500">{symbol} oracle track</div>
          <div className="mt-2 flex items-end gap-3">
            <span className="font-mono-data text-2xl tabular-nums text-white">{fmtPrice(latest, symbol)}</span>
            <span className={`text-xs font-semibold ${trendTone}`}>
              {deltaPct >= 0 ? "+" : ""}
              {deltaPct.toFixed(4)}%
            </span>
          </div>
        </div>
        <div className="data-chip rounded-full px-3 py-1 text-[10px] uppercase tracking-[0.18em] text-slate-400">
          {formatted.length} points loaded
        </div>
      </div>

      <ResponsiveContainer width="100%" height={220}>
        <AreaChart data={formatted} margin={{ top: 8, right: 8, left: -18, bottom: 0 }}>
          <defs>
            <linearGradient id={`${chartId}-fill`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={color} stopOpacity={0.35} />
              <stop offset="100%" stopColor={color} stopOpacity={0.02} />
            </linearGradient>
            <filter id={`${chartId}-glow`}>
              <feGaussianBlur stdDeviation="4" result="blur" />
              <feMerge>
                <feMergeNode in="blur" />
                <feMergeNode in="SourceGraphic" />
              </feMerge>
            </filter>
          </defs>

          <CartesianGrid strokeDasharray="4 8" stroke="rgba(148,163,184,0.16)" vertical={false} />
          <XAxis
            dataKey="time"
            tick={{ fontSize: 10, fill: "#6f86ab" }}
            tickLine={false}
            axisLine={false}
            interval="preserveStartEnd"
          />
          <YAxis
            domain={[minP - pad, maxP + pad]}
            tick={{ fontSize: 10, fill: "#6f86ab" }}
            tickLine={false}
            axisLine={false}
            tickFormatter={(v) => fmtAxis(Number(v), symbol)}
          />
          {!isVolatile && (
            <ReferenceLine y={1} stroke="rgba(255,255,255,0.18)" strokeDasharray="5 5" />
          )}
          <Tooltip
            cursor={{ stroke: glow, strokeWidth: 1 }}
            contentStyle={{
              background: "rgba(6,18,34,0.94)",
              border: "1px solid rgba(255,255,255,0.08)",
              borderRadius: 16,
              fontSize: 12,
              color: "#e2e8f0",
              boxShadow: "0 18px 38px rgba(0,0,0,0.28)",
              backdropFilter: "blur(14px)",
            }}
            labelStyle={{ color: "#8da2c3", marginBottom: 6 }}
            formatter={(v) => {
              const n = typeof v === "number" ? v : 0;
              return [fmtPrice(n, symbol), symbol];
            }}
          />
          <Area
            type="monotone"
            dataKey="price"
            stroke={color}
            strokeWidth={2.4}
            fill={`url(#${chartId}-fill)`}
            dot={false}
            activeDot={{ r: 4, fill: color, stroke: "#07111f", strokeWidth: 2 }}
            filter={`url(#${chartId}-glow)`}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
