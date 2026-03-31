"use client";

import { Shield, RefreshCw, Settings, Wifi, WifiOff } from "lucide-react";
import Link from "next/link";

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
    <header className="sticky top-0 z-50 bg-white border-b border-gray-200">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 h-14 flex items-center justify-between">
        <Link href="/" className="flex items-center gap-2.5 hover:opacity-80 transition-opacity">
          <div className="w-7 h-7 rounded-lg bg-orange-500 flex items-center justify-center">
            <Shield size={14} className="text-white" />
          </div>
          <span className="font-semibold text-gray-900 text-sm">StableGuard</span>
          <span className="text-xs text-gray-400 font-normal hidden sm:inline">
            · Stablecoin Risk Monitor
          </span>
        </Link>

        <div className="flex items-center gap-3">
          {lastUpdate && (
            <span className="text-xs text-gray-400 hidden sm:inline">
              Updated {lastUpdate}
            </span>
          )}

          {/* Connection indicator */}
          <div className="flex items-center gap-1.5">
            {connected ? (
              <>
                <span className="w-2 h-2 rounded-full bg-green-500 animate-pulse" />
                <span className="text-xs text-gray-500">{modeLabel}</span>
              </>
            ) : (
              <>
                <span className="w-2 h-2 rounded-full bg-gray-300" />
                <span className="text-xs text-gray-400">{modeLabel}</span>
              </>
            )}
          </div>

          {onRefresh && (
            <button
              onClick={onRefresh}
              disabled={refreshing}
              className="p-1.5 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors disabled:opacity-40"
              title="Refresh"
            >
              <RefreshCw size={14} className={refreshing ? "animate-spin" : ""} />
            </button>
          )}

          <Link
            href="/settings"
            className="p-1.5 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors"
            title="Settings"
          >
            <Settings size={14} />
          </Link>
        </div>
      </div>
    </header>
  );
}
