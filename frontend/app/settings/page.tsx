"use client";

import { startTransition, useEffect, useState, type ReactNode } from "react";
import { motion } from "framer-motion";
import Link from "next/link";
import {
  ArrowLeft,
  Bell,
  Bot,
  CheckCircle,
  ChevronRight,
  ExternalLink,
  KeyRound,
  Radar,
  Send,
  Shield,
  Sparkles,
  Wallet,
  XCircle,
  Zap,
} from "lucide-react";
import { SettingsResponse } from "@/lib/api";

const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

function authHeaders(): HeadersInit {
  if (typeof window === "undefined") return {};
  const token = window.localStorage.getItem("sg_jwt");
  return token ? { Authorization: `Bearer ${token}` } : {};
}

async function fetchJSON<T>(path: string): Promise<T> {
  const response = await fetch(`${BASE}${path}`, {
    headers: authHeaders(),
    cache: "no-store",
  });
  if (!response.ok) throw new Error(`GET ${path} -> ${response.status}`);
  return response.json();
}

async function post(path: string, body: unknown) {
  const response = await fetch(`${BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...authHeaders() },
    body: JSON.stringify(body),
  });
  return response.json();
}

const STEPS = [
  { id: "wallet", label: "Wallet Authority", icon: Wallet, blurb: "Confirm treasury signer and authority routing." },
  { id: "telegram", label: "Telegram Relay", icon: Bell, blurb: "Wire your real-time operator alert stream." },
  { id: "discord", label: "Discord Relay", icon: Zap, blurb: "Broadcast incidents into your ops war room." },
  { id: "done", label: "Launch Checks", icon: Sparkles, blurb: "Test the stack and review live runtime status." },
] as const;

const stepMotion = {
  hidden: { opacity: 0, y: 18 },
  show: { opacity: 1, y: 0, transition: { duration: 0.35 } },
} as const;

type SaveState = "idle" | "saving" | "ok" | "error";

function StatusBadge({
  ok,
  label,
}: {
  ok: boolean;
  label: string;
}) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[10px] uppercase tracking-[0.18em] ${
        ok
          ? "border-emerald-300/20 bg-emerald-400/10 text-emerald-200"
          : "border-amber-300/20 bg-amber-400/10 text-amber-200"
      }`}
    >
      <span className={`h-1.5 w-1.5 rounded-full ${ok ? "bg-emerald-300" : "bg-amber-300"}`} />
      {label}
    </span>
  );
}

