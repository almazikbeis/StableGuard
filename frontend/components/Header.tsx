"use client";

import { Shield, RefreshCw, Settings } from "lucide-react";
import Link from "next/link";
import { CommandBar } from "@/components/CommandBar";
import { WalletButton } from "@/components/WalletButton";

interface Props {
  lastUpdate?: string;
  onRefresh?: () => void;
  refreshing?: boolean;
  connected?: boolean;
  streamMode?: "sse" | "polling" | "connecting";
}

export function Header({ lastUpdate, onRefresh, refreshing, connected, streamMode }: Props) {
  const modeLabel =
    streamMode === "sse" ? "Live"
    : streamMode === "polling" ? "Polling"
    : "Connecting…";

  return (
    <header className="sticky top-0 z-50 border-b border-white/8 bg-[#08111f]/80 backdrop-blur-xl">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 h-16 flex items-center justify-between gap-4">
        <Link href="/" className="flex items-center gap-3 hover:opacity-90 transition-opacity">
          <div className="w-8 h-8 rounded-xl bg-[linear-gradient(135deg,#ff7a1a,#ffb347)] shadow-[0_0_24px_rgba(255,122,26,0.35)] flex items-center justify-center">
            <Shield size={14} className="text-white" />
          </div>
          <div>
            <span className="font-display font-semibold text-slate-50 text-sm tracking-[0.08em] uppercase">StableGuard</span>
            <span className="text-[11px] text-slate-400 font-normal hidden lg:block">
              AI Treasury Control Plane
            </span>
          </div>
          <span className="hidden xl:inline glass-pill text-[10px] uppercase tracking-[0.22em] text-cyan-200/80 px-2.5 py-1 rounded-full">
            Solana
          </span>
        </Link>

        <div className="flex items-center gap-2 sm:gap-3">
          <WalletButton />
          <CommandBar />

          {lastUpdate && (
            <span className="text-xs text-slate-400 hidden lg:inline">
              Updated {lastUpdate}
            </span>
          )}

          <div className="glass-pill flex items-center gap-2 rounded-full px-3 py-1.5">
            {connected ? (
              <>
                <span className="relative flex h-2.5 w-2.5">
                  <span className="absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75 animate-ping" />
                  <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-emerald-300" />
                </span>
                <span className="text-xs text-slate-200">{modeLabel}</span>
              </>
            ) : (
              <>
                <span className="w-2.5 h-2.5 rounded-full bg-slate-500" />
                <span className="text-xs text-slate-400">{modeLabel}</span>
              </>
            )}
          </div>

          {onRefresh && (
            <button
              onClick={onRefresh}
              disabled={refreshing}
              className="p-2 rounded-xl border border-white/10 bg-white/4 hover:bg-white/8 text-slate-300 hover:text-white transition-colors disabled:opacity-40"
              title="Refresh"
            >
              <RefreshCw size={14} className={refreshing ? "animate-spin" : ""} />
            </button>
          )}

          <Link
            href="/settings"
            className="p-2 rounded-xl border border-white/10 bg-white/4 hover:bg-white/8 text-slate-300 hover:text-white transition-colors"
            title="Settings"
          >
            <Settings size={14} />
          </Link>
        </div>
      </div>
    </header>
  );
}
