"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  Zap,
  Brain,
  Shield,
  CheckCircle2,
  AlertTriangle,
  ExternalLink,
  Activity,
  Bot,
  Loader2,
  Play,
} from "lucide-react";

interface SimScore {
  risk_level: number;
  deviation_pct: number;
  action: string;
}

interface SimDecision {
  action: string;
  rationale: string;
  confidence: number;
  from_index: number;
  to_index: number;
}

interface SimResult {
  depeg_pct: number;
  prices: Record<string, number>;
  score: SimScore;
  decision: SimDecision;
  on_chain_sig: string;
  explorer_url: string;
  error?: string;
}

type Stage = "idle" | "injecting" | "scoring" | "ai" | "onchain" | "done" | "error";

const STAGE_LABELS: Record<Stage, string> = {
  idle: "Ready",
  injecting: "Injecting fake price...",
  scoring: "Computing risk v2...",
  ai: "Running AI agents...",
  onchain: "Writing record_decision to Solana...",
  done: "Complete",
  error: "Failed",
};

const PRESET_LEVELS = [
  { label: "0.5%", pct: 0.5, tone: "text-amber-300" },
  { label: "1.5%", pct: 1.5, tone: "text-orange-400" },
  { label: "2.0%", pct: 2.0, tone: "text-red-400" },
  { label: "3.5%", pct: 3.5, tone: "text-red-500" },
];

async function sleep(ms: number) {
  return new Promise((r) => setTimeout(r, ms));
}

