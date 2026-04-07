"use client";

import { useEffect, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { useRouter } from "next/navigation";
import { useWallet } from "@solana/wallet-adapter-react";
import { useWalletModal } from "@solana/wallet-adapter-react-ui";
import {
  Shield, Wallet, Send, MessageSquare, BarChart2,
  CheckCircle2, ChevronRight, Loader2, Zap, ArrowRight,
  ExternalLink, AlertTriangle, Bot, Lock, Sparkles,
} from "lucide-react";
import { api, type ControlMode, type TelegramNotificationLink } from "@/lib/api";

const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

/* ── Step definitions ─────────────────────────────────────────────── */
const STEPS = [
  { id: "wallet",   label: "Wallet",   icon: Wallet,        color: "text-orange-400" },
  { id: "alerts",   label: "Alerts",   icon: Send,          color: "text-cyan-400" },
  { id: "strategy", label: "Strategy", icon: BarChart2,      color: "text-violet-400" },
  { id: "done",     label: "Launch",   icon: CheckCircle2,  color: "text-emerald-400" },
];

/* ── Agent modes ──────────────────────────────────────────────────── */
const MODES = [
  {
    key: "MANUAL",
    label: "Manual",
    emoji: "🎛️",
    tone: "border-slate-600 hover:border-slate-400",
    active: "border-slate-400 bg-slate-800/60 ring-1 ring-slate-400/30",
    badge: "bg-slate-700 text-slate-300",
    desc: "AI monitors and explains. You decide every action. Full control.",
  },
  {
    key: "GUARDED",
    label: "Guarded",
    emoji: "🛡️",
    tone: "border-emerald-700 hover:border-emerald-500",
    active: "border-emerald-500 bg-emerald-900/30 ring-1 ring-emerald-400/30",
    badge: "bg-emerald-900/60 text-emerald-300",
    desc: "Capital preservation first. AI steps in only for severe reserve instability or >80 portfolio risk.",
  },
  {
    key: "BALANCED",
    label: "Balanced",
    emoji: "⚖️",
    tone: "border-cyan-700 hover:border-cyan-500",
    active: "border-cyan-500 bg-cyan-900/30 ring-1 ring-cyan-400/30",
    badge: "bg-cyan-900/60 text-cyan-300",
    recommended: true,
    desc: "Moderate automation. Auto-rebalance on risk >40. Yield entry below 35.",
  },
  {
    key: "YIELD_MAX",
    label: "Yield Max",
    emoji: "⚡",
    tone: "border-orange-700 hover:border-orange-500",
    active: "border-orange-500 bg-orange-900/30 ring-1 ring-orange-400/30",
    badge: "bg-orange-900/60 text-orange-300",
    desc: "Maximize APY. Kamino + Drift with aggressive allocation rules. Exit on critical treasury risk.",
  },
];

/* ── Confetti ─────────────────────────────────────────────────────── */
const DOTS = Array.from({ length: 24 }, (_, i) => ({
  id: i,
  x: Math.random() * 100,
  delay: Math.random() * 0.5,
  color: ["#ff7a1a", "#4fe3ff", "#9b8cff", "#34d399", "#fbbf24", "#ff6b9d"][i % 6],
  size: 5 + Math.random() * 7,
}));

/* ── Animations ───────────────────────────────────────────────────── */
const slide = {
  hidden: { opacity: 0, x: 32 },
  show:   { opacity: 1, x: 0,  transition: { type: "spring" as const, stiffness: 120, damping: 22 } },
  exit:   { opacity: 0, x: -24, transition: { duration: 0.16 } },
};

/* ── Page ─────────────────────────────────────────────────────────── */
export default function OnboardingPage() {
  const router  = useRouter();
  const [step, setStep] = useState(0);

  const { connected, publicKey, signMessage } = useWallet();
  const { setVisible } = useWalletModal();

  /* auth */
  const [wallet,        setWallet]        = useState<string | null>(null);
  const [walletLoading, setWalletLoading] = useState(false);
  const [demoLoading,   setDemoLoading]   = useState(false);
  const [authError,     setAuthError]     = useState<string | null>(null);

  /* alerts */
  const [tgHandle,  setTgHandle]  = useState("");
  const [tgPhone,   setTgPhone]   = useState("");
  const [tgSaving,  setTgSaving]  = useState(false);
  const [tgStatus,  setTgStatus]  = useState<"idle"|"ok"|"error">("idle");
  const [tgMsg,     setTgMsg]     = useState<string | null>(null);
  const [tgLink,    setTgLink]    = useState<TelegramNotificationLink | null>(null);

  const [dcUrl,     setDcUrl]     = useState("");
  const [dcSaving,  setDcSaving]  = useState(false);
  const [dcStatus,  setDcStatus]  = useState<"idle"|"ok"|"error">("idle");

  /* strategy */
  const [mode,    setMode]    = useState("BALANCED");
  const [saving,  setSaving]  = useState(false);

  function next() { setStep((s) => Math.min(s + 1, STEPS.length - 1)); }

  /* ── Redirect if already authed ── */
  useEffect(() => {
    const jwt = localStorage.getItem("sg_jwt");
    const user = JSON.parse(localStorage.getItem("sg_user") || "{}");
    if (jwt && user.onboarded) {
      router.replace("/dashboard");
    }
  }, [router]);

  /* ── Auto-advance when wallet connected and auth needed ── */
  useEffect(() => {
    if (connected && publicKey && !wallet && step === 0) {
      setAuthError(null);
    }
  }, [connected, publicKey, wallet, step]);

  /* ── Wallet: Demo Login ── */
  async function demoLogin() {
    setDemoLoading(true);
    setAuthError(null);
    try {
      const res  = await fetch(`${BASE}/demo/token`);
      const data = await res.json();
      if (!res.ok) { setAuthError(data.error || "Demo login failed"); return; }
      localStorage.setItem("sg_jwt", data.token);
      setWallet(data.wallet);
      next();
    } catch (e) {
      setAuthError(e instanceof Error ? e.message : "Demo login failed");
    } finally {
      setDemoLoading(false);
    }
  }

  /* ── Wallet: Real Phantom/Solflare sign ── */
  async function authenticateWallet() {
    if (!connected || !publicKey || !signMessage) return;
    setWalletLoading(true);
    setAuthError(null);
    try {
      const ts  = Math.floor(Date.now() / 1000);
      const msg = `StableGuard login: ${ts}`;
      const sig = await signMessage(new TextEncoder().encode(msg));
      const res = await fetch(`${BASE}/auth/wallet-login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ address: publicKey.toBase58(), message: msg, signature: Buffer.from(sig).toString("base64") }),
      });
      const data = await res.json();
      if (!res.ok) { setAuthError(data.error || "Auth failed"); return; }
      if (data.token) localStorage.setItem("sg_jwt", data.token);
      setWallet(publicKey.toBase58());
      next(); // ← advance automatically after sign
    } catch (e) {
      setAuthError(e instanceof Error ? e.message : "Wallet auth failed");
    } finally {
      setWalletLoading(false);
    }
  }

  /* ── Telegram: save to backend then test ── */
  async function saveTelegram() {
    if (!tgHandle && !tgPhone) return;
    setTgSaving(true);
    setTgStatus("idle");
    setTgMsg(null);
    try {
      const result = await api.registerTelegramNotification({
        telegram_handle: tgHandle || undefined,
        phone: tgPhone || undefined,
      });
      setTgLink(result);
      setTgMsg(result.message);
      setTgStatus("ok");
    } catch (e) {
      setTgMsg(e instanceof Error ? e.message : "Telegram setup failed");
      setTgStatus("error");
    } finally {
      setTgSaving(false);
    }
  }

  async function sendTelegramTest() {
    setTgSaving(true);
    setTgMsg(null);
    try {
      const result = await api.sendUserNotificationTest();
      setTgMsg(result.message);
      setTgStatus("ok");
    } catch (e) {
      setTgMsg(e instanceof Error ? e.message : "Telegram test failed");
      setTgStatus("error");
    } finally {
      setTgSaving(false);
    }
  }

  /* ── Discord: save to backend then test ── */
  async function saveDiscord() {
    if (!dcUrl) return;
    setDcSaving(true);
    setDcStatus("idle");
    try {
      const jwt = localStorage.getItem("sg_jwt") ?? "";
      const r1  = await fetch(`${BASE}/settings/discord`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${jwt}` },
        body: JSON.stringify({ webhook_url: dcUrl }),
      });
      if (!r1.ok) { setDcStatus("error"); return; }
      const r2 = await fetch(`${BASE}/settings/test-alert`, {
        method: "POST",
        headers: { Authorization: `Bearer ${jwt}` },
      });
      setDcStatus(r2.ok ? "ok" : "error");
    } catch {
      setDcStatus("error");
    } finally {
      setDcSaving(false);
    }
  }

  /* ── Finish: apply mode + persist ── */
  async function finish() {
    setSaving(true);
    try {
      await api.applyControlMode(mode as ControlMode);
    } catch {
      setSaving(false);
      return;
    }
    localStorage.setItem("sg_user", JSON.stringify({
      wallet, strategy: mode, tgOk: tgStatus === "ok",
      dcOk: dcStatus === "ok", onboarded: true,
    }));
    setSaving(false);
    next();
  }

  /* ─── Shared input style ─── */
  const inputCls = "w-full bg-white/[0.04] border border-white/10 rounded-[14px] px-3.5 py-2.5 text-sm font-mono text-slate-100 placeholder-slate-600 focus:outline-none focus:border-cyan-500/50 focus:ring-1 focus:ring-cyan-500/20 transition-all";

  /* ─── Card wrapper ─── */
  const Card = ({ children }: { children: React.ReactNode }) => (
    <div className="w-full max-w-[460px] rounded-[28px] border border-white/10 bg-[rgba(9,20,38,0.88)] backdrop-blur-xl shadow-[0_32px_80px_rgba(0,0,0,0.6),inset_0_1px_0_rgba(255,255,255,0.06)] p-7">
      {children}
    </div>
  );

  /* ─── Status badge ─── */
  const StatusBadge = ({ status, okText = "Connected ✓", errText = "Failed — check credentials" }: { status: "idle"|"ok"|"error"; okText?: string; errText?: string }) => {
    if (status === "idle") return null;
    return (
      <motion.div initial={{ opacity: 0, y: -4 }} animate={{ opacity: 1, y: 0 }}
        className={`flex items-center gap-2 rounded-[12px] px-3 py-2 text-xs mb-3
          ${status === "ok" ? "bg-emerald-500/10 border border-emerald-500/20 text-emerald-300" : "bg-red-500/10 border border-red-500/20 text-red-300"}`}>
        {status === "ok" ? <CheckCircle2 size={12} /> : <AlertTriangle size={12} />}
        {status === "ok" ? okText : errText}
      </motion.div>
    );
  };

  return (
    <div className="min-h-screen flex flex-col items-center justify-center px-4 py-10 relative overflow-hidden">
      {/* Background */}
      <div className="pointer-events-none fixed inset-0" aria-hidden>
        <div className="absolute inset-0 bg-[#07111f]" />
        <div className="absolute inset-0 bg-[linear-gradient(var(--grid)_1px,transparent_1px),linear-gradient(90deg,var(--grid)_1px,transparent_1px)] bg-[size:72px_72px]" />
        <div className="absolute -top-32 left-1/2 -translate-x-1/2 h-[28rem] w-[28rem] rounded-full bg-orange-500/12 blur-[120px]" />
        <div className="absolute bottom-0 right-1/4 h-64 w-64 rounded-full bg-cyan-400/10 blur-[100px]" />
      </div>

      {/* Logo */}
      <motion.div initial={{ opacity: 0, y: -12 }} animate={{ opacity: 1, y: 0 }} className="relative flex items-center gap-2.5 mb-8">
        <div className="w-10 h-10 rounded-[14px] bg-[linear-gradient(135deg,#ff7a1a,#ffb347)] shadow-[0_0_24px_rgba(255,122,26,0.4)] flex items-center justify-center">
          <Shield size={16} className="text-white" />
        </div>
        <div>
          <span className="block font-display font-bold text-white text-sm tracking-[0.14em] uppercase">StableGuard</span>
          <span className="text-[10px] text-slate-500 tracking-[0.1em]">Autonomous Treasury · Solana</span>
        </div>
      </motion.div>

      {/* Progress bar */}
      {step < 3 && (
        <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="relative flex items-center gap-2 mb-8">
          {STEPS.slice(0, 3).map((s, i) => {
            const Icon = s.icon;
            const done = i < step;
            const active = i === step;
            return (
              <div key={s.id} className="flex items-center gap-2">
                <div className={`flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[10px] uppercase tracking-[0.16em] border transition-all duration-300
                  ${done   ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-300"
                  : active ? "border-white/20 bg-white/8 text-white"
                  :          "border-white/6 bg-transparent text-slate-600"}`}>
                  {done ? <CheckCircle2 size={11} /> : <Icon size={11} />}
                  {s.label}
                </div>
                {i < 2 && <div className={`h-px w-6 rounded transition-all duration-500 ${i < step ? "bg-emerald-500/50" : "bg-white/8"}`} />}
              </div>
            );
          })}
        </motion.div>
      )}

      {/* Step cards */}
      <AnimatePresence mode="wait">

        {/* ── Step 0: Wallet ── */}
        {step === 0 && (
          <motion.div key="wallet" variants={slide} initial="hidden" animate="show" exit="exit" className="w-full flex justify-center">
            <Card>
              <div className="w-11 h-11 rounded-[16px] bg-orange-500/15 border border-orange-500/25 flex items-center justify-center mb-5">
                <Wallet size={20} className="text-orange-400" />
              </div>
              <h2 className="font-display font-bold text-xl text-white mb-1">Connect your wallet</h2>
              <p className="text-sm text-slate-400 mb-6 leading-relaxed">
                Link a Solana wallet to sign transactions and authorize vault operations on-chain.
              </p>

              {/* Already authenticated */}
              {wallet ? (
                <div className="flex items-center gap-3 rounded-[14px] border border-emerald-500/25 bg-emerald-500/8 px-4 py-3 mb-5">
                  <CheckCircle2 size={15} className="text-emerald-400 flex-shrink-0" />
                  <div>
                    <p className="text-xs font-semibold text-emerald-300">Authenticated</p>
                    <p className="text-[11px] font-mono text-emerald-400/70 mt-0.5">{wallet.slice(0, 16)}…{wallet.slice(-8)}</p>
                  </div>
                </div>
              ) : connected && publicKey ? (
                /* Wallet connected, needs signature */
                <div className="space-y-3 mb-5">
                  <div className="flex items-center gap-3 rounded-[14px] border border-cyan-500/25 bg-cyan-500/8 px-4 py-3">
                    <Wallet size={14} className="text-cyan-400 flex-shrink-0" />
                    <div className="flex-1 min-w-0">
                      <p className="text-xs font-semibold text-cyan-300">Wallet detected</p>
                      <p className="text-[11px] font-mono text-cyan-400/70 mt-0.5 truncate">{publicKey.toBase58()}</p>
                    </div>
                  </div>
                  {authError && (
                    <p className="text-xs text-red-400 bg-red-500/8 border border-red-500/20 rounded-[12px] px-3 py-2">{authError}</p>
                  )}
                  <button onClick={authenticateWallet} disabled={walletLoading}
                    className="w-full flex items-center justify-center gap-2 bg-orange-500 hover:bg-orange-400 disabled:opacity-50 text-white text-sm font-bold py-3 rounded-[14px] transition-all shadow-[0_0_20px_rgba(255,122,26,0.3)]">
                    {walletLoading ? <Loader2 size={14} className="animate-spin" /> : <Lock size={14} />}
                    {walletLoading ? "Signing message…" : "Sign & Authenticate"}
                  </button>
                </div>
              ) : (
                /* No wallet — show connect options */
                <div className="space-y-2.5 mb-5">
                  {[
                    { name: "Phantom",  emoji: "👻", desc: "Most popular · Chrome/Firefox/Mobile" },
                    { name: "Solflare", emoji: "🔥", desc: "Native Solana · Built-in staking" },
                  ].map((w) => (
                    <button key={w.name} onClick={() => setVisible(true)}
                      className="w-full flex items-center gap-3 rounded-[14px] border border-white/8 bg-white/[0.03] hover:bg-white/[0.06] hover:border-white/16 px-4 py-3 transition-all group">
                      <span className="text-xl">{w.emoji}</span>
                      <div className="text-left flex-1">
                        <p className="text-sm font-semibold text-slate-100">{w.name}</p>
                        <p className="text-[11px] text-slate-500">{w.desc}</p>
                      </div>
                      <ChevronRight size={14} className="text-slate-600 group-hover:text-slate-400 transition-colors" />
                    </button>
                  ))}

                  <div className="relative flex items-center gap-3 py-1">
                    <div className="flex-1 h-px bg-white/6" />
                    <span className="text-[11px] text-slate-600 uppercase tracking-[0.14em]">or</span>
                    <div className="flex-1 h-px bg-white/6" />
                  </div>

                  <button onClick={demoLogin} disabled={demoLoading}
                    className="w-full flex items-center gap-3 rounded-[14px] border border-orange-500/25 bg-orange-500/8 hover:bg-orange-500/14 px-4 py-3 transition-all disabled:opacity-50 group">
                    {demoLoading ? <Loader2 size={18} className="animate-spin text-orange-400" /> : <span className="text-xl">⚡</span>}
                    <div className="text-left flex-1">
                      <p className="text-sm font-semibold text-orange-300">Quick Demo Login</p>
                      <p className="text-[11px] text-orange-400/60">Server wallet — no extension needed</p>
                    </div>
                    <ChevronRight size={14} className="text-orange-500/40 group-hover:text-orange-400 transition-colors" />
                  </button>

                  {authError && (
                    <p className="text-xs text-red-400 bg-red-500/8 border border-red-500/20 rounded-[12px] px-3 py-2">{authError}</p>
                  )}
                </div>
              )}

              <div className="flex gap-2 mt-2">
                <button onClick={next}
                  className="flex-1 text-xs text-slate-600 hover:text-slate-400 py-2.5 transition-colors">
                  Skip
                </button>
                <button onClick={next} disabled={!wallet}
                  className="flex-1 flex items-center justify-center gap-1.5 bg-white/8 hover:bg-white/12 disabled:opacity-30 text-white text-sm font-semibold py-2.5 rounded-[12px] border border-white/10 transition-all">
                  Continue <ArrowRight size={13} />
                </button>
              </div>
            </Card>
          </motion.div>
        )}

        {/* ── Step 1: Alerts ── */}
        {step === 1 && (
          <motion.div key="alerts" variants={slide} initial="hidden" animate="show" exit="exit" className="w-full flex justify-center">
            <Card>
              <div className="w-11 h-11 rounded-[16px] bg-cyan-500/15 border border-cyan-500/25 flex items-center justify-center mb-5">
                <Bot size={20} className="text-cyan-400" />
              </div>
              <h2 className="font-display font-bold text-xl text-white mb-1">AI Alert Channels</h2>
              <p className="text-sm text-slate-400 mb-6 leading-relaxed">
                Connect Telegram once and receive risk alerts, execution reports, and yield updates from the StableGuard bot.
              </p>

              {/* Telegram */}
              <div className="rounded-[16px] border border-white/8 bg-white/[0.025] p-4 mb-3">
                <div className="flex items-center gap-2 mb-3">
                  <Send size={13} className="text-cyan-400" />
                  <span className="text-xs font-semibold text-slate-300 uppercase tracking-[0.14em]">Telegram</span>
                </div>
                <div className="space-y-2 mb-3">
                  <input value={tgHandle} onChange={e => { setTgHandle(e.target.value); setTgStatus("idle"); }}
                    placeholder="@telegram_username" className={inputCls} />
                  <input value={tgPhone} onChange={e => { setTgPhone(e.target.value); setTgStatus("idle"); }}
                    placeholder="+7 701 000 0000 (optional)" className={inputCls} />
                </div>
                <StatusBadge status={tgStatus} okText="✓ Telegram contact saved" errText="Telegram setup failed" />
                {tgMsg && (
                  <p className="mb-3 text-[11px] leading-relaxed text-slate-400">{tgMsg}</p>
                )}
                {tgLink?.deep_link && (
                  <a
                    href={tgLink.deep_link}
                    target="_blank"
                    rel="noreferrer"
                    className="mb-3 flex items-center gap-1.5 rounded-[10px] border border-emerald-500/25 bg-emerald-500/8 px-3 py-2 text-xs font-semibold text-emerald-300 transition-all hover:bg-emerald-500/14"
                  >
                    <ExternalLink size={11} />
                    Start bot and confirm this chat
                  </a>
                )}
                <div className="flex flex-wrap gap-2">
                <button onClick={saveTelegram} disabled={(!tgHandle && !tgPhone) || tgSaving}
                  className="flex items-center gap-1.5 rounded-[10px] border border-cyan-500/25 bg-cyan-500/8 hover:bg-cyan-500/14 disabled:opacity-40 text-cyan-300 text-xs font-semibold px-3 py-2 transition-all">
                  {tgSaving ? <Loader2 size={11} className="animate-spin" /> : <Zap size={11} />}
                  {tgSaving ? "Saving…" : "Save contact"}
                </button>
                <button onClick={sendTelegramTest} disabled={!tgLink?.deep_link || tgSaving}
                  className="flex items-center gap-1.5 rounded-[10px] border border-emerald-500/25 bg-emerald-500/8 hover:bg-emerald-500/14 disabled:opacity-40 text-emerald-300 text-xs font-semibold px-3 py-2 transition-all">
                  {tgSaving ? <Loader2 size={11} className="animate-spin" /> : <CheckCircle2 size={11} />}
                  Send test alert
                </button>
                </div>
                <ul className="mt-3 space-y-1 text-[11px] leading-relaxed text-slate-500">
                  <li>1. Enter your Telegram username or phone number.</li>
                  <li>2. Save contact and open the bot link once.</li>
                  <li>3. After you press Start, StableGuard can send alerts into that chat.</li>
                </ul>
              </div>

              {/* Discord */}
              <div className="rounded-[16px] border border-white/8 bg-white/[0.025] p-4 mb-5">
                <div className="flex items-center gap-2 mb-3">
                  <MessageSquare size={13} className="text-violet-400" />
                  <span className="text-xs font-semibold text-slate-300 uppercase tracking-[0.14em]">Discord</span>
                </div>
                <input value={dcUrl} onChange={e => { setDcUrl(e.target.value); setDcStatus("idle"); }}
                  placeholder="Webhook URL (Channel → Integrations → Webhooks)" className={`${inputCls} mb-3`} />
                <StatusBadge status={dcStatus} okText="✓ Test message sent to Discord!" />
                <button onClick={saveDiscord} disabled={!dcUrl || dcSaving}
                  className="flex items-center gap-1.5 rounded-[10px] border border-violet-500/25 bg-violet-500/8 hover:bg-violet-500/14 disabled:opacity-40 text-violet-300 text-xs font-semibold px-3 py-2 transition-all">
                  {dcSaving ? <Loader2 size={11} className="animate-spin" /> : <Zap size={11} />}
                  {dcSaving ? "Sending…" : "Save & Test"}
                </button>
              </div>

              <div className="flex gap-2">
                <button onClick={next}
                  className="flex-1 text-xs text-slate-600 hover:text-slate-400 py-2.5 transition-colors">
                  Skip
                </button>
                <button onClick={next}
                  className="flex-1 flex items-center justify-center gap-1.5 bg-white/8 hover:bg-white/12 text-white text-sm font-semibold py-2.5 rounded-[12px] border border-white/10 transition-all">
                  {tgStatus === "ok" || dcStatus === "ok" ? "Continue" : "Skip for now"} <ArrowRight size={13} />
                </button>
              </div>
            </Card>
          </motion.div>
        )}

        {/* ── Step 2: Strategy ── */}
        {step === 2 && (
          <motion.div key="strategy" variants={slide} initial="hidden" animate="show" exit="exit" className="w-full flex justify-center">
            <Card>
              <div className="w-11 h-11 rounded-[16px] bg-violet-500/15 border border-violet-500/25 flex items-center justify-center mb-5">
                <Sparkles size={20} className="text-violet-400" />
              </div>
              <h2 className="font-display font-bold text-xl text-white mb-1">Choose AI mode</h2>
              <p className="text-sm text-slate-400 mb-5 leading-relaxed">
                How much authority does the agent get? You can change this any time from the dashboard.
              </p>

              <div className="space-y-2 mb-6">
                {MODES.map((m) => (
                  <button key={m.key} onClick={() => setMode(m.key)}
                    className={`w-full text-left rounded-[16px] border-2 px-4 py-3.5 transition-all ${mode === m.key ? m.active : m.tone + " bg-transparent"}`}>
                    <div className="flex items-center gap-2 mb-1">
                      <span className="text-base">{m.emoji}</span>
                      <span className="text-sm font-bold text-white">{m.label}</span>
                      {m.recommended && (
                        <span className="text-[9px] font-bold uppercase tracking-[0.18em] bg-cyan-500/20 text-cyan-300 px-1.5 py-0.5 rounded-full border border-cyan-500/25">
                          Recommended
                        </span>
                      )}
                      {mode === m.key && <CheckCircle2 size={13} className="text-emerald-400 ml-auto" />}
                    </div>
                    <p className="text-xs text-slate-500 leading-relaxed">{m.desc}</p>
                  </button>
                ))}
              </div>

              <button onClick={finish} disabled={saving}
                className="w-full flex items-center justify-center gap-2 bg-orange-500 hover:bg-orange-400 disabled:opacity-60 text-white font-bold py-3.5 rounded-[14px] transition-all shadow-[0_0_24px_rgba(255,122,26,0.35)] text-sm">
                {saving ? <Loader2 size={15} className="animate-spin" /> : <CheckCircle2 size={15} />}
                {saving ? "Activating…" : "Activate StableGuard"}
              </button>
            </Card>
          </motion.div>
        )}

        {/* ── Step 3: Done ── */}
        {step === 3 && (
          <motion.div key="done" initial={{ opacity: 0, scale: 0.96 }} animate={{ opacity: 1, scale: 1 }}
            transition={{ type: "spring", stiffness: 100, damping: 18 }}
            className="w-full flex flex-col items-center">

            {/* Confetti */}
            <div className="relative w-28 h-28 mb-7">
              {DOTS.map(d => (
                <motion.div key={d.id}
                  initial={{ opacity: 0, y: 0, x: 0, scale: 0 }}
                  animate={{ opacity: [0, 1, 0], y: [0, -(50 + Math.random() * 50)], x: [(d.x - 50) * 1.6, (d.x - 50) * 2.4], scale: [0, 1, 0.4] }}
                  transition={{ delay: d.delay, duration: 1.1, ease: "easeOut" }}
                  style={{ position: "absolute", left: `${d.x}%`, top: "50%", width: d.size, height: d.size, borderRadius: "50%", backgroundColor: d.color }} />
              ))}
              <motion.div initial={{ scale: 0, rotate: -20 }} animate={{ scale: 1, rotate: 0 }}
                transition={{ type: "spring", stiffness: 200, damping: 14, delay: 0.2 }}
                className="absolute inset-0 flex items-center justify-center">
                <div className="w-20 h-20 rounded-[22px] bg-[linear-gradient(135deg,#ff7a1a,#ffb347)] flex items-center justify-center shadow-[0_0_40px_rgba(255,122,26,0.5)]">
                  <Shield size={32} className="text-white" />
                </div>
              </motion.div>
            </div>

            <motion.div initial={{ opacity: 0, y: 16 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.4 }} className="w-full max-w-[440px] text-center">
              <h2 className="font-display font-bold text-3xl text-white mb-2">You&apos;re protected 🛡️</h2>
              <p className="text-slate-400 text-sm mb-7 max-w-xs mx-auto leading-relaxed">
                StableGuard is live. The AI pipeline is watching your vault 24/7 on Solana.
              </p>

              {/* Summary card */}
              <div className="rounded-[20px] border border-white/8 bg-white/[0.03] p-4 mb-5 text-left space-y-2.5">
                {[
                  { icon: "🤖", label: "AI Mode",   value: MODES.find(m => m.key === mode)?.label ?? mode },
                  { icon: "👛", label: "Wallet",    value: wallet ? `${wallet.slice(0, 10)}…${wallet.slice(-6)}` : "Demo account" },
                  { icon: "📱", label: "Telegram",  value: tgStatus === "ok" ? "Connected ✓" : "Not configured" },
                  { icon: "💬", label: "Discord",   value: dcStatus === "ok" ? "Connected ✓" : "Not configured" },
                  { icon: "⛓️",  label: "Network",  value: "Solana Devnet" },
                ].map(item => (
                  <div key={item.label} className="flex items-center justify-between">
                    <span className="text-xs text-slate-500 flex items-center gap-2">
                      <span>{item.icon}</span>{item.label}
                    </span>
                    <span className="text-xs font-semibold text-slate-200">{item.value}</span>
                  </div>
                ))}
              </div>

              <button onClick={() => router.push("/dashboard")}
                className="w-full flex items-center justify-center gap-2 bg-orange-500 hover:bg-orange-400 text-white font-bold py-4 rounded-[16px] transition-all hover:scale-[1.01] shadow-[0_0_28px_rgba(255,122,26,0.4)] text-sm mb-2">
                Open Dashboard <ArrowRight size={16} />
              </button>
              <button onClick={() => router.push("/settings")}
                className="w-full flex items-center justify-center gap-2 text-xs text-slate-600 hover:text-slate-400 py-2.5 transition-colors">
                <ExternalLink size={11} />
                Configure advanced settings
              </button>
            </motion.div>
          </motion.div>
        )}

      </AnimatePresence>

      {/* Step indicator */}
      {step < 3 && (
        <motion.p initial={{ opacity: 0 }} animate={{ opacity: 1 }} transition={{ delay: 0.3 }}
          className="mt-5 text-[11px] text-slate-600 uppercase tracking-[0.18em]">
          Step {step + 1} of 3 — {STEPS[step].label}
        </motion.p>
      )}
    </div>
  );
}
