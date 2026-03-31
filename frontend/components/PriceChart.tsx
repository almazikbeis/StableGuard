"use client";

import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from "recharts";
import { PricePoint } from "@/lib/api";

interface Props {
  data: PricePoint[];
  symbol: string;
  color?: string;
}

export function PriceChart({ data, symbol, color = "#2563eb" }: Props) {
  if (!data || data.length === 0) {
    return (
      <div className="h-40 flex items-center justify-center text-sm text-gray-400">
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
  const minP = Math.min(...prices);
  const maxP = Math.max(...prices);
  const pad = (maxP - minP) * 0.1 || 0.001;

  return (
    <ResponsiveContainer width="100%" height={160}>
      <LineChart data={formatted} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="#f3f4f6" />
        <XAxis
          dataKey="time"
          tick={{ fontSize: 10, fill: "#9ca3af" }}
          tickLine={false}
          axisLine={false}
          interval="preserveStartEnd"
        />
        <YAxis
          domain={[minP - pad, maxP + pad]}
          tick={{ fontSize: 10, fill: "#9ca3af" }}
          tickLine={false}
          axisLine={false}
          tickFormatter={(v) => v.toFixed(4)}
        />
        <Tooltip
          contentStyle={{
            background: "#fff",
            border: "1px solid #e5e7eb",
            borderRadius: 8,
            fontSize: 12,
            boxShadow: "0 4px 6px -1px rgb(0 0 0 / 0.05)",
          }}
          labelStyle={{ color: "#6b7280" }}
          formatter={(v) => {
            const n = typeof v === "number" ? v : 0;
            return [`$${n.toFixed(6)}`, symbol];
          }}
        />
        <Line
          type="monotone"
          dataKey="price"
          stroke={color}
          strokeWidth={1.5}
          dot={false}
          activeDot={{ r: 3, fill: color }}
        />
      </LineChart>
    </ResponsiveContainer>
  );
}
