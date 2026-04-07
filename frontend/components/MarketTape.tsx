"use client";

import { motion } from "framer-motion";
import { Activity, ArrowDownRight, ArrowUpRight, Minus } from "lucide-react";
import { TokenInfo } from "@/lib/api";
import { isStableSymbol, isVolatileSymbol } from "@/lib/assets";

interface Props {
  tokens: TokenInfo[];
}

function deltaTone(delta: number) {
  const abs = Math.abs(delta);
  if (abs < 0.005) {
    return {
      icon: Minus,
      iconClass: "text-slate-400",
      textClass: "text-slate-300",
      glowClass: "shadow-[0_0_0_1px_rgba(148,163,184,0.2)]",
    };
  }
  if (delta > 0) {
    return {
      icon: ArrowUpRight,
      iconClass: "text-emerald-300",
      textClass: "text-emerald-200",
      glowClass: "shadow-[0_0_0_1px_rgba(110,231,183,0.22),0_0_24px_rgba(16,185,129,0.14)]",
    };
  }
  return {
    icon: ArrowDownRight,
    iconClass: "text-rose-300",
    textClass: "text-rose-200",
    glowClass: "shadow-[0_0_0_1px_rgba(253,164,175,0.22),0_0_24px_rgba(244,63,94,0.12)]",
  };
}

function formatTapePrice(symbol: string, price: number) {
  if (isVolatileSymbol(symbol)) {
    if (price >= 10000) return `$${price.toLocaleString("en-US", { maximumFractionDigits: 0 })}`;
    if (price >= 100) return `$${price.toFixed(2)}`;
    return `$${price.toFixed(4)}`;
  }
  return `$${price.toFixed(6)}`;
}

export function MarketTape({ tokens }: Props) {
  const items = [...tokens, ...tokens];

  return (
    <div className="panel-surface-soft rounded-[22px] border border-white/8 overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3 border-b border-white/8">
        <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.22em] text-slate-400">
          <Activity size={13} className="text-cyan-300" />
          Market Tape
        </div>
        <div className="text-[11px] uppercase tracking-[0.18em] text-slate-500">
          Cross-asset pulse live
        </div>
      </div>

      <div className="relative overflow-hidden">
        <div className="pointer-events-none absolute inset-y-0 left-0 w-20 bg-gradient-to-r from-[#07111f] to-transparent z-10" />
        <div className="pointer-events-none absolute inset-y-0 right-0 w-20 bg-gradient-to-l from-[#07111f] to-transparent z-10" />

        <motion.div
          className="flex min-w-max gap-3 px-4 py-4"
          animate={{ x: ["0%", "-50%"] }}
          transition={{ duration: 24, ease: "linear", repeat: Infinity }}
        >
          {items.map((token, index) => {
            const tone = deltaTone(token.deviation_pct);
            const Icon = tone.icon;

            return (
              <div
                key={`${token.symbol}-${index}`}
                className={`group relative min-w-[210px] rounded-[18px] border border-white/8 bg-white/[0.04] px-4 py-3 backdrop-blur-md transition-transform duration-300 hover:-translate-y-0.5 ${tone.glowClass}`}
              >
                <div className="absolute inset-x-4 top-0 h-px bg-gradient-to-r from-transparent via-white/30 to-transparent opacity-60" />
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="font-display text-sm tracking-[0.08em] text-white">{token.symbol}</span>
                      <span className="rounded-full border border-white/10 bg-white/5 px-2 py-0.5 text-[10px] uppercase tracking-[0.18em] text-slate-400">
                        {token.name}
                      </span>
                    </div>
                    <div className="mt-2 font-mono-data text-lg tabular-nums text-white">
                      {formatTapePrice(token.symbol, token.price)}
                    </div>
                  </div>

                  <div className={`rounded-full border border-white/10 bg-black/20 p-2 ${tone.iconClass}`}>
                    <Icon size={14} />
                  </div>
                </div>

                <div className="mt-3 flex items-center justify-between gap-3 text-xs">
                  <span className={tone.textClass}>
                    {isStableSymbol(token.symbol)
                      ? Math.abs(token.deviation_pct) < 0.0001
                        ? "On peg"
                        : `${token.deviation_pct > 0 ? "+" : ""}${token.deviation_pct.toFixed(4)}% vs peg`
                      : isVolatileSymbol(token.symbol)
                      ? "Volatile spot asset"
                      : "Live market asset"}
                  </span>
                  <span className="text-slate-500">conf ±{token.confidence.toFixed(6)}</span>
                </div>
              </div>
            );
          })}
        </motion.div>
      </div>
    </div>
  );
}
