"use client";

import { motion } from "framer-motion";
import { FeedMessage } from "@/lib/useRealtime";
import {
  Activity,
  Zap,
  Shield,
  Brain,
  GitMerge,
  AlertTriangle,
  CheckCircle2,
  Clock,
  Bot,
  Lock,
} from "lucide-react";

interface Props {
  liveData: FeedMessage | null;
  connected: boolean;
}

interface StageProps {
  icon: React.ReactNode;
  label: string;
  sublabel: string;
  value?: string;
  active?: boolean;
  alerting?: boolean;
  isFirst?: boolean;
}

function Stage({ icon, label, sublabel, value, active, alerting, isFirst }: StageProps) {
  const border = alerting
    ? "border-red-300/30 bg-red-400/10"
    : active
    ? "border-emerald-300/30 bg-emerald-400/10"
    : "border-white/10 bg-white/4";

  const dot = alerting
    ? "bg-red-500"
    : active
    ? "bg-green-500"
    : "bg-slate-500";

  return (
    <div className="flex items-center gap-1.5">
      {!isFirst && (
        <motion.div
          animate={{ opacity: active ? [0.3, 1, 0.3] : 0.2 }}
          transition={{ repeat: Infinity, duration: 1.2, ease: "easeInOut" }}
          className={`w-5 h-0.5 flex-shrink-0 ${active ? "bg-emerald-300" : alerting ? "bg-red-400" : "bg-white/10"}`}
        />
      )}
      <div className={`border rounded-xl px-3 py-2 min-w-[90px] transition-all ${border}`}>
        <div className="flex items-center gap-1.5 mb-0.5">
          <span className="text-slate-300">{icon}</span>
          <span className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${dot}`} />
        </div>
        <p className="text-[11px] font-semibold text-slate-100 leading-tight">{label}</p>
        <p className="text-[9px] text-slate-400 leading-tight mt-0.5">{sublabel}</p>
        {value && (
          <p className="text-[10px] font-mono text-slate-300 mt-1 truncate">{value}</p>
        )}
      </div>
    </div>
  );
}

export function PipelineVisualizer({ liveData, connected }: Props) {
  const risk = liveData?.risk;
  const decision = liveData?.decision;
  const hotPath = liveData?.hot_path;
  const tripped = hotPath?.tripped ?? false;
  const riskLevel = risk?.risk_level ?? 0;

  const hotActive = connected && !!hotPath;
  const riskActive = connected && !!risk;
  const aiActive = connected && !!decision;

  const priceStr = hotPath?.last_price
    ? `$${(hotPath.last_price / 1_000_000).toFixed(6)}`
    : undefined;

  const riskStr = riskLevel > 0 ? `${riskLevel.toFixed(0)}/100` : undefined;
  const aiStr = decision?.action ?? undefined;
  const execStatus = liveData?.exec_status ?? "standby";
  const execLabel =
    execStatus === "signal_only"
      ? "Signal only"
      : execStatus === "executed"
      ? "Executed"
      : execStatus === "failed"
      ? "Failed"
      : undefined;

  return (
    <div className="panel-surface rounded-[24px] p-5">
      {/* Header */}
      <div className="flex items-center justify-between mb-5">
        <div>
          <h3 className="font-display font-bold text-slate-50 text-sm flex items-center gap-1.5">
            <GitMerge size={14} className="text-orange-500" />
            Pipeline Architecture
          </h3>
          <p className="text-xs text-slate-400 mt-0.5">Realtime risk intake plus configurable AI autonomy</p>
        </div>
        <div className={`flex items-center gap-1.5 text-xs font-semibold px-2.5 py-1 rounded-full ${connected ? "bg-emerald-400/10 text-emerald-200 border border-emerald-300/20" : "bg-white/5 text-slate-400 border border-white/8"}`}>
          <motion.span
            animate={{ opacity: connected ? [1, 0.3, 1] : 1 }}
            transition={{ repeat: Infinity, duration: 2 }}
            className={`w-1.5 h-1.5 rounded-full ${connected ? "bg-emerald-300" : "bg-slate-500"}`}
          />
          {connected ? "Live" : "Offline"}
        </div>
      </div>

      {/* Hot Path row */}
      <div className="mb-4">
        <div className="flex items-center gap-2 mb-2.5">
          <Zap size={11} className="text-orange-500" />
          <span className="text-[10px] font-bold uppercase tracking-widest text-orange-300">
            Hot Path · ~400ms · Oracle + Guardrails
          </span>
        </div>

        <div className="flex items-center overflow-x-auto pb-1">
          <Stage
            icon={<Activity size={12} />}
            label="Pyth Oracle"
            sublabel="SSE stream"
            active={hotActive}
            isFirst
          />
          <Stage
            icon={<Zap size={12} />}
            label="update_price"
            sublabel="Anchor ix"
            value={priceStr}
            active={hotActive && !tripped}
            alerting={tripped}
          />
          <Stage
            icon={tripped ? <AlertTriangle size={12} /> : <Shield size={12} />}
            label={tripped ? "CB Tripped" : "On-chain CB"}
            sublabel={tripped ? "Vault paused!" : "safety boundary"}
            active={hotActive && !tripped}
            alerting={tripped}
          />
          <Stage
            icon={<Shield size={12} />}
            label="Vault State"
            sublabel="is_paused"
            active={hotActive}
            alerting={tripped}
          />
        </div>
      </div>

      {/* Divider with badge */}
      <div className="flex items-center gap-3 mb-4">
        <div className="flex-1 h-px bg-gray-100" />
        <div className="flex items-center gap-1.5 bg-purple-400/10 border border-purple-300/20 rounded-full px-2.5 py-0.5">
          <Bot size={9} className="text-purple-500" />
          <span className="text-[9px] font-bold uppercase tracking-widest text-purple-200">
            AI Policy Layer
          </span>
        </div>
        <div className="flex-1 h-px bg-gray-100" />
      </div>

      {/* Cold Path row */}
      <div>
        <div className="flex items-center gap-2 mb-2.5">
          <Clock size={11} className="text-blue-500" />
          <span className="text-[10px] font-bold uppercase tracking-widest text-cyan-300">
            Cold Path · 5–30s · Risk, AI, Policy
          </span>
        </div>

        <div className="flex items-center overflow-x-auto pb-1">
          <Stage
            icon={<Activity size={12} />}
            label="Risk Engine"
            sublabel="v2 scorer"
            value={riskStr}
            active={riskActive}
            alerting={riskLevel >= 80}
            isFirst
          />
          <Stage
            icon={<Brain size={12} />}
            label="AI Analysis"
            sublabel="risk + yield + rationale"
            value={aiStr}
            active={aiActive}
          />
          <Stage
            icon={<Bot size={12} />}
            label="Control Mode"
            sublabel="manual to yield max"
            active={aiActive}
          />
          <Stage
            icon={aiActive ? <Lock size={12} /> : <Shield size={12} />}
            label="Execution Gate"
            sublabel="record / protect / standby"
            value={execLabel}
            active={aiActive && (execStatus === "executed" || execStatus === "signal_only")}
            alerting={execStatus === "failed"}
          />
          <Stage
            icon={aiActive ? <CheckCircle2 size={12} /> : <GitMerge size={12} />}
            label="Vault State"
            sublabel="decision + policy trace"
            value={liveData?.exec_sig ? "tx" : undefined}
            active={aiActive}
          />
        </div>
      </div>

      {/* Footer note */}
      <p className="text-[9px] text-slate-400 mt-4 leading-relaxed">
        StableGuard separates market observation, AI policy selection, and vault-side state changes. Current custody keeps execution safety-first: decisions can be recorded even when market swaps are intentionally unavailable.
      </p>
    </div>
  );
}
