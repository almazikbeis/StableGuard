"use client";

import { startTransition, useCallback, useEffect, useRef, useState } from "react";
import { motion } from "framer-motion";
import Link from "next/link";
import {
  Activity,
  AlertTriangle,
  ArrowRightLeft,
  CheckCircle2,
  ExternalLink,
  Layers3,
  RadioTower,
  Radar,
  RefreshCw,
  ShieldCheck,
  Sparkles,
  TerminalSquare,
  Wallet,
} from "lucide-react";
import { Header } from "@/components/Header";
import { api, type ExecutionJob, type SettingsResponse } from "@/lib/api";
import { toast } from "@/lib/toast";

const STAGE_META: Record<string, { label: string; tone: string; blurb: string }> = {
  custody_staged: {
    label: "Custody Staged",
    tone: "text-cyan-200 border-cyan-300/20 bg-cyan-400/10",
    blurb: "Source funds moved into trusted execution custody and are ready for route construction.",
  },
  swap_submitted: {
    label: "Swap Submitted",
    tone: "text-orange-200 border-orange-300/20 bg-orange-400/10",
    blurb: "External swap transaction submitted. Confirmation and custody reconciliation still pending.",
  },
  swap_confirmed: {
    label: "Swap Confirmed",
    tone: "text-violet-200 border-violet-300/20 bg-violet-400/10",
    blurb: "On-chain transaction confirmed. StableGuard is validating custody deltas.",
  },
  reconciled_in_custody: {
    label: "Reconciled",
    tone: "text-emerald-200 border-emerald-300/20 bg-emerald-400/10",
    blurb: "Custody balances line up with expected deltas. Job is ready for treasury settlement.",
  },
  settled_back_to_treasury: {
    label: "Settled",
    tone: "text-emerald-100 border-emerald-200/18 bg-emerald-400/12",
    blurb: "Output asset returned to treasury. Lifecycle closed successfully.",
  },
  failed: {
    label: "Failed",
    tone: "text-rose-200 border-rose-300/20 bg-rose-400/10",
    blurb: "Execution failed closed. Operator review required before the next attempt.",
  },
};

const STAGE_ORDER = [
  "custody_staged",
  "swap_submitted",
  "swap_confirmed",
  "reconciled_in_custody",
  "settled_back_to_treasury",
];

function numberFmt(value: number) {
  return new Intl.NumberFormat("en-US").format(value);
}

function shortSig(value: string) {
  if (!value) return "—";
  if (value.length <= 14) return value;
  return `${value.slice(0, 6)}…${value.slice(-6)}`;
}

function formatTs(ts?: number) {
  if (!ts) return "—";
  return new Date(ts * 1000).toLocaleString();
}

function stageMeta(stage: string) {
  return STAGE_META[stage] ?? {
    label: stage.replaceAll("_", " "),
    tone: "text-slate-200 border-white/10 bg-white/5",
    blurb: "Execution stage metadata unavailable.",
  };
}

function explorerUrl(sig?: string) {
  if (!sig) return "";
  return `https://explorer.solana.com/tx/${sig}?cluster=devnet`;
}

