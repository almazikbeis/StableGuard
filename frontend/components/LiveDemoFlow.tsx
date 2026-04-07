"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  Brain, Shield, Bot, Play, Loader2, CheckCircle2,
  AlertTriangle, ExternalLink, ArrowRight, Activity,
  GitBranch, Layers, ChevronRight, TrendingDown,
} from "lucide-react";

const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

interface SimResult {
  kind?: "depeg" | "crash";
  magnitude_pct?: number;
  depeg_pct?: number;
  asset?: string;
  crash_pct?: number;
  price_before?: number;
  price_after?: number;
  score: { risk_level: number; deviation_pct: number; action: string };
  decision: { action: string; rationale: string; confidence: number; from_index: number; to_index: number };
  on_chain_sig: string;
  explorer_url: string;
  error?: string;
}

interface RebalanceStep {
  step: number;
  name: string;
  description: string;
  sig: string;
  explorer: string;
}

interface RebalanceResult {
  steps: RebalanceStep[];
  message: string;
  note: string;
  error?: string;
}

type FlowStage = "idle" | "simulating" | "sim_done" | "rebalancing" | "all_done" | "error";
type ScenarioMode = "depeg" | "crash";

const DEPEG_PRESETS = [
  { label: "0.5%", pct: 0.5, color: "text-amber-300",  bg: "border-amber-400/30 bg-amber-400/8" },
  { label: "1.5%", pct: 1.5, color: "text-orange-400", bg: "border-orange-400/30 bg-orange-400/8" },
  { label: "2.0%", pct: 2.0, color: "text-red-400",    bg: "border-red-400/30 bg-red-400/8" },
  { label: "3.5%", pct: 3.5, color: "text-red-500",    bg: "border-red-500/30 bg-red-500/8" },
];

const CRASH_PRESETS = [
  { label: "BTC -5%",  asset: "BTC", pct: 5,  color: "text-amber-300",  bg: "border-amber-400/30 bg-amber-400/8" },
  { label: "BTC -15%", asset: "BTC", pct: 15, color: "text-orange-400", bg: "border-orange-400/30 bg-orange-400/8" },
  { label: "ETH -20%", asset: "ETH", pct: 20, color: "text-red-400",    bg: "border-red-400/30 bg-red-400/8" },
  { label: "SOL -30%", asset: "SOL", pct: 30, color: "text-red-500",    bg: "border-red-500/30 bg-red-500/8" },
];

function sleep(ms: number) { return new Promise(r => setTimeout(r, ms)); }

function actionColor(a?: string) {
  if (a === "PROTECT")  return "text-red-300";
  if (a === "OPTIMIZE") return "text-cyan-300";
  return "text-emerald-300";
}

function formatPrice(price: number, asset: string) {
  if (asset === "BTC" || price > 1000) return `$${price.toLocaleString("en-US", { maximumFractionDigits: 0 })}`;
  if (price > 10) return `$${price.toFixed(2)}`;
  return `$${price.toFixed(4)}`;
}

