"use client";

import { Shield, RefreshCw, Settings } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
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
  const pathname = usePathname();
  const modeLabel =
    streamMode === "sse" ? "Live"
    : streamMode === "polling" ? "Polling"
    : "Connecting…";
  const navItems = [
    { href: "/dashboard", label: "Dashboard" },
    { href: "/audit", label: "AI Audit" },
    { href: "/execution", label: "Execution" },
    { href: "/settings", label: "Settings" },
    { href: "/", label: "Landing" },
  ];

  return (
    <header className="sticky top-0 z-50 border-b border-white/8 bg-[#08111f]/84 backdrop-blur-xl">
      <div className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-cyan-300/40 to-transparent" />
      <div className="pointer-events-none absolute inset-x-0 bottom-0 h-px bg-gradient-to-r from-transparent via-orange-300/18 to-transparent" />
      <div className="pointer-events-none absolute inset-x-0 bottom-0 h-24 bg-[radial-gradient(circle_at_top,rgba(79,227,255,0.08),transparent_62%)]" />
      <div className="max-w-7xl mx-auto px-4 sm:px-6 h-[78px] flex items-center justify-between gap-4">
        <Link href="/" className="flex items-center gap-3 hover:opacity-90 transition-opacity">
          <div className="relative w-10 h-10 rounded-2xl bg-[linear-gradient(135deg,#ff7a1a,#ffb347)] shadow-[0_0_24px_rgba(255,122,26,0.35)] flex items-center justify-center">
            <span className="absolute inset-0 rounded-2xl border border-white/20" />
            <Shield size={14} className="text-white" />
          </div>
          <div>
            <span className="block font-display font-semibold text-slate-50 text-sm tracking-[0.12em] uppercase">StableGuard</span>
            <span className="text-[11px] text-slate-400 font-normal hidden lg:block">
              Autonomous Treasury Control Surface
            </span>
          </div>
          <span className="hidden xl:inline data-chip text-[10px] uppercase tracking-[0.22em] text-cyan-200/80 px-2.5 py-1 rounded-full">
            Solana / Web3 Ops
          </span>
        </Link>

        <div className="hidden lg:flex items-center gap-2">
          {navItems.map((item) => {
            const active = pathname === item.href;
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`rounded-full px-3 py-1.5 text-[11px] uppercase tracking-[0.18em] transition-all ${
                  active
                    ? "status-node text-cyan-100 shadow-[0_0_0_1px_rgba(79,227,255,0.12)]"
                    : "text-slate-400 hover:text-slate-100"
                }`}
              >
                {item.label}
              </Link>
            );
          })}
        </div>

        <div className="flex items-center gap-2 sm:gap-3">
          <WalletButton />
          <CommandBar />

          {lastUpdate && (
            <span className="hidden lg:inline rounded-full border border-white/8 bg-white/[0.03] px-3 py-1.5 text-[11px] uppercase tracking-[0.18em] text-slate-400">
              Sync {lastUpdate}
            </span>
          )}

          <div className="data-chip flex items-center gap-2 rounded-full px-3 py-1.5">
            {connected ? (
              <>
                <span className="relative flex h-2.5 w-2.5">
                  <span className="absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75 animate-ping" />
                  <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-emerald-300" />
                </span>
                <span className="text-[11px] uppercase tracking-[0.18em] text-slate-200">{modeLabel}</span>
              </>
            ) : (
              <>
                <span className="w-2.5 h-2.5 rounded-full bg-slate-500" />
                <span className="text-[11px] uppercase tracking-[0.18em] text-slate-400">{modeLabel}</span>
              </>
            )}
          </div>

          {onRefresh && (
            <button
              onClick={onRefresh}
              disabled={refreshing}
              className="p-2.5 rounded-2xl border border-white/10 bg-white/4 hover:bg-white/8 text-slate-300 hover:text-white transition-colors disabled:opacity-40"
              title="Refresh"
            >
              <RefreshCw size={14} className={refreshing ? "animate-spin" : ""} />
            </button>
          )}

          <Link
            href="/settings"
            className="p-2.5 rounded-2xl border border-white/10 bg-white/4 hover:bg-white/8 text-slate-300 hover:text-white transition-colors"
            title="Settings"
          >
            <Settings size={14} />
          </Link>
        </div>
      </div>
      <div className="stage-divider max-w-7xl mx-auto px-4 sm:px-6 pb-3">
        <div className="status-ribbon">
          <div className="status-node rounded-full px-3 py-1.5 text-[11px] uppercase tracking-[0.18em] text-cyan-100">
            Oracle stream {modeLabel}
          </div>
          <div className="status-node rounded-full px-3 py-1.5 text-[11px] uppercase tracking-[0.18em] text-orange-100">
            Operator shell active
          </div>
          {lastUpdate ? (
            <div className="status-node rounded-full px-3 py-1.5 text-[11px] uppercase tracking-[0.18em] text-slate-200">
              Snapshot {lastUpdate}
            </div>
          ) : null}
        </div>
      </div>
    </header>
  );
}