export default function ExecutionPage() {
  const [jobs, setJobs] = useState<ExecutionJob[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [detail, setDetail] = useState<ExecutionJob | null>(null);
  const [settings, setSettings] = useState<SettingsResponse | null>(null);
  const [manualTx, setManualTx] = useState("");
  const [slippageBps, setSlippageBps] = useState(50);
  const [lastUpdate, setLastUpdate] = useState("");
  const [loading, setLoading] = useState(false);
  const [acting, setActing] = useState<"build" | "execute" | "submit" | "settle" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const previousStageRef = useRef<string | null>(null);

  const loadExecution = useCallback(async (forceSelectedId?: number | null) => {
    setLoading(true);
    const [jobsResult, settingsResult] = await Promise.allSettled([
      api.executionJobs(24),
      api.settings(),
    ]);

    startTransition(() => {
      if (jobsResult.status === "fulfilled") {
        const rows = jobsResult.value.execution_jobs ?? [];
        setJobs(rows);
        const activeCandidate = rows.find((job) => job.stage !== "settled_back_to_treasury" && job.stage !== "failed")?.id ?? null;
        const nextId = forceSelectedId ?? selectedId ?? activeCandidate ?? rows[0]?.id ?? null;
        setSelectedId(nextId);
        if (nextId) {
          void api.executionJob(nextId)
            .then((job) => {
              setDetail(job);
              setManualTx(job.swap_transaction ?? "");
            })
            .catch((detailErr) => setError(String(detailErr)));
        } else {
          setDetail(null);
        }
      } else {
        setError(String(jobsResult.reason));
      }

      if (settingsResult.status === "fulfilled") {
        setSettings(settingsResult.value);
      }

      setLastUpdate(new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" }));
      setLoading(false);
    });
  }, [selectedId]);

  useEffect(() => {
    void loadExecution();
  }, [loadExecution]);

  useEffect(() => {
    const active = jobs.some((job) => job.stage !== "settled_back_to_treasury" && job.stage !== "failed");
    const intervalMs = active ? 5000 : 15000;
    const timer = setInterval(() => {
      void loadExecution(selectedId);
    }, intervalMs);
    return () => clearInterval(timer);
  }, [jobs, selectedId, loadExecution]);

  useEffect(() => {
    if (!detail) return;
    const previous = previousStageRef.current;
    if (previous && previous !== detail.stage) {
      const meta = stageMeta(detail.stage);
      if (detail.stage === "failed") {
        toast.show("danger", `Job #${detail.id} failed`, meta.blurb);
      } else {
        toast.show("success", `Job #${detail.id} → ${meta.label}`, detail.note || meta.blurb);
      }
    }
    previousStageRef.current = detail.stage;
  }, [detail]);

  async function refreshDetail(id: number) {
    const job = await api.executionJob(id);
    setDetail(job);
    if (job.swap_transaction) {
      setManualTx(job.swap_transaction);
    }
    return job;
  }

  async function handleBuild() {
    if (!detail) return;
    setActing("build");
    try {
      const job = await api.buildExecutionSwap(detail.id, { slippage_bps: slippageBps });
      setDetail(job);
      setManualTx(job.swap_transaction ?? "");
      toast.show("success", "Swap route built", job.message ?? "Jupiter transaction prepared.");
      await loadExecution(detail.id);
    } catch (err) {
      toast.show("danger", "Build failed", String(err));
    } finally {
      setActing(null);
    }
  }

  async function handleExecute() {
    if (!detail) return;
    setActing("execute");
    try {
      const job = await api.executeExecutionSwap(detail.id, { slippage_bps: slippageBps });
      setDetail(job);
      toast.show("success", "Swap executed", job.message ?? "Swap executed and reconciled.");
      await loadExecution(detail.id);
    } catch (err) {
      toast.show("danger", "Execution failed", String(err));
    } finally {
      setActing(null);
    }
  }

  async function handleSubmitManual() {
    if (!detail || !manualTx.trim()) return;
    setActing("submit");
    try {
      const job = await api.submitExecutionSwap(detail.id, manualTx.trim());
      setDetail(job);
      toast.show("success", "Manual relay accepted", job.message ?? "External swap submitted.");
      await loadExecution(detail.id);
    } catch (err) {
      toast.show("danger", "Manual submit failed", String(err));
    } finally {
      setActing(null);
    }
  }

  async function handleSettle() {
    if (!detail) return;
    setActing("settle");
    try {
      const job = await api.settleExecutionJob(detail.id, 0);
      setDetail(job);
      toast.show("success", "Treasury settled", job.message ?? "Output asset deposited back into treasury.");
      await loadExecution(detail.id);
    } catch (err) {
      toast.show("danger", "Settlement failed", String(err));
    } finally {
      setActing(null);
    }
  }

  const activeStage = detail ? stageMeta(detail.stage) : null;
  const totalSettled = jobs.reduce((sum, job) => sum + (job.settled_amount ?? 0), 0);
  const activeJobs = jobs.filter((job) => STAGE_ORDER.includes(job.stage) && job.stage !== "settled_back_to_treasury").length;
  const incidentMode = Boolean(
    detail && (
      detail.stage === "failed" ||
      (detail.stage === "swap_confirmed" && (detail.target_delta ?? 0) === 0) ||
      (detail.stage === "reconciled_in_custody" && (detail.source_delta ?? 0) === 0)
    ),
  );
  const stageFeed = jobs
    .filter((job) => job.stage !== "settled_back_to_treasury")
    .slice(0, 8);

  return (
    <div className="app-shell min-h-screen relative overflow-x-hidden">
      <Header
        lastUpdate={lastUpdate}
        onRefresh={() => void loadExecution(selectedId)}
        refreshing={loading}
        connected={!error}
        streamMode="polling"
      />

      <main className="relative max-w-7xl mx-auto px-4 sm:px-6 py-6 space-y-6">
        <motion.section
          initial={{ opacity: 0, y: 18 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.35 }}
          className={`hero-stage panel-surface neon-border rounded-[30px] px-5 py-6 sm:px-7 ${incidentMode ? "shadow-[0_0_0_1px_rgba(251,113,133,0.14),0_0_90px_rgba(244,63,94,0.10)]" : ""}`}
        >
          <div className="grid gap-6 xl:grid-cols-[1.28fr_0.92fr] xl:items-start">
            <div>
              <div className="status-ribbon">
                <span className="data-chip rounded-full px-3 py-1 text-[10px] uppercase tracking-[0.22em] text-cyan-200">Execution Monitor</span>
                <span className="data-chip rounded-full px-3 py-1 text-[10px] uppercase tracking-[0.22em] text-orange-200">Reconciliation Truth</span>
                <span className="status-node rounded-full px-3 py-1 text-[10px] uppercase tracking-[0.22em] text-violet-100">
                  Approval {settings?.execution_approval_mode ?? "manual"}
                </span>
              </div>
              <h1 className="mt-4 max-w-3xl font-display text-3xl leading-[0.98] tracking-[-0.04em] text-white sm:text-4xl lg:text-5xl">
                Watch every treasury swap move from staged custody to reconciled settlement.
              </h1>
              <p className="mt-4 max-w-2xl text-sm leading-relaxed text-slate-300 sm:text-base">
                This is the screen that proves StableGuard is not just generating signals. It shows build, submit, confirm, reconcile, and settle as one bounded operator workflow.
              </p>
              <div className="mt-5 flex flex-wrap items-center gap-3">
                <Link href="/dashboard" className="status-node rounded-full px-4 py-2 text-xs font-semibold uppercase tracking-[0.18em] text-white transition-transform hover:-translate-y-0.5">
                  Back to Dashboard
                </Link>
                <Link href="/settings" className="text-xs uppercase tracking-[0.18em] text-slate-400 hover:text-slate-100">
                  Open operator settings
                </Link>
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 xl:grid-cols-2">
              {[
                { label: "Tracked jobs", value: `${jobs.length}` },
                { label: "Active jobs", value: `${activeJobs}` },
                { label: "Settled units", value: numberFmt(totalSettled) },
                { label: "Execution lane", value: settings?.execution_auto_enabled ? "autonomous" : settings?.execution_mode ?? "record_only" },
              ].map((item) => (
                <div key={item.label} className="data-chip rounded-[18px] px-4 py-3">
                  <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">{item.label}</div>
                  <div className="mt-2 text-sm font-semibold text-white">{item.value}</div>
                </div>
              ))}
            </div>
          </div>
        </motion.section>

        <section className="panel-surface-soft rounded-[24px] p-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-slate-500">
              <RadioTower size={13} className="text-cyan-300" />
              Live Execution Tape
            </div>
            <div className="text-[11px] uppercase tracking-[0.18em] text-slate-500">
              {activeJobs > 0 ? "high-frequency refresh" : "standby cadence"}
            </div>
          </div>
          <div className="mt-4 flex gap-3 overflow-x-auto pb-1">
            {stageFeed.length === 0 ? (
              <div className="rounded-[18px] border border-white/8 bg-white/[0.03] px-4 py-3 text-sm text-slate-400">
                No active execution jobs.
              </div>
            ) : (
              stageFeed.map((job) => {
                const meta = stageMeta(job.stage);
                return (
                  <button
                    key={job.id}
                    onClick={() => {
                      setSelectedId(job.id);
                      void refreshDetail(job.id);
                    }}
                    className={`min-w-[220px] rounded-[18px] border px-4 py-3 text-left transition-all ${
                      selectedId === job.id
                        ? "border-cyan-300/24 bg-cyan-400/10 shadow-[0_0_0_1px_rgba(79,227,255,0.08)]"
                        : "border-white/8 bg-white/[0.03] hover:bg-white/[0.05]"
                    }`}
                  >
                    <div className="flex items-center justify-between gap-3">
                      <div className="font-display text-sm tracking-[0.06em] text-white">
                        {job.source_symbol} → {job.target_symbol}
                      </div>
                      <span className={`rounded-full border px-2 py-0.5 text-[10px] uppercase tracking-[0.18em] ${meta.tone}`}>
                        {meta.label}
                      </span>
                    </div>
                    <div className="mt-3 text-[11px] uppercase tracking-[0.18em] text-slate-500">
                      Job #{job.id} · {formatTs(job.updated_ts)}
                    </div>
                  </button>
                );
              })
            )}
          </div>
        </section>

        {error ? (
          <div className="rounded-[22px] border border-rose-300/18 bg-rose-400/8 px-4 py-4 text-sm text-rose-100/85">
            {error}
          </div>
        ) : null}

        <div className="grid gap-6 xl:grid-cols-[360px_1fr]">
          <section className="panel-surface neon-border rounded-[26px] p-4 sm:p-5">
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.22em] text-slate-500">
                <Layers3 size={13} className="text-cyan-300" />
                Recent Jobs
              </div>
              <button
                onClick={() => void loadExecution(selectedId)}
                className="rounded-full border border-white/10 bg-white/5 p-2 text-slate-300 transition-colors hover:bg-white/8 hover:text-white"
                title="Refresh jobs"
              >
                <RefreshCw size={14} className={loading ? "animate-spin" : ""} />
              </button>
            </div>

            <div className="mt-4 space-y-3">
              {jobs.length === 0 ? (
                <div className="rounded-[20px] border border-white/8 bg-white/[0.03] px-4 py-5 text-sm text-slate-400">
                  No execution jobs yet. Stage a market rebalance from the dashboard or API first.
                </div>
              ) : (
                jobs.map((job) => {
                  const meta = stageMeta(job.stage);
                  const active = selectedId === job.id;
                  return (
                    <button
                      key={job.id}
                      onClick={() => {
                        setSelectedId(job.id);
                        void refreshDetail(job.id);
                      }}
                      className={`w-full rounded-[22px] border px-4 py-4 text-left transition-all ${
                        active
                          ? "border-cyan-300/24 bg-cyan-400/10 shadow-[0_0_0_1px_rgba(79,227,255,0.08)]"
                          : "border-white/8 bg-white/[0.03] hover:bg-white/[0.05]"
                      }`}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <div className="font-display text-lg tracking-[0.06em] text-white">
                            {job.source_symbol} <ArrowRightLeft className="inline mx-1 h-4 w-4 text-slate-500" /> {job.target_symbol}
                          </div>
                          <div className="mt-2 text-xs uppercase tracking-[0.18em] text-slate-500">
                            Job #{job.id} · {formatTs(job.updated_ts)}
                          </div>
                        </div>
                        <span className={`rounded-full border px-2.5 py-1 text-[10px] uppercase tracking-[0.18em] ${meta.tone}`}>
                          {meta.label}
                        </span>
                      </div>
                      <div className="mt-4 grid grid-cols-2 gap-3 text-xs">
                        <div>
                          <div className="text-slate-500">Amount</div>
                          <div className="mt-1 font-mono-data text-slate-100">{numberFmt(job.amount)}</div>
                        </div>
                        <div>
                          <div className="text-slate-500">Settled</div>
                          <div className="mt-1 font-mono-data text-slate-100">{numberFmt(job.settled_amount ?? 0)}</div>
                        </div>
                      </div>
                    </button>
                  );
                })
              )}
            </div>
          </section>

          <section className="space-y-6">
            {detail ? (
              <>
                {incidentMode ? (
                  <motion.section
                    initial={{ opacity: 0, y: 8 }}
                    animate={{ opacity: 1, y: 0 }}
                    className="rounded-[24px] border border-rose-300/18 bg-rose-400/10 px-5 py-4 shadow-[0_0_60px_rgba(244,63,94,0.08)]"
                  >
                    <div className="flex items-start gap-3">
                      <div className="mt-0.5 rounded-2xl border border-rose-300/18 bg-rose-400/12 p-2 text-rose-200">
                        <AlertTriangle size={16} />
                      </div>
                      <div>
                        <div className="text-[11px] uppercase tracking-[0.18em] text-rose-200/80">Incident Mode</div>
                        <div className="mt-1 text-lg font-semibold text-rose-100">Fail-closed review required before the next move.</div>
                        <p className="mt-2 max-w-3xl text-sm leading-relaxed text-rose-100/85">
                          StableGuard detected a lifecycle state that should not be glossed over in a demo or in production. Review the custody deltas, note, and transaction trail before attempting another autonomous action.
                        </p>
                      </div>
                    </div>
                  </motion.section>
                ) : null}

                <section className="panel-surface neon-border rounded-[26px] p-5 sm:p-6">
                  <div className="flex flex-wrap items-start justify-between gap-4">
                    <div>
                      <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.22em] text-slate-500">
                        <TerminalSquare size={13} className="text-orange-300" />
                        Lifecycle Focus
                      </div>
                      <h2 className="mt-3 font-display text-2xl tracking-[0.06em] text-white">
                        Job #{detail.id} · {detail.source_symbol} to {detail.target_symbol}
                      </h2>
                      <p className="mt-2 max-w-2xl text-sm leading-relaxed text-slate-300">{activeStage?.blurb}</p>
                    </div>
                    <span className={`rounded-full border px-3 py-1.5 text-[11px] uppercase tracking-[0.18em] ${activeStage?.tone}`}>
                      {activeStage?.label}
                    </span>
                  </div>

                  <div className="mt-6 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                    {[
                      { label: "Staged amount", value: numberFmt(detail.amount) },
                      { label: "Quote out", value: detail.quote_out_amount || "—" },
                      { label: "Min out", value: detail.min_out_amount || "—" },
                      { label: "Simulation CU", value: detail.simulation_units ? numberFmt(detail.simulation_units) : "—" },
                    ].map((item) => (
                      <div key={item.label} className="data-chip rounded-[18px] px-4 py-3">
                        <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">{item.label}</div>
                        <div className="mt-2 text-sm font-semibold text-white">{item.value}</div>
                      </div>
                    ))}
                  </div>

                  <div className="mt-6 grid gap-3 xl:grid-cols-[1.05fr_0.95fr]">
                    <div className="rounded-[22px] border border-white/8 bg-white/[0.03] p-4">
                      <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-slate-500">
                        <Radar size={13} className="text-cyan-300" />
                        Stage Timeline
                      </div>
                      <div className="mt-4 space-y-3">
                        {STAGE_ORDER.map((stage, index) => {
                          const meta = stageMeta(stage);
                          const currentIndex = STAGE_ORDER.indexOf(detail.stage);
                          const reached = currentIndex >= index || detail.stage === stage;
                          return (
                            <div key={stage} className="flex items-start gap-3">
                              <div className={`mt-1 flex h-7 w-7 items-center justify-center rounded-full border ${reached ? "border-emerald-300/25 bg-emerald-400/12 text-emerald-200" : "border-white/10 bg-white/5 text-slate-500"}`}>
                                {reached ? <CheckCircle2 size={14} /> : <span className="text-[10px]">{index + 1}</span>}
                              </div>
                              <div className="min-w-0">
                                <div className="text-sm font-semibold text-white">{meta.label}</div>
                                <div className="mt-1 text-xs leading-relaxed text-slate-400">{meta.blurb}</div>
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    </div>

                    <div className="rounded-[22px] border border-white/8 bg-white/[0.03] p-4">
                      <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-slate-500">
                        <Wallet size={13} className="text-emerald-300" />
                        Custody Reconciliation
                      </div>
                      <div className="mt-4 grid grid-cols-2 gap-3 text-sm">
                        <div className="rounded-[18px] border border-white/8 bg-black/15 px-4 py-3">
                          <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Source before</div>
                          <div className="mt-2 font-mono-data text-white">{numberFmt(detail.source_balance_before ?? 0)}</div>
                        </div>
                        <div className="rounded-[18px] border border-white/8 bg-black/15 px-4 py-3">
                          <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Source after</div>
                          <div className="mt-2 font-mono-data text-white">{numberFmt(detail.source_balance_after ?? 0)}</div>
                        </div>
                        <div className="rounded-[18px] border border-white/8 bg-black/15 px-4 py-3">
                          <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Target before</div>
                          <div className="mt-2 font-mono-data text-white">{numberFmt(detail.target_balance_before ?? 0)}</div>
                        </div>
                        <div className="rounded-[18px] border border-white/8 bg-black/15 px-4 py-3">
                          <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Target after</div>
                          <div className="mt-2 font-mono-data text-white">{numberFmt(detail.target_balance_after ?? 0)}</div>
                        </div>
                      </div>
                      <div className="mt-3 grid grid-cols-2 gap-3 text-sm">
                        <div className="rounded-[18px] border border-emerald-300/16 bg-emerald-400/8 px-4 py-3">
                          <div className="text-[10px] uppercase tracking-[0.18em] text-emerald-200/80">Source delta</div>
                          <div className="mt-2 font-mono-data text-emerald-100">{numberFmt(detail.source_delta ?? 0)}</div>
                        </div>
                        <div className="rounded-[18px] border border-cyan-300/16 bg-cyan-400/8 px-4 py-3">
                          <div className="text-[10px] uppercase tracking-[0.18em] text-cyan-200/80">Target delta</div>
                          <div className="mt-2 font-mono-data text-cyan-100">{numberFmt(detail.target_delta ?? 0)}</div>
                        </div>
                      </div>
                      <div className="mt-3 grid grid-cols-2 gap-3 text-sm">
                        <div className="rounded-[18px] border border-white/8 bg-black/15 px-4 py-3">
                          <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Reconciled amount</div>
                          <div className="mt-2 font-mono-data text-white">{numberFmt(detail.reconciled_amount ?? 0)}</div>
                        </div>
                        <div className="rounded-[18px] border border-white/8 bg-black/15 px-4 py-3">
                          <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Available to settle</div>
                          <div className="mt-2 font-mono-data text-white">{numberFmt(detail.available_amount_before_settlement ?? 0)}</div>
                        </div>
                      </div>
                    </div>
                  </div>

                  <div className="mt-6 rounded-[22px] border border-white/8 bg-white/[0.03] p-4">
                    <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-slate-500">
                      <ShieldCheck size={13} className="text-violet-300" />
                      Operator Actions
                    </div>
                    <div className="mt-4 flex flex-wrap gap-3">
                      <button
                        onClick={() => void handleBuild()}
                        disabled={acting !== null || detail.stage !== "custody_staged"}
                        className="status-node rounded-full px-4 py-2 text-xs font-semibold uppercase tracking-[0.18em] text-white disabled:opacity-40"
                      >
                        {acting === "build" ? "Building…" : "Build route"}
                      </button>
                      <button
                        onClick={() => void handleExecute()}
                        disabled={acting !== null || detail.stage !== "custody_staged"}
                        className="status-node rounded-full px-4 py-2 text-xs font-semibold uppercase tracking-[0.18em] text-cyan-100 disabled:opacity-40"
                      >
                        {acting === "execute" ? "Executing…" : "Auto execute"}
                      </button>
                      <button
                        onClick={() => void handleSettle()}
                        disabled={acting !== null || !detail.can_settle}
                        className="status-node rounded-full px-4 py-2 text-xs font-semibold uppercase tracking-[0.18em] text-emerald-100 disabled:opacity-40"
                      >
                        {acting === "settle" ? "Settling…" : "Settle to treasury"}
                      </button>
                    </div>

                    <div className="mt-5 grid gap-3 xl:grid-cols-[0.95fr_1.05fr]">
                      <div className="rounded-[18px] border border-white/8 bg-black/15 px-4 py-4">
                        <div className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Route parameters</div>
                        <div className="mt-4 flex flex-wrap items-center gap-3">
                          <label className="text-xs uppercase tracking-[0.18em] text-slate-400">Slippage bps</label>
                          <input
                            value={slippageBps}
                            onChange={(e) => setSlippageBps(Number(e.target.value || 0))}
                            type="number"
                            min={0}
                            className="w-28 rounded-xl border border-white/10 bg-white/5 px-3 py-2 text-sm text-white outline-none"
                          />
                        </div>
                        <div className="mt-4 space-y-2 text-xs text-slate-400">
                          <div>Custody: <span className="font-mono-data text-slate-200">{shortSig(detail.custody_account)}</span></div>
                          <div>Target: <span className="font-mono-data text-slate-200">{shortSig(detail.target_custody_account)}</span></div>
                          <div>Price impact: <span className="font-mono-data text-slate-200">{detail.price_impact_pct || "—"}</span></div>
                          <div>Route hops: <span className="font-mono-data text-slate-200">{detail.route_hops ?? "—"}</span></div>
                        </div>
                      </div>

                      <div className="rounded-[18px] border border-white/8 bg-black/15 px-4 py-4">
                        <div className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Manual relay</div>
                        <textarea
                          value={manualTx}
                          onChange={(e) => setManualTx(e.target.value)}
                          placeholder="Paste a base64 versioned swap transaction"
                          className="mt-3 min-h-[120px] w-full rounded-[18px] border border-white/10 bg-white/5 px-4 py-3 text-xs text-white outline-none"
                        />
                        <button
                          onClick={() => void handleSubmitManual()}
                          disabled={acting !== null || !manualTx.trim() || !detail.can_submit_swap}
                          className="mt-3 status-node rounded-full px-4 py-2 text-xs font-semibold uppercase tracking-[0.18em] text-orange-100 disabled:opacity-40"
                        >
                          {acting === "submit" ? "Submitting…" : "Submit external swap"}
                        </button>
                      </div>
                    </div>
                  </div>
                </section>

                <section className="grid gap-6 xl:grid-cols-[0.95fr_1.05fr]">
                  <div className="panel-surface-soft rounded-[24px] p-5">
                    <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-slate-500">
                      <Activity size={13} className="text-cyan-300" />
                      Signatures
                    </div>
                    <div className="mt-4 space-y-3">
                      {[
                        { label: "Funding", value: detail.funding_sig },
                        { label: "Swap", value: detail.swap_sig },
                        { label: "Settlement", value: detail.settlement_sig },
                      ].map((item) => (
                        <div key={item.label} className="rounded-[18px] border border-white/8 bg-white/[0.03] px-4 py-3">
                          <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">{item.label}</div>
                          {item.value ? (
                            <a
                              href={explorerUrl(item.value)}
                              target="_blank"
                              rel="noreferrer"
                              className="mt-2 inline-flex items-center gap-2 font-mono-data text-sm text-cyan-200 hover:text-cyan-100"
                            >
                              {shortSig(item.value)} <ExternalLink size={12} />
                            </a>
                          ) : (
                            <div className="mt-2 text-sm text-slate-400">No signature yet.</div>
                          )}
                        </div>
                      ))}
                    </div>
                  </div>

                  <div className="panel-surface-soft rounded-[24px] p-5">
                    <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-slate-500">
                      <Sparkles size={13} className="text-orange-300" />
                      Operator Narrative
                    </div>
                    <div className="mt-4 rounded-[20px] border border-white/8 bg-white/[0.03] px-4 py-4 text-sm leading-relaxed text-slate-300">
                      {detail.note || "No operator note available for this job."}
                    </div>
                    <div className="mt-4 grid gap-3 sm:grid-cols-2">
                      <div className="rounded-[18px] border border-white/8 bg-black/15 px-4 py-3">
                        <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Created</div>
                        <div className="mt-2 text-sm text-white">{formatTs(detail.ts)}</div>
                      </div>
                      <div className="rounded-[18px] border border-white/8 bg-black/15 px-4 py-3">
                        <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Updated</div>
                        <div className="mt-2 text-sm text-white">{formatTs(detail.updated_ts)}</div>
                      </div>
                    </div>
                  </div>
                </section>
              </>
            ) : (
              <div className="panel-surface-soft rounded-[24px] p-6 text-sm text-slate-400">
                Select an execution job to inspect its lifecycle.
              </div>
            )}
          </section>
        </div>
      </main>
    </div>
  );
}