export function DepegSimulator() {
  const [stage, setStage] = useState<Stage>("idle");
  const [depegPct, setDepegPct] = useState(2.0);
  const [result, setResult] = useState<SimResult | null>(null);

  const run = async () => {
    if (stage !== "idle" && stage !== "done" && stage !== "error") return;

    setResult(null);
    setStage("injecting");
    await sleep(600);
    setStage("scoring");
    await sleep(700);
    setStage("ai");

    try {
      const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";
      const res = await fetch(`${BASE}/demo/simulate-depeg`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ depeg_pct: depegPct }),
      });
      if (!res.ok) throw new Error(`API ${res.status}`);
      const data: SimResult = await res.json();

      setStage("onchain");
      await sleep(500);

      if (data.error && !data.on_chain_sig) {
        setStage("error");
        setResult(data);
        return;
      }

      setResult(data);
      setStage("done");
    } catch (e: unknown) {
      setResult({ error: e instanceof Error ? e.message : String(e) } as SimResult);
      setStage("error");
    }
  };

  const actionColor = (a?: string) => {
    if (a === "PROTECT") return "text-red-400";
    if (a === "OPTIMIZE") return "text-cyan-300";
    return "text-slate-300";
  };

  const isRunning = stage !== "idle" && stage !== "done" && stage !== "error";

  return (
    <div className="panel-surface rounded-[24px] p-5 space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h3 className="font-display font-bold text-slate-50 text-sm flex items-center gap-1.5">
            <Zap size={14} className="text-orange-400" />
            Stablecoin Shock Demo
          </h3>
          <p className="text-xs text-slate-400 mt-0.5">
            Stablecoin-specific scenario: inject a depeg event → AI agents decide → on-chain proof on Solana
          </p>
        </div>
      </div>

      {/* Depeg level selector */}
      <div>
        <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500 mb-2">
          USDT Depeg magnitude
        </div>
        <div className="flex gap-2 flex-wrap">
          {PRESET_LEVELS.map((p) => (
            <button
              key={p.pct}
              onClick={() => setDepegPct(p.pct)}
              className={`rounded-full border px-3 py-1 text-xs font-semibold transition-all ${
                depegPct === p.pct
                  ? "border-white/30 bg-white/10 " + p.tone
                  : "border-white/10 text-slate-400 hover:border-white/20"
              }`}
            >
              {p.label}
            </button>
          ))}
          <div className="flex items-center gap-2 ml-2">
            <span className="text-[10px] text-slate-500">Custom:</span>
            <input
              type="number"
              min={0.1}
              max={10}
              step={0.1}
              value={depegPct}
              onChange={(e) => setDepegPct(parseFloat(e.target.value) || 2.0)}
              className="w-16 bg-white/5 border border-white/10 rounded-lg px-2 py-1 text-xs text-slate-200 focus:outline-none focus:border-white/30"
            />
            <span className="text-[10px] text-slate-500">%</span>
          </div>
        </div>
      </div>

      {/* Pipeline stages */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
        {(
          [
            { key: "injecting", icon: <Activity size={13} />, label: "Price Injected", sub: `USDT $${(1 - depegPct / 100).toFixed(4)}` },
            { key: "scoring", icon: <Shield size={13} />, label: "Risk Scored", sub: result ? `${result.score?.risk_level?.toFixed(0)}/100` : "v2 scorer" },
            { key: "ai", icon: <Brain size={13} />, label: "AI Agents", sub: result?.decision ? result.decision.action : "3 Claude agents" },
            { key: "onchain", icon: <Bot size={13} />, label: "On-Chain", sub: result?.on_chain_sig ? result.on_chain_sig.slice(0, 8) + "…" : "record_decision" },
          ] as { key: Stage; icon: React.ReactNode; label: string; sub: string }[]
        ).map((s, i) => {
          const stageOrder: Stage[] = ["injecting", "scoring", "ai", "onchain", "done"];
          const currentIdx = stageOrder.indexOf(stage);
          const thisIdx = stageOrder.indexOf(s.key as Stage);
          const active = currentIdx >= thisIdx && stage !== "idle";
          const current = stage === s.key;

          return (
            <motion.div
              key={s.key}
              animate={current ? { scale: [1, 1.03, 1] } : {}}
              transition={{ repeat: current ? Infinity : 0, duration: 1.2 }}
              className={`rounded-xl border p-3 transition-all ${
                active
                  ? "border-emerald-300/25 bg-emerald-400/8"
                  : current
                  ? "border-orange-300/25 bg-orange-400/8"
                  : "border-white/8 bg-white/3"
              }`}
            >
              <div className="flex items-center gap-1.5 mb-1">
                <span className={active ? "text-emerald-400" : "text-slate-500"}>{s.icon}</span>
                <span className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${active ? "bg-emerald-400" : current ? "bg-orange-400 animate-pulse" : "bg-slate-600"}`} />
                <span className="text-[9px] font-mono text-slate-500">{i + 1}</span>
              </div>
              <div className="text-[11px] font-semibold text-slate-100">{s.label}</div>
              <div className="text-[9px] text-slate-400 mt-0.5 font-mono truncate">{s.sub}</div>
            </motion.div>
          );
        })}
      </div>

      {/* Run button */}
      <button
        onClick={run}
        disabled={isRunning}
        className={`w-full rounded-xl py-3 text-sm font-semibold tracking-wide transition-all flex items-center justify-center gap-2 ${
          isRunning
            ? "bg-white/5 text-slate-500 cursor-not-allowed"
            : "bg-orange-500/20 border border-orange-400/30 text-orange-200 hover:bg-orange-500/30"
        }`}
      >
        {isRunning ? (
          <>
            <Loader2 size={14} className="animate-spin" />
            {STAGE_LABELS[stage]}
          </>
        ) : (
          <>
            <Play size={14} />
            Simulate {depegPct}% USDT Depeg
          </>
        )}
      </button>

      {/* Result */}
      <AnimatePresence>
        {(stage === "done" || stage === "error") && result && (
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0 }}
            className={`rounded-xl border p-4 space-y-3 ${
              stage === "error"
                ? "border-red-400/20 bg-red-400/8"
                : "border-emerald-300/20 bg-emerald-400/8"
            }`}
          >
            {stage === "error" ? (
              <div className="flex items-center gap-2 text-red-300 text-sm">
                <AlertTriangle size={14} />
                {result.error || "Simulation failed"}
              </div>
            ) : (
              <>
                <div className="flex items-center gap-2">
                  <CheckCircle2 size={14} className="text-emerald-400" />
                  <span className="text-emerald-300 text-sm font-semibold">Autonomous loop complete</span>
                </div>

                <div className="grid grid-cols-3 gap-2">
                  <div className="bg-white/5 rounded-lg p-2">
                    <div className="text-[9px] text-slate-500 uppercase tracking-wide">Risk Score</div>
                    <div className="text-lg font-display text-red-300">{result.score?.risk_level?.toFixed(0)}/100</div>
                  </div>
                  <div className="bg-white/5 rounded-lg p-2">
                    <div className="text-[9px] text-slate-500 uppercase tracking-wide">AI Decision</div>
                    <div className={`text-lg font-display ${actionColor(result.decision?.action)}`}>
                      {result.decision?.action || "—"}
                    </div>
                  </div>
                  <div className="bg-white/5 rounded-lg p-2">
                    <div className="text-[9px] text-slate-500 uppercase tracking-wide">Confidence</div>
                    <div className="text-lg font-display text-slate-200">{result.decision?.confidence ?? 0}%</div>
                  </div>
                </div>

                {result.decision?.rationale && (
                  <div className="text-xs text-slate-400 italic border-l-2 border-white/10 pl-3">
                    {result.decision.rationale}
                  </div>
                )}

                {result.on_chain_sig ? (
                  <a
                    href={result.explorer_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-2 text-xs text-cyan-300 hover:text-cyan-200 transition-colors font-mono bg-white/5 rounded-lg px-3 py-2"
                  >
                    <Bot size={12} />
                    <span className="truncate">On-chain proof: {result.on_chain_sig}</span>
                    <ExternalLink size={10} className="flex-shrink-0 ml-auto" />
                  </a>
                ) : result.error ? (
                  <div className="text-xs text-amber-400/80 border-l-2 border-amber-400/20 pl-3">
                    {result.error}
                  </div>
                ) : null}
              </>
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