export function LiveDemoFlow() {
  const [mode, setMode] = useState<ScenarioMode>("depeg");

  // Depeg state
  const [depegPct, setDepegPct]   = useState(2.0);
  // Crash state
  const [crashAsset, setCrashAsset] = useState("BTC");
  const [crashPct, setCrashPct]     = useState(15);
  const selectedCrashPreset = CRASH_PRESETS.find(p => p.asset === crashAsset && p.pct === crashPct) ?? CRASH_PRESETS[1];

  const [stage, setStage]       = useState<FlowStage>("idle");
  const [simStage, setSimStage] = useState(0);
  const [sim, setSim]           = useState<SimResult | null>(null);
  const [rebalance, setRebalance] = useState<RebalanceResult | null>(null);
  const [error, setError]       = useState<string | null>(null);

  const selectedDepegPreset = DEPEG_PRESETS.find(p => p.pct === depegPct) ?? DEPEG_PRESETS[2];
  const activeColor = mode === "crash" ? selectedCrashPreset.color : selectedDepegPreset.color;
  const activeBg    = mode === "crash" ? selectedCrashPreset.bg    : selectedDepegPreset.bg;

  const runSim = async () => {
    setStage("simulating");
    setSimStage(0);
    setSim(null);
    setRebalance(null);
    setError(null);

    try {
      setSimStage(1); await sleep(500);
      setSimStage(2); await sleep(600);
      setSimStage(3);

      const res = await fetch(`${BASE}/demo/simulate-event`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(
          mode === "depeg"
            ? { kind: "depeg", magnitude_pct: depegPct }
            : { kind: "crash", asset: crashAsset, magnitude_pct: crashPct },
        ),
      });

      const data: SimResult = await res.json();

      if (data.error && !data.on_chain_sig) {
        setError(data.error);
        setStage("error");
        return;
      }

      setSim(data);
      setStage("sim_done");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Request failed");
      setStage("error");
    }
  };

  const runRebalance = async () => {
    if (!sim) return;
    setStage("rebalancing");
    try {
      const res = await fetch(`${BASE}/demo/full-rebalance`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          from_index: sim.decision.from_index >= 0 ? sim.decision.from_index : 1,
          to_index:   sim.decision.to_index   >= 0 ? sim.decision.to_index   : 0,
          amount:     500_000_000,
          action:     sim.decision.action,
          rationale:  sim.decision.rationale,
          confidence: sim.decision.confidence,
        }),
      });
      const data: RebalanceResult = await res.json();
      setRebalance(data);
      setStage("all_done");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Rebalance failed");
      setStage("error");
    }
  };

  const reset = () => {
    setStage("idle"); setSimStage(0);
    setSim(null); setRebalance(null); setError(null);
  };

  const isRunning = stage === "simulating" || stage === "rebalancing";

  return (
    <div className="panel-surface rounded-[28px] p-5 sm:p-6 space-y-5">
      {/* ── Header ── */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <h3 className="font-display font-bold text-slate-50 text-base flex items-center gap-2">
            <span className="relative flex h-2.5 w-2.5 mr-0.5">
              <span className="absolute inline-flex h-full w-full rounded-full bg-orange-400 opacity-75 animate-ping" />
              <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-orange-300" />
            </span>
            Autonomous AI Loop · Live Demo
          </h3>
          <p className="text-xs text-slate-400 mt-0.5">
            Inject a market event → Claude AI agents decide → 3 on-chain TXs prove it on Solana
          </p>
        </div>
        {(stage === "sim_done" || stage === "all_done" || stage === "error") && (
          <button
            onClick={reset}
            className="text-[10px] uppercase tracking-[0.18em] text-slate-500 hover:text-slate-300 transition-colors border border-white/10 rounded-full px-3 py-1.5"
          >
            Reset
          </button>
        )}
      </div>

      {/* ── Mode toggle ── */}
      <div className="flex gap-1 p-1 rounded-full bg-white/[0.04] border border-white/8 w-fit">
        {(["depeg", "crash"] as ScenarioMode[]).map((m) => (
          <button
            key={m}
            disabled={isRunning || stage !== "idle"}
            onClick={() => { setMode(m); reset(); }}
            className={`rounded-full px-4 py-1.5 text-xs font-semibold uppercase tracking-[0.12em] transition-all ${
              mode === m
                ? "bg-white text-slate-950 shadow-sm"
                : "text-slate-400 hover:text-slate-200"
            }`}
          >
            {m === "depeg" ? "Stablecoin Depeg" : "Crypto Crash"}
          </button>
        ))}
      </div>

      {/* ── Step 1: Choose scenario ── */}
      <div className={`rounded-[20px] border p-4 space-y-4 transition-all ${stage !== "idle" ? "border-white/8 opacity-70" : "border-white/12 bg-white/[0.02]"}`}>
        <div className="flex items-center gap-2">
          <span className="w-5 h-5 rounded-full bg-orange-500/20 border border-orange-400/30 flex items-center justify-center text-[10px] font-bold text-orange-300">1</span>
          <span className="text-xs font-semibold text-slate-200 uppercase tracking-[0.14em]">
            {mode === "depeg" ? "Choose reserve shock magnitude" : "Choose crash scenario"}
          </span>
        </div>

        {mode === "depeg" ? (
          <div className="flex flex-wrap gap-2">
            {DEPEG_PRESETS.map(p => (
              <button
                key={p.pct}
                disabled={isRunning}
                onClick={() => setDepegPct(p.pct)}
                className={`rounded-full border px-3 py-1.5 text-xs font-semibold transition-all ${
                  depegPct === p.pct ? `${p.bg} ${p.color}` : "border-white/10 text-slate-400 hover:border-white/20 hover:text-slate-200"
                }`}
              >
                -{p.label}
              </button>
            ))}
          </div>
        ) : (
          <div className="flex flex-wrap gap-2">
            {CRASH_PRESETS.map(p => (
              <button
                key={`${p.asset}-${p.pct}`}
                disabled={isRunning}
                onClick={() => { setCrashAsset(p.asset); setCrashPct(p.pct); }}
                className={`rounded-full border px-3 py-1.5 text-xs font-semibold transition-all flex items-center gap-1 ${
                  crashAsset === p.asset && crashPct === p.pct
                    ? `${p.bg} ${p.color}`
                    : "border-white/10 text-slate-400 hover:border-white/20 hover:text-slate-200"
                }`}
              >
                <TrendingDown size={10} />
                {p.label}
              </button>
            ))}
          </div>
        )}

        {/* Pipeline animation dots */}
        <div className="grid grid-cols-4 gap-2">
          {[
            {
              icon: <Activity size={11} />,
              label: mode === "depeg" ? "Price injected" : "Crash injected",
              val: mode === "depeg"
                ? `$${(1 - depegPct / 100).toFixed(4)}`
                : sim ? formatPrice(sim.price_after ?? 0, crashAsset) : `${crashAsset} -${crashPct}%`,
            },
            { icon: <Shield size={11} />, label: "Risk v2 scored",  val: sim ? `${sim.score?.risk_level?.toFixed(0)}/100` : "0–100" },
            { icon: <Brain size={11} />,  label: "AI agents",       val: sim?.decision?.action ?? "3 Claude" },
            { icon: <Bot size={11} />,    label: "On-chain proof",  val: sim?.on_chain_sig ? sim.on_chain_sig.slice(0, 6) + "…" : "PDA" },
          ].map((s, i) => {
            const done   = simStage > i || stage === "sim_done" || stage === "rebalancing" || stage === "all_done";
            const active = simStage === i + 1 && stage === "simulating";
            return (
              <motion.div
                key={i}
                animate={active ? { scale: [1, 1.04, 1] } : {}}
                transition={{ repeat: active ? Infinity : 0, duration: 1.1 }}
                className={`rounded-xl border p-2.5 transition-all ${done ? "border-emerald-300/20 bg-emerald-400/6" : active ? "border-orange-300/20 bg-orange-400/6" : "border-white/8 bg-white/2"}`}
              >
                <div className="flex items-center gap-1 mb-1">
                  <span className={done ? "text-emerald-400" : active ? "text-orange-400" : "text-slate-600"}>{s.icon}</span>
                  <span className={`w-1.5 h-1.5 rounded-full ${done ? "bg-emerald-400" : active ? "bg-orange-400 animate-pulse" : "bg-slate-700"}`} />
                </div>
                <div className="text-[10px] font-semibold text-slate-300 truncate">{s.label}</div>
                <div className="text-[9px] text-slate-500 mt-0.5 font-mono truncate">{s.val}</div>
              </motion.div>
            );
          })}
        </div>

        <button
          onClick={stage === "idle" ? runSim : undefined}
          disabled={stage !== "idle" || isRunning}
          className={`w-full rounded-xl py-3 text-sm font-semibold tracking-wide transition-all flex items-center justify-center gap-2 ${
            stage !== "idle"
              ? "bg-white/5 text-slate-500 cursor-not-allowed"
              : `${activeBg} ${activeColor} hover:opacity-90 border`
          }`}
        >
          {stage === "simulating" ? (
            <><Loader2 size={14} className="animate-spin" /> Running autonomous loop…</>
          ) : stage !== "idle" ? (
            <><CheckCircle2 size={14} className="text-emerald-400" /> Loop completed</>
          ) : mode === "depeg" ? (
            <><Play size={14} /> Simulate {depegPct}% Reserve Depeg</>
          ) : (
            <><TrendingDown size={14} /> Simulate {crashAsset} -{crashPct}% Crash</>
          )}
        </button>
      </div>

      {/* ── Step 2: AI Result ── */}
      <AnimatePresence>
        {sim && (stage === "sim_done" || stage === "rebalancing" || stage === "all_done") && (
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            className="rounded-[20px] border border-white/12 bg-white/[0.02] p-4 space-y-3"
          >
            <div className="flex items-center gap-2">
              <span className="w-5 h-5 rounded-full bg-purple-500/20 border border-purple-400/30 flex items-center justify-center text-[10px] font-bold text-purple-300">2</span>
              <span className="text-xs font-semibold text-slate-200 uppercase tracking-[0.14em]">AI Decision</span>
              <CheckCircle2 size={12} className="text-emerald-400 ml-auto" />
            </div>

            {/* Crash price info */}
            {mode === "crash" && sim.price_before != null && sim.price_after != null && (
              <div className="flex items-center gap-3 rounded-xl bg-red-400/6 border border-red-400/15 px-3 py-2">
                <TrendingDown size={13} className="text-red-400 flex-shrink-0" />
                <span className="text-xs text-slate-300">
                  <span className="font-semibold text-red-300">{sim.asset}</span>
                  {" "}{formatPrice(sim.price_before, sim.asset ?? "BTC")}
                  {" → "}
                  <span className="text-red-400 font-bold">{formatPrice(sim.price_after, sim.asset ?? "BTC")}</span>
                  <span className="text-slate-500 ml-1">(-{sim.crash_pct?.toFixed(0)}%)</span>
                </span>
              </div>
            )}

            <div className="grid grid-cols-3 gap-2">
              <div className="bg-white/5 rounded-xl p-3">
                <div className="text-[9px] text-slate-500 uppercase tracking-wide mb-1">Risk Score</div>
                <div className={`text-xl font-display ${sim.score?.risk_level > 60 ? "text-red-300" : "text-amber-300"}`}>
                  {sim.score?.risk_level?.toFixed(0)}<span className="text-sm text-slate-500">/100</span>
                </div>
              </div>
              <div className="bg-white/5 rounded-xl p-3">
                <div className="text-[9px] text-slate-500 uppercase tracking-wide mb-1">AI Action</div>
                <div className={`text-xl font-display ${actionColor(sim.decision?.action)}`}>
                  {sim.decision?.action ?? "—"}
                </div>
              </div>
              <div className="bg-white/5 rounded-xl p-3">
                <div className="text-[9px] text-slate-500 uppercase tracking-wide mb-1">Confidence</div>
                <div className="text-xl font-display text-slate-200">
                  {sim.decision?.confidence ?? 0}<span className="text-sm text-slate-500">%</span>
                </div>
              </div>
            </div>

            {sim.decision?.rationale && (
              <p className="text-[11px] text-slate-400 italic border-l-2 border-white/10 pl-3 leading-relaxed">
                {sim.decision.rationale}
              </p>
            )}

            {sim.on_chain_sig && (
              <a
                href={sim.explorer_url}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 text-[11px] text-cyan-300 hover:text-cyan-200 transition-colors font-mono bg-white/5 rounded-xl px-3 py-2.5 group"
              >
                <Bot size={11} className="flex-shrink-0" />
                <span className="text-slate-400">record_decision</span>
                <span className="truncate flex-1">{sim.on_chain_sig.slice(0, 20)}…</span>
                <ExternalLink size={9} className="flex-shrink-0 text-slate-600 group-hover:text-cyan-400 transition-colors" />
              </a>
            )}

            {stage === "sim_done" && sim.decision?.action !== "HOLD" && (
              <button
                onClick={runRebalance}
                className="w-full rounded-xl py-2.5 text-sm font-semibold tracking-wide bg-cyan-500/15 border border-cyan-400/25 text-cyan-200 hover:bg-cyan-500/25 transition-all flex items-center justify-center gap-2"
              >
                <ArrowRight size={14} />
                Execute On-Chain Rebalance (3 TXs)
              </button>
            )}
            {stage === "sim_done" && sim.decision?.action === "HOLD" && (
              <div className="text-center text-xs text-slate-500 py-1">
                Risk below threshold — AI decision: HOLD (no rebalance needed)
              </div>
            )}
          </motion.div>
        )}
      </AnimatePresence>

      {/* ── Step 3: On-Chain Rebalance ── */}
      <AnimatePresence>
        {(stage === "rebalancing" || stage === "all_done") && (
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            className="rounded-[20px] border border-white/12 bg-white/[0.02] p-4 space-y-3"
          >
            <div className="flex items-center gap-2">
              <span className="w-5 h-5 rounded-full bg-cyan-500/20 border border-cyan-400/30 flex items-center justify-center text-[10px] font-bold text-cyan-300">3</span>
              <span className="text-xs font-semibold text-slate-200 uppercase tracking-[0.14em]">On-Chain Execution · 3 TX Audit Trail</span>
              {stage === "rebalancing" && <Loader2 size={12} className="text-cyan-400 animate-spin ml-auto" />}
              {stage === "all_done" && <CheckCircle2 size={12} className="text-emerald-400 ml-auto" />}
            </div>

            {stage === "rebalancing" && (
              <div className="grid grid-cols-3 gap-2">
                {[
                  { icon: <GitBranch size={12} />, label: "execute_rebalance", color: "text-orange-400" },
                  { icon: <Brain size={12} />,     label: "record_decision",   color: "text-purple-400" },
                  { icon: <Layers size={12} />,    label: "record_swap_result",color: "text-cyan-400" },
                ].map((s, i) => (
                  <motion.div
                    key={i}
                    animate={{ scale: [1, 1.03, 1] }}
                    transition={{ repeat: Infinity, duration: 1.2, delay: i * 0.3 }}
                    className="rounded-xl border border-white/10 bg-white/3 p-2.5"
                  >
                    <span className={`${s.color} block mb-1`}>{s.icon}</span>
                    <div className="text-[9px] font-mono text-slate-400">{s.label}</div>
                  </motion.div>
                ))}
              </div>
            )}

            {stage === "all_done" && rebalance && (
              <>
                {rebalance.error ? (
                  <div className="flex items-start gap-2 rounded-xl border border-red-400/20 bg-red-400/6 p-3">
                    <AlertTriangle size={12} className="text-red-400 flex-shrink-0 mt-0.5" />
                    <div>
                      <p className="text-xs text-red-300">{rebalance.error}</p>
                      <p className="text-[10px] text-slate-500 mt-0.5">Call /demo/init-vault first if vault is not set up.</p>
                    </div>
                  </div>
                ) : (
                  <>
                    <div className="flex items-center gap-2 text-xs text-emerald-300 font-semibold">
                      <CheckCircle2 size={13} />
                      {rebalance.steps?.length ?? 0} transactions confirmed on Solana
                    </div>
                    <div className="space-y-1.5">
                      {rebalance.steps?.map(step => (
                        <a
                          key={step.step}
                          href={step.explorer}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="flex items-center gap-2 text-[10px] font-mono rounded-xl border border-white/8 bg-white/3 hover:bg-white/6 px-3 py-2 transition-colors group"
                        >
                          <span className="text-slate-600 w-4">{step.step}.</span>
                          <span className="text-slate-200 font-semibold w-36 truncate">{step.name}</span>
                          <span className="text-slate-500 truncate flex-1">{step.sig.slice(0, 14)}…</span>
                          <ExternalLink size={9} className="text-slate-600 group-hover:text-cyan-400 transition-colors flex-shrink-0" />
                        </a>
                      ))}
                    </div>
                    <div className="flex items-start gap-2 bg-blue-400/6 border border-blue-400/15 rounded-xl px-3 py-2">
                      <ChevronRight size={10} className="text-blue-400 flex-shrink-0 mt-0.5" />
                      <p className="text-[10px] text-blue-300">
                        <span className="font-semibold">Mainnet preview:</span> On mainnet this executes a real Jupiter swap. Devnet uses accounting-only rebalance with settlement receipt.
                      </p>
                    </div>
                  </>
                )}
              </>
            )}
          </motion.div>
        )}
      </AnimatePresence>

      {/* ── Error state ── */}
      {stage === "error" && error && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          className="rounded-xl border border-red-400/20 bg-red-400/6 p-3 flex items-start gap-2"
        >
          <AlertTriangle size={13} className="text-red-400 flex-shrink-0 mt-0.5" />
          <div>
            <p className="text-xs text-red-300">{error}</p>
            <p className="text-[10px] text-slate-500 mt-1">Make sure backend is running on localhost:8080</p>
          </div>
        </motion.div>
      )}
    </div>
  );
}
