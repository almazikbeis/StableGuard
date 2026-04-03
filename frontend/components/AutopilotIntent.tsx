"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Zap, Loader2, CheckCircle2, ChevronRight, Shield, BarChart2, TrendingUp } from "lucide-react";
import { api, IntentConfig } from "@/lib/api";
import { toast } from "@/lib/toast";

const EXAMPLES = [
  "Earn max yield while protecting against any depeg above 0.5%",
  "Keep my capital safe, only USDC, sleep at night",
  "Aggressive APY on Kamino, I'm okay with medium risk",
  "Balanced approach, rebalance when risk exceeds 40",
];

const STRATEGY_META: Record<number, { label: string; icon: React.ElementType; color: string; bg: string }> = {
  0: { label: "GUARDED",  icon: Shield,    color: "text-green-700",  bg: "bg-green-50 border-green-200" },
  1: { label: "BALANCED", icon: BarChart2, color: "text-blue-700",   bg: "bg-blue-50 border-blue-200" },
  2: { label: "YIELD MAX",icon: TrendingUp,color: "text-orange-700", bg: "bg-orange-50 border-orange-200" },
};

export function AutopilotIntent() {
  const [intent, setIntent]     = useState("");
  const [loading, setLoading]   = useState(false);
  const [result, setResult]     = useState<IntentConfig | null>(null);
  const [applying, setApplying] = useState(false);
  const [applied, setApplied]   = useState(false);

  async function analyze() {
    if (!intent.trim()) return;
    setLoading(true);
    setResult(null);
    setApplied(false);
    try {
      const cfg = await api.parseIntent(intent);
      setResult(cfg);
    } catch {
      toast.show("danger", "AI offline", "Start the backend to use intent parsing");
      // Demo fallback
      setResult({
        strategy_mode: 1,
        risk_threshold: 10,
        yield_entry_risk: 35,
        yield_exit_risk: 55,
        circuit_breaker_pct: 1.5,
        strategy_name: "Balanced",
        explanation: "Balanced strategy with moderate yield and depeg protection. Rebalances when risk exceeds 40.",
      });
    } finally {
      setLoading(false);
    }
  }

  async function applyConfig() {
    if (!result) return;
    setApplying(true);
    try {
      const applied = await api.applyAutopilot({
        strategy_mode: result.strategy_mode,
        risk_threshold: result.risk_threshold,
        yield_entry_risk: result.yield_entry_risk,
        yield_exit_risk: result.yield_exit_risk,
        circuit_breaker_pct: result.circuit_breaker_pct,
      });
      setApplied(true);
      toast.show("success", `${applied.control_mode} mode active`, result.explanation);
    } catch (e) {
      toast.show("danger", "Failed to apply", String(e));
    } finally {
      setApplying(false);
    }
  }

  const meta = result ? STRATEGY_META[result.strategy_mode] ?? STRATEGY_META[1] : null;

  return (
    <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
      {/* Header */}
      <div className="px-4 pt-4 pb-3 border-b border-gray-100 flex items-center gap-2">
        <div className="w-7 h-7 rounded-lg bg-orange-100 flex items-center justify-center">
          <Zap size={13} className="text-orange-500" />
        </div>
        <div>
          <span className="text-sm font-semibold text-gray-900">Autopilot Intent</span>
          <span className="text-xs text-gray-400 ml-2">Describe your goal — AI sets the vault</span>
        </div>
      </div>

      <div className="p-4 space-y-4">
        {/* Intent input */}
        <div>
          <textarea
            value={intent}
            onChange={e => setIntent(e.target.value)}
            onKeyDown={e => e.key === "Enter" && !e.shiftKey && (e.preventDefault(), analyze())}
            placeholder="Describe what you want the vault to do in plain language…"
            rows={2}
            className="w-full text-sm bg-gray-50 border border-gray-200 rounded-xl px-3 py-2.5 resize-none focus:outline-none focus:ring-2 focus:ring-orange-200 focus:border-orange-300 transition-all text-gray-800 placeholder-gray-400"
          />
          {/* Example pills */}
          <div className="flex flex-wrap gap-1.5 mt-2">
            {EXAMPLES.map(ex => (
              <button
                key={ex}
                onClick={() => { setIntent(ex); setResult(null); setApplied(false); }}
                className="text-[10px] bg-gray-50 border border-gray-200 text-gray-500 hover:border-orange-300 hover:text-orange-600 px-2 py-1 rounded-full transition-colors"
              >
                {ex.length > 42 ? ex.slice(0, 42) + "…" : ex}
              </button>
            ))}
          </div>
        </div>

        <button
          onClick={analyze}
          disabled={!intent.trim() || loading}
          className="w-full flex items-center justify-center gap-2 bg-orange-500 hover:bg-orange-600 disabled:opacity-40 text-white text-sm font-bold py-2.5 rounded-xl transition-all"
        >
          {loading
            ? <><Loader2 size={14} className="animate-spin" /> Analyzing intent…</>
            : <><Zap size={14} /> Analyze &amp; configure</>
          }
        </button>

        {/* Result */}
        <AnimatePresence>
          {result && meta && (
            <motion.div
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -8 }}
              className="space-y-3"
            >
              {/* Strategy badge */}
              <div className={`flex items-center gap-2 rounded-xl border px-3 py-2 ${meta.bg}`}>
                <meta.icon size={14} className={meta.color} />
                <div className="flex-1">
                  <p className={`text-xs font-bold ${meta.color}`}>{meta.label} MODE</p>
                  <p className="text-[11px] text-gray-600 mt-0.5 leading-relaxed">{result.explanation}</p>
                </div>
              </div>

              {/* Config details */}
              <div className="grid grid-cols-2 gap-2">
                {[
                  { label: "Risk threshold",    value: `${result.risk_threshold}` },
                  { label: "Circuit breaker",   value: `${result.circuit_breaker_pct}%` },
                  { label: "Yield entry risk",  value: `${result.yield_entry_risk}` },
                  { label: "Yield exit risk",   value: `${result.yield_exit_risk}` },
                ].map(item => (
                  <div key={item.label} className="bg-gray-50 rounded-lg p-2 border border-gray-100">
                    <p className="text-[10px] text-gray-400">{item.label}</p>
                    <p className="text-sm font-mono font-semibold text-gray-800">{item.value}</p>
                  </div>
                ))}
              </div>

              {/* Apply button */}
              {applied ? (
                <div className="flex items-center justify-center gap-2 bg-green-50 border border-green-200 rounded-xl py-2.5">
                  <CheckCircle2 size={14} className="text-green-500" />
                  <span className="text-sm font-semibold text-green-700">Autopilot active!</span>
                </div>
              ) : (
                <button
                  onClick={applyConfig}
                  disabled={applying}
                  className="w-full flex items-center justify-center gap-2 bg-gray-900 hover:bg-gray-800 disabled:opacity-50 text-white text-sm font-bold py-2.5 rounded-xl transition-all"
                >
                  {applying
                    ? <><Loader2 size={13} className="animate-spin" /> Applying…</>
                    : <><ChevronRight size={14} /> Apply {result.strategy_name} config</>
                  }
                </button>
              )}
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  );
}
