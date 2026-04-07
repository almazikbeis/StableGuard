"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  GitBranch,
  Brain,
  Layers,
  CheckCircle2,
  ExternalLink,
  Loader2,
  Play,
  AlertTriangle,
  ArrowRight,
} from "lucide-react";
import { ASSET_OPTIONS } from "@/lib/assets";

const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

interface RebalanceStep {
  step: number;
  name: string;
  description: string;
  sig: string;
  explorer: string;
  input?: number;
  output?: number;
}

interface RebalanceResult {
  from_index: number;
  to_index: number;
  amount: number;
  steps: RebalanceStep[];
  jupiter_quote?: Record<string, unknown>;
  message: string;
  note: string;
  error?: string;
}

const STEP_META = [
  {
    icon: <GitBranch size={13} />,
    label: "execute_rebalance",
    sub: "Intent recorded on-chain",
    color: "text-orange-400",
    borderActive: "border-orange-300/25 bg-orange-400/8",
  },
  {
    icon: <Brain size={13} />,
    label: "record_decision",
    sub: "AI decision immutable PDA",
    color: "text-purple-400",
    borderActive: "border-purple-300/25 bg-purple-400/8",
  },
  {
    icon: <Layers size={13} />,
    label: "record_swap_result",
    sub: "Settlement receipt on-chain",
    color: "text-cyan-400",
    borderActive: "border-cyan-300/25 bg-cyan-400/8",
  },
];