function StepRail({
  step,
  setStep,
}: {
  step: number;
  setStep: (index: number) => void;
}) {
  return (
    <div className="panel-surface neon-border rounded-[26px] p-4 sm:p-5">
      <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.22em] text-slate-500">
        <Radar size={13} className="text-cyan-300" />
        Setup Sequence
      </div>

      <div className="mt-4 space-y-3">
        {STEPS.map((item, index) => {
          const Icon = item.icon;
          const active = index === step;
          const completed = index < step;

          return (
            <button
              key={item.id}
              onClick={() => setStep(index)}
              className={`group flex w-full items-start gap-3 rounded-[20px] border px-3 py-3 text-left transition-all ${
                active
                  ? "border-cyan-300/24 bg-cyan-400/10 shadow-[0_0_0_1px_rgba(79,227,255,0.08)]"
                  : completed
                  ? "border-emerald-300/18 bg-emerald-400/8"
                  : "border-white/8 bg-white/[0.03] hover:bg-white/[0.05]"
              }`}
            >
              <div
                className={`mt-0.5 flex h-10 w-10 items-center justify-center rounded-2xl border ${
                  active
                    ? "border-cyan-300/25 bg-cyan-400/12 text-cyan-200"
                    : completed
                    ? "border-emerald-300/25 bg-emerald-400/12 text-emerald-200"
                    : "border-white/10 bg-white/5 text-slate-300"
                }`}
              >
                {completed ? <CheckCircle size={16} /> : <Icon size={16} />}
              </div>

              <div className="min-w-0 flex-1">
                <div className="flex items-center justify-between gap-2">
                  <p className="font-display text-sm tracking-[0.08em] text-white">{item.label}</p>
                  <ChevronRight
                    size={14}
                    className={`transition-transform ${active ? "text-cyan-200" : "text-slate-500 group-hover:translate-x-0.5"}`}
                  />
                </div>
                <p className="mt-1 text-xs leading-relaxed text-slate-400">{item.blurb}</p>
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}

function RuntimePanel({
  wallet,
  settings,
}: {
  wallet: string | null;
  settings: SettingsResponse | null;
}) {
  return (
    <div className="panel-surface-soft rounded-[26px] p-4 sm:p-5">
      <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.22em] text-slate-500">
        <Bot size={13} className="text-orange-300" />
        Runtime Snapshot
      </div>

      <div className="mt-4 grid grid-cols-2 gap-3">
        <div className="data-chip rounded-[18px] px-3 py-3">
          <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">AI profile</div>
          <div className="mt-2 text-sm font-semibold text-white">{settings?.ai_decision_profile ?? "balanced"}</div>
        </div>
        <div className="data-chip rounded-[18px] px-3 py-3">
          <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Approval mode</div>
          <div className="mt-2 text-sm font-semibold text-white">{settings?.execution_approval_mode ?? "manual"}</div>
        </div>
        <div className="data-chip rounded-[18px] px-3 py-3">
          <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Execution lane</div>
          <div className="mt-2 text-sm font-semibold text-white">{settings?.execution_mode ?? "record_only"}</div>
        </div>
        <div className="data-chip rounded-[18px] px-3 py-3">
          <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Yield status</div>
          <div className="mt-2 text-sm font-semibold text-white">{settings?.yield_live_mode ?? "disabled"}</div>
        </div>
        <div className="data-chip rounded-[18px] px-3 py-3">
          <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Growth mode</div>
          <div className="mt-2 text-sm font-semibold text-white">{settings?.growth_sleeve_mode ?? "disabled"}</div>
        </div>
        <div className="data-chip rounded-[18px] px-3 py-3">
          <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Growth budget</div>
          <div className="mt-2 text-sm font-semibold text-white">{settings?.growth_sleeve_budget_pct ?? 0}%</div>
        </div>
      </div>

      <div className="mt-4 space-y-2">
        <StatusBadge ok={Boolean(wallet)} label={wallet ? "wallet authority online" : "wallet unavailable"} />
        <StatusBadge ok={Boolean(settings?.execution_ready_for_staging)} label="custody staging" />
        <StatusBadge ok={Boolean(settings?.execution_ready_for_auto_swap)} label="auto swap path" />
        <StatusBadge ok={Boolean(settings?.growth_sleeve_ready_for_live)} label="growth sleeve live" />
      </div>

      {settings?.mode_readiness && (
        <div className="mt-4 rounded-[18px] border border-white/8 bg-white/[0.03] p-4">
          <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Mode Readiness</div>
          <div className="mt-3 space-y-3">
            {(["MANUAL", "GUARDED", "BALANCED", "YIELD_MAX"] as const).map((mode) => {
              const item = settings.mode_readiness?.[mode];
              if (!item) return null;
              const riskTone =
                item.risk === "high"
                  ? "text-rose-200 border-rose-300/20 bg-rose-400/8"
                  : item.risk === "medium"
                  ? "text-amber-200 border-amber-300/20 bg-amber-400/8"
                  : "text-emerald-200 border-emerald-300/20 bg-emerald-400/8";

              return (
                <div key={mode} className="rounded-[16px] border border-white/8 bg-[#08101c] px-3 py-3">
                  <div className="flex items-center justify-between gap-3">
                    <div className="flex items-center gap-2">
                      <span className="font-display text-sm tracking-[0.08em] text-white">{mode}</span>
                      <span className={`rounded-full border px-2 py-0.5 text-[10px] uppercase tracking-[0.16em] ${riskTone}`}>
                        {item.risk} risk
                      </span>
                    </div>
                    <StatusBadge ok={item.ready} label={item.ready ? "launch ready" : "blocked"} />
                  </div>
                  <p className="mt-2 text-sm leading-relaxed text-slate-300">{item.summary}</p>
                  {item.blockers.length > 0 && (
                    <div className="mt-2 text-xs leading-relaxed text-amber-200/85">
                      Blockers: {item.blockers.join(" ")}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}

      {settings?.growth_sleeve_note && (
        <div className="mt-4 rounded-[18px] border border-cyan-300/14 bg-cyan-400/8 px-4 py-3 text-sm leading-relaxed text-cyan-100/85">
          {settings.growth_sleeve_note}
        </div>
      )}

      {wallet ? (
        <div className="mt-4 rounded-[18px] border border-emerald-300/18 bg-emerald-400/8 px-4 py-3">
          <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-emerald-200">
            <CheckCircle size={13} />
            Wallet Authority
          </div>
          <p className="mt-2 break-all font-mono-data text-xs text-emerald-100/90">{wallet}</p>
        </div>
      ) : (
        <div className="mt-4 rounded-[18px] border border-rose-300/18 bg-rose-400/8 px-4 py-3 text-sm text-rose-100/85">
          Could not load backend wallet authority. Check whether the backend is running and authenticated.
        </div>
      )}
    </div>
  );
}

function MessageBanner({
  state,
  message,
}: {
  state: SaveState;
  message: string;
}) {
  if (!message) return null;
  const ok = state === "ok";

  return (
    <div
      className={`flex items-center gap-2 rounded-[18px] border px-3 py-3 text-sm ${
        ok
          ? "border-emerald-300/18 bg-emerald-400/8 text-emerald-100"
          : "border-rose-300/18 bg-rose-400/8 text-rose-100"
      }`}
    >
      {ok ? <CheckCircle size={14} /> : <XCircle size={14} />}
      {message}
    </div>
  );
}

function ActionButton({
  children,
  disabled,
  onClick,
}: {
  children: ReactNode;
  disabled?: boolean;
  onClick?: () => void;
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className="inline-flex items-center justify-center rounded-2xl border border-orange-300/20 bg-orange-500 px-4 py-3 text-sm font-semibold text-white shadow-[0_16px_40px_rgba(255,122,26,0.25)] transition-all hover:bg-orange-400 disabled:opacity-40"
    >
      {children}
    </button>
  );
}

export default function SettingsPage() {
  const [step, setStep] = useState(0);
  const [settings, setSettings] = useState<SettingsResponse | null>(null);
  const [wallet, setWallet] = useState<string | null>(null);

  const [tgToken, setTgToken] = useState("");
  const [tgChatID, setTgChatID] = useState("");
  const [tgStatus, setTgStatus] = useState<SaveState>("idle");
  const [tgMsg, setTgMsg] = useState("");

  const [dcWebhook, setDcWebhook] = useState("");
  const [dcStatus, setDcStatus] = useState<SaveState>("idle");
  const [dcMsg, setDcMsg] = useState("");

  const [testStatus, setTestStatus] = useState<SaveState>("idle");
  const [testMsg, setTestMsg] = useState("");

  useEffect(() => {
    let cancelled = false;

    async function loadRuntime() {
      const [settingsResult, vaultResult] = await Promise.allSettled([
        fetchJSON<SettingsResponse>("/settings"),
        fetchJSON<{ authority: string }>("/vault"),
      ]);

      if (cancelled) return;

      startTransition(() => {
        if (settingsResult.status === "fulfilled") setSettings(settingsResult.value);
        if (vaultResult.status === "fulfilled") setWallet(vaultResult.value.authority);
      });
    }

    void loadRuntime();
    return () => {
      cancelled = true;
    };
  }, []);

  async function saveTelegram() {
    if (!tgToken || !tgChatID) return;
    setTgStatus("saving");
    try {
      const result = await post("/settings/telegram", { bot_token: tgToken, chat_id: tgChatID });
      setTgStatus(result.ok ? "ok" : "error");
      setTgMsg(result.message ?? result.error ?? "");
    } catch {
      setTgStatus("error");
      setTgMsg("Telegram relay request failed");
    }
  }

  async function saveDiscord() {
    if (!dcWebhook) return;
    setDcStatus("saving");
    try {
      const result = await post("/settings/discord", { webhook_url: dcWebhook });
      setDcStatus(result.ok ? "ok" : "error");
      setDcMsg(result.message ?? result.error ?? "");
    } catch {
      setDcStatus("error");
      setDcMsg("Discord relay request failed");
    }
  }

  async function sendTestAlert() {
    setTestStatus("saving");
    try {
      const result = await post("/settings/test-alert", {});
      setTestStatus(result.ok ? "ok" : "error");
      setTestMsg(result.message ?? result.error ?? "");
    } catch {
      setTestStatus("error");
      setTestMsg("Test alert request failed");
    }
  }

  return (
    <div className="min-h-screen relative overflow-hidden">
      <div className="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden>
        <div className="absolute -top-32 left-[8%] h-72 w-72 rounded-full bg-cyan-400/10 blur-[110px]" />
        <div className="absolute top-24 right-[4%] h-[26rem] w-[26rem] rounded-full bg-orange-500/12 blur-[120px]" />
        <div className="absolute inset-x-0 top-0 h-[30rem] opacity-40 animate-grid-drift bg-[linear-gradient(transparent_96%,rgba(255,255,255,0.05)_100%),linear-gradient(90deg,transparent_96%,rgba(255,255,255,0.05)_100%)] bg-[size:52px_52px]" />
      </div>

      <header className="sticky top-0 z-50 border-b border-white/8 bg-[#08111f]/80 backdrop-blur-xl">
        <div className="max-w-7xl mx-auto flex h-[72px] items-center justify-between gap-4 px-4 sm:px-6">
          <div className="flex items-center gap-3">
            <Link
              href="/dashboard"
              className="flex h-11 w-11 items-center justify-center rounded-2xl border border-white/10 bg-white/[0.04] text-slate-300 transition-colors hover:bg-white/[0.08] hover:text-white"
            >
              <ArrowLeft size={16} />
            </Link>
            <div className="flex items-center gap-3">
              <div className="relative flex h-10 w-10 items-center justify-center rounded-2xl bg-[linear-gradient(135deg,#ff7a1a,#ffb347)] shadow-[0_0_24px_rgba(255,122,26,0.35)]">
                <Shield size={15} className="text-white" />
              </div>
              <div>
                <p className="font-display text-sm tracking-[0.12em] text-white uppercase">Control Settings</p>
                <p className="text-xs text-slate-400">Operator relay setup, execution truth, and runtime health</p>
              </div>
            </div>
          </div>

          <div className="hidden md:flex items-center gap-2">
            <StatusBadge ok={Boolean(settings?.pipeline_running)} label="pipeline online" />
            <StatusBadge ok={Boolean(settings?.execution_ready_for_staging)} label="execution staging" />
          </div>
        </div>
      </header>

      <main className="app-shell relative mx-auto max-w-7xl px-4 py-6 sm:px-6">
        <motion.section
          initial={{ opacity: 0, y: 18 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.35 }}
          className="hero-stage panel-surface neon-border rounded-[30px] px-5 py-6 sm:px-7"
        >
          <div className="grid gap-6 xl:grid-cols-[1.3fr_0.9fr] xl:items-start">
            <div>
              <div className="status-ribbon">
                <span className="data-chip rounded-full px-3 py-1 text-[10px] uppercase tracking-[0.22em] text-cyan-200">Web3 Ops</span>
                <span className="data-chip rounded-full px-3 py-1 text-[10px] uppercase tracking-[0.22em] text-orange-200">Relay Setup</span>
                <span className="data-chip rounded-full px-3 py-1 text-[10px] uppercase tracking-[0.22em] text-violet-200">
                  {settings?.ai_agent_model ?? "claude-haiku-4-5"}
                </span>
                <span className="status-node rounded-full px-3 py-1 text-[10px] uppercase tracking-[0.22em] text-emerald-100">
                  {settings?.settings_persisted ? "Runtime Persisted" : "Runtime Volatile"}
                </span>
              </div>
              <h1 className="mt-4 max-w-3xl font-display text-3xl leading-[0.98] tracking-[-0.04em] text-white sm:text-4xl lg:text-5xl">
                Configure the operator layer like a Solana mission console, not a generic settings page.
              </h1>
              <p className="mt-4 max-w-2xl text-sm leading-relaxed text-slate-300 sm:text-base">
                This screen should tell the truth about custody, AI runtime, alert relays, and operator readiness. Wire the channels, verify the authority, then test the stack.
              </p>
              <div className="mt-5 status-ribbon">
                <span className="status-node rounded-full px-3 py-1.5 text-[11px] uppercase tracking-[0.18em] text-slate-100">
                  Updated {settings?.settings_updated_at ? new Date(settings.settings_updated_at).toLocaleString() : "not yet persisted"}
                </span>
                <span className="status-node rounded-full px-3 py-1.5 text-[11px] uppercase tracking-[0.18em] text-violet-100">
                  Growth {settings?.growth_sleeve_mode ?? "disabled"}
                </span>
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 xl:grid-cols-2">
              {[
                { label: "Control mode", value: settings?.control_mode ?? "UNKNOWN" },
                { label: "AI profile", value: settings?.ai_decision_profile ?? "balanced" },
                { label: "Approval", value: settings?.execution_approval_mode ?? "manual" },
                { label: "Yield", value: settings?.yield_live_mode ?? "disabled" },
                { label: "Growth", value: settings?.growth_sleeve_mode ?? "disabled" },
                { label: "Growth budget", value: `${settings?.growth_sleeve_budget_pct ?? 0}%` },
              ].map((item) => (
                <div key={item.label} className="data-chip rounded-[18px] px-4 py-3">
                  <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">{item.label}</div>
                  <div className="mt-2 text-sm font-semibold text-white">{item.value}</div>
                </div>
              ))}
            </div>
          </div>
        </motion.section>

        <div className="mt-6 grid gap-6 lg:grid-cols-[290px_1fr]">
          <div className="space-y-4">
            <StepRail step={step} setStep={setStep} />
            <RuntimePanel wallet={wallet} settings={settings} />
          </div>

          <div className="space-y-4">
            {step === 0 && (
              <motion.section
                variants={stepMotion}
                initial="hidden"
                animate="show"
                className="panel-surface neon-border rounded-[28px] p-5 sm:p-6"
              >
                <div className="flex items-center gap-3">
                  <div className="flex h-12 w-12 items-center justify-center rounded-2xl border border-cyan-300/20 bg-cyan-400/10 text-cyan-200">
                    <Wallet size={18} />
                  </div>
                  <div>
                    <h2 className="font-display text-xl tracking-[0.08em] text-white uppercase">Wallet Authority</h2>
                    <p className="text-sm text-slate-400">Verify who is actually signing treasury actions.</p>
                  </div>
                </div>

                <div className="mt-5 grid gap-4 lg:grid-cols-[1.15fr_0.85fr]">
                  <div className="rounded-[22px] border border-white/8 bg-white/[0.03] p-4">
                    <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-slate-500">
                      <KeyRound size={13} className="text-orange-300" />
                      Current signer
                    </div>
                    {wallet ? (
                      <>
                        <p className="mt-3 break-all rounded-[18px] border border-emerald-300/16 bg-emerald-400/8 px-4 py-4 font-mono-data text-xs text-emerald-100/90">
                          {wallet}
                        </p>
                        <a
                          href={`https://explorer.solana.com/address/${wallet}?cluster=devnet`}
                          target="_blank"
                          rel="noreferrer"
                          className="mt-3 inline-flex items-center gap-1.5 text-xs text-cyan-200 hover:text-cyan-100"
                        >
                          View signer on explorer <ExternalLink size={12} />
                        </a>
                      </>
                    ) : (
                      <div className="mt-3 rounded-[18px] border border-rose-300/18 bg-rose-400/8 px-4 py-4 text-sm text-rose-100/85">
                        Could not load backend authority wallet. Confirm the backend is online and authenticated.
                      </div>
                    )}
                  </div>

                  <div className="rounded-[22px] border border-white/8 bg-white/[0.03] p-4">
                    <div className="text-[11px] uppercase tracking-[0.18em] text-slate-500">What this means</div>
                    <ul className="mt-3 space-y-3 text-sm leading-relaxed text-slate-300">
                      <li>StableGuard executes treasury actions from the backend authority, not from this browser session.</li>
                      <li>Execution custody and policy approval sit on top of that authority model.</li>
                      <li>Before enabling more autonomy, confirm that this signer and your runtime config are the ones you expect.</li>
                    </ul>
                  </div>
                </div>

                <div className="mt-5 flex flex-wrap gap-3">
                  <ActionButton onClick={() => setStep(1)}>Continue to Telegram Relay</ActionButton>
                  <Link
                    href="/dashboard"
                    className="inline-flex items-center justify-center rounded-2xl border border-white/10 bg-white/[0.04] px-4 py-3 text-sm font-medium text-slate-300 transition-colors hover:bg-white/[0.08] hover:text-white"
                  >
                    Back to dashboard
                  </Link>
                </div>
              </motion.section>
            )}

            {step === 1 && (
              <motion.section
                variants={stepMotion}
                initial="hidden"
                animate="show"
                className="panel-surface neon-border rounded-[28px] p-5 sm:p-6"
              >
                <div className="flex items-center gap-3">
                  <div className="flex h-12 w-12 items-center justify-center rounded-2xl border border-cyan-300/20 bg-cyan-400/10 text-cyan-200">
                    <Bell size={18} />
                  </div>
                  <div>
                    <h2 className="font-display text-xl tracking-[0.08em] text-white uppercase">Telegram Relay</h2>
                    <p className="text-sm text-slate-400">Route treasury risk warnings, reserve instability alerts, and operator actions into your primary feed.</p>
                  </div>
                </div>

                <div className="mt-5 grid gap-4 lg:grid-cols-[1fr_0.9fr]">
                  <div className="rounded-[22px] border border-white/8 bg-white/[0.03] p-4">
                    <div className="space-y-4">
                      <div>
                        <label className="mb-2 block text-[11px] uppercase tracking-[0.18em] text-slate-500">Bot token</label>
                        <input
                          type="text"
                          placeholder="1234567890:ABCdefGHIjklMNOpqrsTUVwxyz"
                          value={tgToken}
                          onChange={(event) => setTgToken(event.target.value)}
                          className="w-full rounded-2xl border border-white/10 bg-[#06101d] px-4 py-3 text-sm text-white outline-none transition-colors placeholder:text-slate-500 focus:border-cyan-300/30"
                        />
                      </div>
                      <div>
                        <label className="mb-2 block text-[11px] uppercase tracking-[0.18em] text-slate-500">Chat ID</label>
                        <input
                          type="text"
                          placeholder="123456789"
                          value={tgChatID}
                          onChange={(event) => setTgChatID(event.target.value)}
                          className="w-full rounded-2xl border border-white/10 bg-[#06101d] px-4 py-3 text-sm text-white outline-none transition-colors placeholder:text-slate-500 focus:border-cyan-300/30"
                        />
                      </div>
                    </div>

                    <div className="mt-4">
                      <MessageBanner state={tgStatus} message={tgMsg} />
                    </div>

                    <div className="mt-4 flex flex-wrap gap-3">
                      <ActionButton onClick={saveTelegram} disabled={!tgToken || !tgChatID || tgStatus === "saving"}>
                        {tgStatus === "saving" ? "Saving relay…" : "Save Telegram"}
                      </ActionButton>
                      <button
                        onClick={() => setStep(2)}
                        className="inline-flex items-center justify-center rounded-2xl border border-white/10 bg-white/[0.04] px-4 py-3 text-sm font-medium text-slate-300 transition-colors hover:bg-white/[0.08] hover:text-white"
                      >
                        Skip for now
                      </button>
                    </div>
                  </div>

                  <div className="rounded-[22px] border border-white/8 bg-white/[0.03] p-4">
                    <div className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Relay setup guide</div>
                    <ol className="mt-3 space-y-3 text-sm leading-relaxed text-slate-300">
                      <li>
                        Create a bot with{" "}
                        <a href="https://t.me/BotFather" target="_blank" rel="noreferrer" className="text-cyan-200 hover:text-cyan-100">
                          @BotFather
                        </a>
                        .
                      </li>
                      <li>Start the bot and send a message from the Telegram account that should receive treasury alerts.</li>
                      <li>
                        Retrieve the chat ID from{" "}
                        <a href="https://t.me/userinfobot" target="_blank" rel="noreferrer" className="text-cyan-200 hover:text-cyan-100">
                          @userinfobot
                        </a>
                        .
                      </li>
                    </ol>
                  </div>
                </div>
              </motion.section>
            )}

            {step === 2 && (
              <motion.section
                variants={stepMotion}
                initial="hidden"
                animate="show"
                className="panel-surface neon-border rounded-[28px] p-5 sm:p-6"
              >
                <div className="flex items-center gap-3">
                  <div className="flex h-12 w-12 items-center justify-center rounded-2xl border border-orange-300/20 bg-orange-400/10 text-orange-200">
                    <Zap size={18} />
                  </div>
                  <div>
                    <h2 className="font-display text-xl tracking-[0.08em] text-white uppercase">Discord Relay</h2>
                    <p className="text-sm text-slate-400">Mirror critical events into your ops channel and incident room.</p>
                  </div>
                </div>

                <div className="mt-5 grid gap-4 lg:grid-cols-[1fr_0.9fr]">
                  <div className="rounded-[22px] border border-white/8 bg-white/[0.03] p-4">
                    <label className="mb-2 block text-[11px] uppercase tracking-[0.18em] text-slate-500">Discord webhook URL</label>
                    <input
                      type="text"
                      placeholder="https://discord.com/api/webhooks/..."
                      value={dcWebhook}
                      onChange={(event) => setDcWebhook(event.target.value)}
                      className="w-full rounded-2xl border border-white/10 bg-[#06101d] px-4 py-3 text-sm text-white outline-none transition-colors placeholder:text-slate-500 focus:border-orange-300/30"
                    />

                    <div className="mt-4">
                      <MessageBanner state={dcStatus} message={dcMsg} />
                    </div>

                    <div className="mt-4 flex flex-wrap gap-3">
                      <ActionButton onClick={saveDiscord} disabled={!dcWebhook || dcStatus === "saving"}>
                        {dcStatus === "saving" ? "Saving relay…" : "Save Discord"}
                      </ActionButton>
                      <button
                        onClick={() => setStep(3)}
                        className="inline-flex items-center justify-center rounded-2xl border border-white/10 bg-white/[0.04] px-4 py-3 text-sm font-medium text-slate-300 transition-colors hover:bg-white/[0.08] hover:text-white"
                      >
                        Skip for now
                      </button>
                    </div>
                  </div>

                  <div className="rounded-[22px] border border-white/8 bg-white/[0.03] p-4">
                    <div className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Webhook setup guide</div>
                    <ol className="mt-3 space-y-3 text-sm leading-relaxed text-slate-300">
                      <li>Create a dedicated treasury-alerts channel in your Discord server.</li>
                      <li>Open channel settings, go to Integrations, then create a new webhook.</li>
                      <li>Paste that URL here so StableGuard can push operator alerts and incident notices.</li>
                    </ol>
                  </div>
                </div>
              </motion.section>
            )}

            {step === 3 && (
              <motion.section
                variants={stepMotion}
                initial="hidden"
                animate="show"
                className="panel-surface neon-border rounded-[28px] p-5 sm:p-6"
              >
                <div className="flex items-center gap-3">
                  <div className="flex h-12 w-12 items-center justify-center rounded-2xl border border-emerald-300/20 bg-emerald-400/10 text-emerald-200">
                    <Sparkles size={18} />
                  </div>
                  <div>
                    <h2 className="font-display text-xl tracking-[0.08em] text-white uppercase">Launch Checks</h2>
                    <p className="text-sm text-slate-400">Test the alert path and review the runtime envelope before going back to the dashboard.</p>
                  </div>
                </div>

                <div className="mt-5 grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
                  <div className="space-y-4">
                    <div className="grid gap-3 sm:grid-cols-3">
                      {[
                        { label: "Monitoring", desc: "Pyth price streams and live vault state." },
                        { label: "AI runtime", desc: `Profile ${settings?.ai_decision_profile ?? "balanced"} on ${settings?.ai_agent_model ?? "claude-haiku-4-5"}.` },
                        { label: "Execution", desc: settings?.execution_note ?? "Execution path status unavailable." },
                        { label: "Growth sleeve", desc: settings?.growth_sleeve_note ?? "Growth sleeve status unavailable." },
                      ].map((item) => (
                        <div key={item.label} className="rounded-[20px] border border-white/8 bg-white/[0.03] p-4">
                          <div className="text-[11px] uppercase tracking-[0.18em] text-slate-500">{item.label}</div>
                          <p className="mt-2 text-sm leading-relaxed text-slate-200">{item.desc}</p>
                        </div>
                      ))}
                    </div>

                    <div className="rounded-[22px] border border-white/8 bg-white/[0.03] p-4">
                      <div className="flex items-center justify-between gap-3">
                        <div>
                          <div className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Alert probe</div>
                          <p className="mt-1 text-sm text-slate-300">Send a test notification through the configured relays.</p>
                        </div>
                        <ActionButton onClick={sendTestAlert} disabled={testStatus === "saving"}>
                          <span className="inline-flex items-center gap-2">
                            <Send size={14} />
                            {testStatus === "saving" ? "Sending…" : "Send Test Alert"}
                          </span>
                        </ActionButton>
                      </div>

                      <div className="mt-4">
                        <MessageBanner state={testStatus} message={testMsg} />
                      </div>
                    </div>
                  </div>

                  <div className="rounded-[22px] border border-white/8 bg-white/[0.03] p-4">
                    <div className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Operational thresholds</div>
                    <div className="mt-4 space-y-2">
                      {[
                        { label: "Risk alert", value: `${settings?.alert_risk_threshold ?? 80}` },
                        { label: "Circuit breaker", value: `${settings?.circuit_breaker_pause_pct ?? 1.5}%` },
                        { label: "Yield entry", value: `${settings?.yield_entry_risk ?? 35}` },
                        { label: "Yield exit", value: `${settings?.yield_exit_risk ?? 55}` },
                      ].map((item) => (
                        <div key={item.label} className="flex items-center justify-between rounded-2xl border border-white/8 bg-[#06101d] px-4 py-3">
                          <span className="text-sm text-slate-300">{item.label}</span>
                          <span className="font-mono-data text-sm text-white">{item.value}</span>
                        </div>
                      ))}
                    </div>

                    <Link
                      href="/dashboard"
                      className="mt-5 inline-flex w-full items-center justify-center rounded-2xl border border-cyan-300/18 bg-cyan-400/10 px-4 py-3 text-sm font-semibold text-cyan-100 transition-colors hover:bg-cyan-400/15"
                    >
                      Return to Dashboard
                    </Link>
                  </div>
                </div>
              </motion.section>
            )}
          </div>
        </div>
      </main>
    </div>
  );
}
