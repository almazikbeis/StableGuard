"use client";

import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer } from "recharts";

// slot → { symbol, decimals, color }
const SLOT_META: Record<number, { symbol: string; decimals: number; color: string }> = {
  0: { symbol: "USDC",  decimals: 6, color: "#2563eb" },
  1: { symbol: "USDT",  decimals: 6, color: "#16a34a" },
  2: { symbol: "ETH",   decimals: 6, color: "#8b5cf6" },
  3: { symbol: "SOL",   decimals: 9, color: "#06b6d4" },
  4: { symbol: "BTC",   decimals: 6, color: "#f97316" },
  5: { symbol: "DAI",   decimals: 6, color: "#d97706" },
  6: { symbol: "PYUSD", decimals: 6, color: "#9333ea" },
};

interface Props {
  balances: number[];
  numTokens: number;
}

function fmtAmount(raw: number, decimals: number, symbol: string): string {
  const amount = raw / Math.pow(10, decimals);
  if (symbol === "BTC")  return amount.toFixed(4);
  if (symbol === "ETH")  return amount.toFixed(3);
  if (symbol === "SOL")  return amount.toFixed(2);
  return amount.toLocaleString("en-US", { maximumFractionDigits: 2 });
}

export function AllocationPie({ balances, numTokens }: Props) {
  const data = Array.from({ length: numTokens }, (_, i) => {
    const meta = SLOT_META[i];
    const raw = balances[i] || 0;
    if (!raw) return null;
    const decimals = meta?.decimals ?? 6;
    const amount = raw / Math.pow(10, decimals);
    return {
      slot: i,
      name: meta?.symbol ?? `Token ${i}`,
      color: meta?.color ?? "#6b7280",
      raw,
      amount,
      decimals,
    };
  }).filter(Boolean) as Array<{ slot: number; name: string; color: string; raw: number; amount: number; decimals: number }>;

  if (data.length === 0) {
    return (
      <div className="h-40 flex flex-col items-center justify-center gap-2">
        <div className="w-20 h-20 rounded-full border-4 border-white/10 flex items-center justify-center">
          <span className="text-xs text-slate-500">Empty</span>
        </div>
        <p className="text-xs text-slate-500">Vault is empty</p>
      </div>
    );
  }

  const totalRaw = data.reduce((s, d) => s + d.raw, 0);

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
            dataKey="raw"
          >
            {data.map((d) => (
              <Cell key={d.slot} fill={d.color} />
            ))}
          </Pie>
          <Tooltip
            contentStyle={{
              background: "rgba(15,15,20,0.95)",
              border: "1px solid rgba(255,255,255,0.12)",
              borderRadius: 10,
              fontSize: 12,
              color: "#e2e8f0",
            }}
            formatter={(v, _name, props) => {
              const entry = props.payload;
              const pct = ((entry.raw / totalRaw) * 100).toFixed(1);
              const amt = fmtAmount(entry.raw, entry.decimals, entry.name);
              return [`${amt} ${entry.name} (${pct}%)`, ""];
            }}
          />
        </PieChart>
      </ResponsiveContainer>

      <div className="flex flex-col gap-2 min-w-0">
        {data.map((d) => (
          <div key={d.slot} className="flex items-center gap-2 text-xs">
            <span className="w-2.5 h-2.5 rounded-sm flex-shrink-0" style={{ background: d.color }} />
            <span className="text-slate-200 font-semibold w-10 flex-shrink-0">{d.name}</span>
            <span className="text-slate-400 font-mono text-[11px]">
              {fmtAmount(d.raw, d.decimals, d.name)}
            </span>
            <span className="text-slate-600 ml-auto pl-1 text-[10px]">
              {((d.raw / totalRaw) * 100).toFixed(1)}%
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
