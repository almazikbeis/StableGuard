"use client";

import { TokenInfo } from "@/lib/api";
import { TrendingDown, TrendingUp, Minus } from "lucide-react";

function deviationColor(dev: number) {
  if (dev < 0.05) return "text-emerald-300";
  if (dev < 0.15) return "text-amber-300";
  return "text-rose-300";
}

function DeviationBadge({ value }: { value: number }) {
  const cls = deviationColor(Math.abs(value));
  const Icon = Math.abs(value) < 0.01 ? Minus : value > 0 ? TrendingUp : TrendingDown;
  return (
    <span className={`inline-flex items-center gap-1 rounded-full border border-white/8 bg-white/[0.04] px-2 py-1 text-xs font-medium ${cls}`}>
      <Icon size={11} />
      {value === 0 ? "—" : `${value > 0 ? "+" : ""}${value.toFixed(4)}%`}
    </span>
  );
}

function formatPrice(price: number, assetType: string): string {
  if (assetType === "volatile") {
    if (price >= 10000) return `$${price.toLocaleString("en-US", { maximumFractionDigits: 0 })}`;
    if (price >= 100) return `$${price.toFixed(2)}`;
    return `$${price.toFixed(4)}`;
  }
  return `$${price.toFixed(6)}`;
}

const ASSET_TYPE_COLORS: Record<string, string> = {
  volatile: "bg-[linear-gradient(180deg,rgba(251,146,60,0.28),rgba(251,146,60,0.08))] border-orange-400/30",
  stable:   "bg-[linear-gradient(180deg,rgba(79,227,255,0.28),rgba(79,227,255,0.08))] border-cyan-400/30",
};

const SYMBOL_COLORS: Record<string, string> = {
  USDC:  "bg-[linear-gradient(180deg,rgba(37,99,235,0.35),rgba(37,99,235,0.12))] border-blue-500/40",
  USDT:  "bg-[linear-gradient(180deg,rgba(22,163,74,0.35),rgba(22,163,74,0.12))] border-green-500/40",
  DAI:   "bg-[linear-gradient(180deg,rgba(217,119,6,0.35),rgba(217,119,6,0.12))] border-amber-500/40",
  PYUSD: "bg-[linear-gradient(180deg,rgba(147,51,234,0.35),rgba(147,51,234,0.12))] border-purple-500/40",
  ETH:   "bg-[linear-gradient(180deg,rgba(139,92,246,0.35),rgba(139,92,246,0.12))] border-violet-500/40",
  SOL:   "bg-[linear-gradient(180deg,rgba(6,182,212,0.35),rgba(6,182,212,0.12))] border-cyan-500/40",
  BTC:   "bg-[linear-gradient(180deg,rgba(249,115,22,0.35),rgba(249,115,22,0.12))] border-orange-500/40",
};

const SYMBOL_TEXT: Record<string, string> = {
  USDC: "text-blue-300", USDT: "text-green-300", DAI: "text-amber-300",
  PYUSD: "text-purple-300", ETH: "text-violet-300", SOL: "text-cyan-300", BTC: "text-orange-300",
};

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
      <div className="overflow-x-auto rounded-[20px] border border-white/8 bg-[linear-gradient(180deg,rgba(255,255,255,0.04),rgba(255,255,255,0.02))] px-2 py-2">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-white/8">
              <th className="text-left px-3 pb-2 text-[11px] uppercase tracking-[0.18em] font-medium text-slate-500 pr-4">Token</th>
              <th className="text-right px-3 pb-2 text-[11px] uppercase tracking-[0.18em] font-medium text-slate-500 pr-4">Price</th>
              <th className="text-right px-3 pb-2 text-[11px] uppercase tracking-[0.18em] font-medium text-slate-500 pr-4">Confidence</th>
              <th className="text-right px-3 pb-2 text-[11px] uppercase tracking-[0.18em] font-medium text-slate-500">Peg δ</th>
            </tr>
          </thead>
          <tbody>
            {tokens.map((t) => (
              <tr key={t.symbol} className="border-b border-white/6 last:border-0 transition-colors hover:bg-white/[0.03]">
                <td className="px-3 py-3 pr-4">
                  <div className="flex items-center gap-2">
                    <span className={`w-9 h-9 rounded-2xl border flex items-center justify-center text-[10px] font-bold shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] ${SYMBOL_COLORS[t.symbol] ?? ASSET_TYPE_COLORS[t.asset_type] ?? ASSET_TYPE_COLORS.stable} ${SYMBOL_TEXT[t.symbol] ?? (t.asset_type === "volatile" ? "text-orange-200" : "text-cyan-100")}`}>
                      {t.symbol.slice(0, 2)}
                    </span>
                    <div>
                      <div className="font-display text-sm tracking-[0.08em] text-slate-100">{t.symbol}</div>
                      <div className="text-xs text-slate-500">{t.name}</div>
                    </div>
                  </div>
                </td>
                <td className="px-3 py-3 pr-4 text-right font-mono-data tabular-nums text-slate-100">
                  {formatPrice(t.price, t.asset_type)}
                </td>
                <td className="px-3 py-3 pr-4 text-right">
                  <div className="inline-flex flex-col items-end gap-1">
                    <span className="font-mono-data text-xs tabular-nums text-slate-300">±{t.confidence.toFixed(6)}</span>
                    <span className="h-1.5 w-20 overflow-hidden rounded-full bg-white/6">
                      <span
                        className="block h-full rounded-full bg-gradient-to-r from-cyan-300 to-orange-300"
                        style={{ width: `${Math.max(8, Math.min(100, t.confidence * 100000))}%` }}
                      />
                    </span>
                  </div>
                </td>
                <td className="px-3 py-3 text-right">
                  {t.asset_type === "volatile" ? (
                    <span className="inline-flex items-center gap-1 rounded-full border border-white/8 bg-white/[0.04] px-2 py-1 text-xs font-medium text-slate-500">
                      n/a
                    </span>
                  ) : (
                    <DeviationBadge value={t.symbol === "USDC" ? 0 : t.deviation_pct} />
                  )}
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
