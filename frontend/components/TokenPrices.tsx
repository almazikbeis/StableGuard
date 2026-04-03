"use client";

import { TokenInfo } from "@/lib/api";
import { TrendingDown, TrendingUp, Minus } from "lucide-react";

function deviationColor(dev: number) {
  if (dev < 0.05) return "text-green-600";
  if (dev < 0.15) return "text-yellow-600";
  return "text-red-600";
}

function DeviationBadge({ value }: { value: number }) {
  const cls = deviationColor(Math.abs(value));
  const Icon = Math.abs(value) < 0.01 ? Minus : value > 0 ? TrendingUp : TrendingDown;
  return (
    <span className={`inline-flex items-center gap-1 text-xs font-medium ${cls}`}>
      <Icon size={11} />
      {value === 0 ? "—" : `${value > 0 ? "+" : ""}${value.toFixed(4)}%`}
    </span>
  );
}

interface Props {
  tokens: TokenInfo[];
  fetchedAt?: string;
}

export function TokenPrices({ tokens, fetchedAt }: Props) {
  const time = fetchedAt
    ? new Date(fetchedAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })
    : null;

  return (
    <div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-white/8">
              <th className="text-left pb-2 text-xs font-medium text-slate-400 pr-4">Token</th>
              <th className="text-right pb-2 text-xs font-medium text-slate-400 pr-4">Price</th>
              <th className="text-right pb-2 text-xs font-medium text-slate-400 pr-4">Confidence</th>
              <th className="text-right pb-2 text-xs font-medium text-slate-400">vs USDC</th>
            </tr>
          </thead>
          <tbody>
            {tokens.map((t) => (
              <tr key={t.symbol} className="border-b border-white/6 last:border-0">
                <td className="py-2.5 pr-4">
                  <div className="flex items-center gap-2">
                    <span className="w-7 h-7 rounded-full bg-white/6 border border-white/8 flex items-center justify-center text-[10px] font-bold text-slate-300">
                      {t.symbol.slice(0, 2)}
                    </span>
                    <div>
                      <div className="font-medium text-slate-100">{t.symbol}</div>
                      <div className="text-xs text-slate-400">{t.name}</div>
                    </div>
                  </div>
                </td>
                <td className="py-2.5 pr-4 text-right font-mono text-slate-100">
                  ${t.price.toFixed(6)}
                </td>
                <td className="py-2.5 pr-4 text-right font-mono text-slate-400 text-xs">
                  ±{t.confidence.toFixed(6)}
                </td>
                <td className="py-2.5 text-right">
                  <DeviationBadge value={t.symbol === "USDC" ? 0 : t.deviation_pct} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {time && (
        <p className="text-xs text-slate-400 mt-3 text-right">Updated {time}</p>
      )}
    </div>
  );
}