export function OnChainRebalance() {
  const [running, setRunning] = useState(false);
  const [result, setResult] = useState<RebalanceResult | null>(null);
  const [activeStep, setActiveStep] = useState(-1);
  const [fromIndex, setFromIndex] = useState(1);
  const [toIndex, setToIndex] = useState(0);
  const [amount, setAmount] = useState("500");

  const fromAsset = ASSET_OPTIONS.find((asset) => asset.index === fromIndex) ?? ASSET_OPTIONS[1];
  const toAsset = ASSET_OPTIONS.find((asset) => asset.index === toIndex) ?? ASSET_OPTIONS[0];

  const run = async () => {
    if (running) return;
    if (fromIndex === toIndex) {
      setResult({ error: "Choose different source and target assets." } as RebalanceResult);
      setActiveStep(-1);
      return;
    }

    const amountNum = Number(amount);
    if (!Number.isFinite(amountNum) || amountNum <= 0) {
      setResult({ error: "Enter a valid asset amount greater than zero." } as RebalanceResult);
      setActiveStep(-1);
      return;
    }

    setRunning(true);
    setResult(null);
    setActiveStep(0);

    try {
      const rawAmount = Math.floor(amountNum * Math.pow(10, fromAsset.decimals));
      const res = await fetch(`${BASE}/demo/full-rebalance`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          from_index: fromIndex,
          to_index: toIndex,
          amount: rawAmount,
          rationale: `AI routed capital from ${fromAsset.symbol} to ${toAsset.symbol} based on the current treasury risk/yield signal.`,
          confidence: 85,
        }),
      });

      // Animate through steps as we wait
      await new Promise((r) => setTimeout(r, 1200));
      setActiveStep(1);
      await new Promise((r) => setTimeout(r, 800));
      setActiveStep(2);

      const data: RebalanceResult = await res.json();
      setResult(data);
      setActiveStep(data.error ? -1 : 3);
    } catch (e: unknown) {
      setResult({ error: e instanceof Error ? e.message : "Request failed" } as RebalanceResult);
      setActiveStep(-1);
    } finally {
      setRunning(false);
    }
  };

  return (
    <div className="panel-surface rounded-[24px] p-5 space-y-4">
      {/* Header */}
      <div>
        <h3 className="font-display font-bold text-slate-50 text-sm flex items-center gap-1.5">
          <Layers size={14} className="text-cyan-400" />
          On-Chain Route · 3 TX Audit Trail
        </h3>
        <p className="text-xs text-slate-400 mt-0.5">
          Intent → AI decision → settlement receipt for any supported treasury asset pair
        </p>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div>
          <label className="block text-[10px] uppercase tracking-[0.16em] text-slate-500 mb-1.5">From</label>
          <select
            value={fromIndex}
            onChange={(event) => setFromIndex(Number(event.target.value))}
            className="w-full rounded-xl border border-white/10 bg-white/5 px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-cyan-300/30"
          >
            {ASSET_OPTIONS.map((asset) => (
              <option key={asset.symbol} value={asset.index} className="bg-slate-950">
                {asset.symbol}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-[10px] uppercase tracking-[0.16em] text-slate-500 mb-1.5">To</label>
          <select
            value={toIndex}
            onChange={(event) => setToIndex(Number(event.target.value))}
            className="w-full rounded-xl border border-white/10 bg-white/5 px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-cyan-300/30"
          >
            {ASSET_OPTIONS.map((asset) => (
              <option key={asset.symbol} value={asset.index} className="bg-slate-950">
                {asset.symbol}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-[10px] uppercase tracking-[0.16em] text-slate-500 mb-1.5">
            Amount ({fromAsset.symbol})
          </label>
          <input
            type="number"
            min="0"
            step="0.000001"
            value={amount}
            onChange={(event) => setAmount(event.target.value)}
            className="w-full rounded-xl border border-white/10 bg-white/5 px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-cyan-300/30"
          />
        </div>
      </div>

      {/* Steps */}
      <div className="grid grid-cols-3 gap-2">
        {STEP_META.map((s, i) => {
          const done = activeStep > i;
          const current = activeStep === i && running;
          return (
            <motion.div
              key={i}
              animate={current ? { scale: [1, 1.03, 1] } : {}}
              transition={{ repeat: current ? Infinity : 0, duration: 1.2 }}
              className={`rounded-xl border p-3 transition-all ${
                done
                  ? s.borderActive
                  : current
                  ? "border-white/15 bg-white/5"
                  : "border-white/8 bg-white/3"
              }`}
            >
              <div className="flex items-center gap-1.5 mb-1">
                <span className={done || current ? s.color : "text-slate-600"}>{s.icon}</span>
                <span
                  className={`w-1.5 h-1.5 rounded-full ${
                    done ? "bg-emerald-400" : current ? "bg-orange-400 animate-pulse" : "bg-slate-700"
                  }`}
                />
              </div>
              <div className="text-[10px] font-semibold text-slate-200 font-mono">{s.label}</div>
              <div className="text-[9px] text-slate-500 mt-0.5">{s.sub}</div>
            </motion.div>
          );
        })}
      </div>

      {/* Jupiter quote note */}
      <div className="flex items-center gap-2 bg-blue-400/6 border border-blue-400/15 rounded-xl px-3 py-2">
        <ArrowRight size={10} className="text-blue-400 flex-shrink-0" />
        <p className="text-[10px] text-blue-300">
          Swap route and settlement receipts adapt to the selected asset pair. Devnet may still fall back to demo-safe execution paths.
        </p>
      </div>

      {/* Run button */}
      <button
        onClick={run}
        disabled={running}
        className={`w-full rounded-xl py-3 text-sm font-semibold tracking-wide transition-all flex items-center justify-center gap-2 ${
          running
            ? "bg-white/5 text-slate-500 cursor-not-allowed"
            : "bg-cyan-500/15 border border-cyan-400/25 text-cyan-200 hover:bg-cyan-500/25"
        }`}
        >
        {running ? (
          <>
            <Loader2 size={14} className="animate-spin" />
            Executing {fromAsset.symbol} → {toAsset.symbol}…
          </>
        ) : (
          <>
            <Play size={14} />
            Execute {fromAsset.symbol} → {toAsset.symbol}
          </>
        )}
      </button>

      {/* Result */}
      <AnimatePresence>
        {result && (
          <motion.div
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0 }}
            className="space-y-2"
          >
            {result.error && !result.steps?.length ? (
              <div className="flex items-start gap-2 rounded-xl border border-red-400/20 bg-red-400/6 p-3">
                <AlertTriangle size={13} className="text-red-400 flex-shrink-0 mt-0.5" />
                <div>
                  <p className="text-xs font-semibold text-red-300">Rebalance failed</p>
                  <p className="text-[10px] text-red-400/80 mt-0.5">{result.error}</p>
                  <p className="text-[10px] text-slate-500 mt-1">
                    Initialize vault first: POST /api/v1/demo/init-vault
                  </p>
                </div>
              </div>
            ) : (
              <>
                <div className="flex items-center gap-2">
                  <CheckCircle2 size={13} className="text-emerald-400" />
                  <span className="text-xs font-semibold text-emerald-300">
                    {result.steps?.length ?? 0} on-chain transactions confirmed
                  </span>
                </div>

                {result.steps?.map((step) => (
                  <a
                    key={step.step}
                    href={step.explorer}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-2 text-[10px] font-mono rounded-xl border border-white/8 bg-white/3 hover:bg-white/6 px-3 py-2 transition-colors group"
                  >
                    <span className="text-slate-600 w-4">{step.step}.</span>
                    <span className="text-slate-300 font-semibold w-32 truncate">{step.name}</span>
                    <span className="text-slate-500 truncate flex-1">{step.sig.slice(0, 12)}…</span>
                    <ExternalLink size={9} className="text-slate-600 group-hover:text-cyan-400 transition-colors flex-shrink-0" />
                  </a>
                ))}

                {result.note && (
                  <p className="text-[10px] text-slate-500 italic pl-1">{result.note}</p>
                )}
              </>
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
