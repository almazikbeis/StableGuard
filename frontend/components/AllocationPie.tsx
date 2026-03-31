"use client";

import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer } from "recharts";

const COLORS = ["#2563eb", "#16a34a", "#d97706", "#9333ea", "#0891b2", "#dc2626", "#6b7280", "#1e293b"];

interface Props {
  balances: number[];
  mints: string[];
  numTokens: number;
}

export function AllocationPie({ balances, mints, numTokens }: Props) {
  const data = Array.from({ length: numTokens }, (_, i) => ({
    name: mints[i] ? mints[i].slice(0, 6) + "…" : `Token ${i}`,
    value: balances[i] || 0,
  })).filter((d) => d.value > 0);

  if (data.length === 0) {
    return (
      <div className="h-40 flex flex-col items-center justify-center gap-2">
        <div className="w-20 h-20 rounded-full border-4 border-gray-100 flex items-center justify-center">
          <span className="text-xs text-gray-400">Empty</span>
        </div>
        <p className="text-xs text-gray-400">Vault is empty</p>
      </div>
    );
  }

  const total = data.reduce((s, d) => s + d.value, 0);

  return (
    <div className="flex items-center gap-6">
      <ResponsiveContainer width={140} height={140}>
        <PieChart>
          <Pie
            data={data}
            cx="50%"
            cy="50%"
            innerRadius={42}
            outerRadius={60}
            paddingAngle={2}
            dataKey="value"
          >
            {data.map((_, index) => (
              <Cell key={index} fill={COLORS[index % COLORS.length]} />
            ))}
          </Pie>
          <Tooltip
            contentStyle={{
              background: "#fff",
              border: "1px solid #e5e7eb",
              borderRadius: 8,
              fontSize: 12,
            }}
            formatter={(v) => {
              const n = typeof v === "number" ? v : 0;
              return [`${((n / total) * 100).toFixed(1)}% (${(n / 1e6).toFixed(2)}M)`, ""];
            }}
          />
        </PieChart>
      </ResponsiveContainer>

      <div className="flex flex-col gap-1.5 min-w-0">
        {data.map((d, i) => (
          <div key={i} className="flex items-center gap-2 text-xs">
            <span
              className="w-2.5 h-2.5 rounded-sm flex-shrink-0"
              style={{ background: COLORS[i % COLORS.length] }}
            />
            <span className="text-gray-600 truncate">{d.name}</span>
            <span className="text-gray-400 ml-auto pl-2">
              {((d.value / total) * 100).toFixed(1)}%
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
