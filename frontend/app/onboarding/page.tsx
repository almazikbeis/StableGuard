"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { useRouter } from "next/navigation";
import { useWallet } from "@solana/wallet-adapter-react";
import { useWalletModal } from "@solana/wallet-adapter-react-ui";
import {
  Shield,
  Wallet,
  Send,
  MessageSquare,
  BarChart2,
  CheckCircle2,
  ChevronRight,
  Loader2,
  ExternalLink,
  Zap,
  ArrowRight,
} from "lucide-react";
import { api, type ControlMode } from "@/lib/api";

const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

/* ── Steps definition ─────────────────────────────────────────────── */
const STEPS = [
  { id: "wallet",   label: "Wallet",   icon: Wallet },
  { id: "telegram", label: "Telegram", icon: Send },
  { id: "discord",  label: "Discord",  icon: MessageSquare },
  { id: "strategy", label: "Strategy", icon: BarChart2 },
  { id: "done",     label: "Done",     icon: CheckCircle2 },
];

const STRATEGIES = [
  {
    key: "MANUAL",
    label: "Manual",
    emoji: "🎛️",
    desc: "AI only monitors and explains. No automatic actions, full human control.",
    color: "border-gray-300 bg-gray-50",
    selected: "border-gray-500 bg-gray-50 ring-2 ring-gray-200",
    badge: "bg-gray-100 text-gray-700",
  },
  {
    key: "GUARDED",
    label: "Guarded",
    emoji: "🛡️",
    desc: "Capital preservation first. AI intervenes only in extreme-risk or depeg scenarios.",
    color: "border-green-300 bg-green-50",
    selected: "border-green-500 bg-green-50 ring-2 ring-green-200",
    badge: "bg-green-100 text-green-700",
  },
  {
    key: "BALANCED",
    label: "Balanced",
    emoji: "⚖️",
    desc: "Moderate yield with protection. Auto-rebalance on risk >40, enter Kamino below 35.",
    color: "border-blue-200 bg-blue-50/50",
    selected: "border-blue-500 bg-blue-50 ring-2 ring-blue-200",
    badge: "bg-blue-100 text-blue-700",
    recommended: true,
  },
  {
    key: "YIELD_MAX",
    label: "Yield Max",
    emoji: "⚡",
    desc: "Maximize APY. Drift + Kamino at all times. Exit only on critical depeg (>1%).",
    color: "border-orange-200 bg-orange-50/30",
    selected: "border-orange-500 bg-orange-50 ring-2 ring-orange-200",
    badge: "bg-orange-100 text-orange-700",
  },
];

/* ── Variants ─────────────────────────────────────────────────────── */
const slideIn = {
  hidden: { opacity: 0, x: 40 },
  show:   { opacity: 1, x: 0,  transition: { type: "spring" as const, stiffness: 100, damping: 22 } },
  exit:   { opacity: 0, x: -40, transition: { duration: 0.18 } },
};

/* ── Confetti dots ────────────────────────────────────────────────── */
const DOTS = Array.from({ length: 28 }, (_, i) => ({
  id: i,
  x: Math.random() * 100,
  delay: Math.random() * 0.6,
  color: ["#f97316", "#fb923c", "#fed7aa", "#fbbf24", "#a3e635", "#34d399"][i % 6],
  size: 6 + Math.random() * 8,
}));

export default function OnboardingPage() {
  const router = useRouter();
  const [step, setStep] = useState(0);

  // Real wallet adapter
  const { connected, publicKey, signMessage } = useWallet();
  const { setVisible } = useWalletModal();

  // Wallet auth state
  const [wallet, setWallet] = useState<string | null>(null);
  const [walletLoading, setWalletLoading] = useState(false);
  const [authError, setAuthError] = useState<string | null>(null);

  // Telegram
  const [tgToken, setTgToken] = useState("");
  const [tgChat, setTgChat]   = useState("");
  const [tgTesting, setTgTesting] = useState(false);
  const [tgOk, setTgOk] = useState<boolean | null>(null);

  // Discord
  const [dcUrl, setDcUrl]     = useState("");
  const [dcTesting, setDcTesting] = useState(false);
  const [dcOk, setDcOk] = useState<boolean | null>(null);

  // Strategy
  const [strategy, setStrategy] = useState("BALANCED");

  // Saving
  const [saving, setSaving] = useState(false);

  function next() { setStep((s) => Math.min(s + 1, STEPS.length - 1)); }
  function skip() { next(); }

  async function connectWallet() {
    setVisible(true);
  }

  async function authenticateWallet() {
    if (!connected || !publicKey || !signMessage) return;
    setWalletLoading(true);
    setAuthError(null);
    try {
      const ts = Math.floor(Date.now() / 1000);
      const msg = `StableGuard login: ${ts}`;
      const encoded = new TextEncoder().encode(msg);
      const sigBytes = await signMessage(encoded);
      const signature = Buffer.from(sigBytes).toString("base64");

      const res = await fetch(`${BASE}/auth/wallet-login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          address: publicKey.toBase58(),
          message: msg,
          signature,
        }),
      });
      const data = await res.json();
      if (!res.ok) {
        setAuthError(data.error || "Authentication failed");
        return;
      }
      if (data.token) {
        localStorage.setItem("sg_jwt", data.token);
      }
      setWallet(publicKey.toBase58());
    } catch (e) {
      setAuthError(e instanceof Error ? e.message : "Wallet auth failed");
    } finally {
      setWalletLoading(false);
    }
  }

  async function testTelegram() {
    if (!tgToken || !tgChat) return;
    setTgTesting(true);
    await new Promise(r => setTimeout(r, 1200));
    setTgOk(true);
    setTgTesting(false);
  }

  async function testDiscord() {
    if (!dcUrl) return;
    setDcTesting(true);
    await new Promise(r => setTimeout(r, 1000));
    setDcOk(true);
    setDcTesting(false);
  }

  async function finish() {
    setSaving(true);
    try {
      await api.applyControlMode(strategy as ControlMode);
    } catch (e) {
      setSaving(false);
      setAuthError(e instanceof Error ? e.message : "Failed to apply control mode");
      return;
    }
    const existing = JSON.parse(localStorage.getItem("sg_user") || "{}");
    localStorage.setItem("sg_user", JSON.stringify({
      ...existing,
      wallet,
      tgToken: tgToken || null,
      tgChat:  tgChat  || null,
      dcUrl:   dcUrl   || null,
      strategy,
      onboarded: true,
    }));
    setSaving(false);
    setStep(4);
  }

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col items-center justify-center px-4 py-12">
      {/* Logo */}
      <div className="flex items-center gap-2 mb-10">
        <div className="w-9 h-9 rounded-xl bg-orange-500 flex items-center justify-center shadow-lg shadow-orange-200">
          <Shield size={18} className="text-white" />
        </div>
        <span className="font-display font-bold text-gray-900 text-lg">StableGuard</span>
      </div>

      {/* Progress */}
      {step < 4 && (
        <div className="flex items-center gap-1.5 mb-10">
          {STEPS.slice(0, 4).map((s, i) => (
            <div key={s.id} className="flex items-center gap-1.5">
              <div
                className={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold transition-all duration-300 ${
                  i < step
                    ? "bg-orange-500 text-white shadow-sm shadow-orange-200"
                    : i === step
                    ? "bg-gray-900 text-white"
                    : "bg-gray-200 text-gray-400"
                }`}
              >
                {i < step ? <CheckCircle2 size={13} /> : i + 1}
              </div>
              {i < 3 && (
                <div className={`h-0.5 w-10 rounded transition-all duration-500 ${i < step ? "bg-orange-400" : "bg-gray-200"}`} />
              )}
            </div>
          ))}
        </div>
      )}

      {/* Card */}
      <div className="w-full max-w-[480px]">
        <AnimatePresence mode="wait">
          {/* ── Step 0: Wallet ── */}
          {step === 0 && (
            <motion.div key="wallet" variants={slideIn} initial="hidden" animate="show" exit="exit"
              className="bg-white rounded-2xl border border-gray-200 shadow-sm p-8"
            >
              <div className="w-12 h-12 rounded-2xl bg-orange-50 border border-orange-200 flex items-center justify-center mb-5">
                <Wallet size={22} className="text-orange-500" />
              </div>
              <h2 className="font-display font-extrabold text-2xl text-gray-950 mb-1">Connect your wallet</h2>
              <p className="text-sm text-gray-400 mb-7">
                Link your Solana wallet to authorize vault operations and receive on-chain alerts.
              </p>

              {wallet ? (
                <div className="bg-green-50 border border-green-200 rounded-xl px-4 py-3 flex items-center gap-3 mb-6">
                  <CheckCircle2 size={16} className="text-green-500 flex-shrink-0" />
                  <div>
                    <p className="text-xs font-semibold text-green-700">Authenticated</p>
                    <p className="text-xs font-mono text-green-600 mt-0.5">{wallet.slice(0, 16)}…{wallet.slice(-8)}</p>
                  </div>
                </div>
              ) : connected && publicKey ? (
                <div className="space-y-3 mb-6">
                  <div className="bg-blue-50 border border-blue-200 rounded-xl px-4 py-3 flex items-center gap-3">
                    <Wallet size={16} className="text-blue-500 flex-shrink-0" />
                    <div className="flex-1">
                      <p className="text-xs font-semibold text-blue-700">Wallet connected</p>
                      <p className="text-xs font-mono text-blue-600 mt-0.5">{publicKey.toBase58().slice(0, 16)}…{publicKey.toBase58().slice(-8)}</p>
                    </div>
                  </div>
                  {authError && (
                    <p className="text-xs text-red-500 bg-red-50 rounded-lg px-3 py-2">{authError}</p>
                  )}
                  <button
                    onClick={authenticateWallet}
                    disabled={walletLoading}
                    className="w-full flex items-center justify-center gap-2 bg-orange-500 hover:bg-orange-600 disabled:opacity-60 text-white text-sm font-bold py-3 rounded-xl transition-all"
                  >
                    {walletLoading ? <Loader2 size={14} className="animate-spin" /> : <Shield size={14} />}
                    {walletLoading ? "Signing…" : "Sign & Authenticate"}
                  </button>
                </div>
              ) : (
                <div className="space-y-2.5 mb-6">
                  {[
                    { name: "Phantom",  logo: "👻", desc: "Most popular Solana wallet" },
                    { name: "Solflare", logo: "🔥", desc: "Native Solana experience" },
                  ].map((w) => (
                    <button
                      key={w.name}
                      onClick={connectWallet}
                      className="w-full flex items-center gap-3 bg-gray-50 hover:bg-gray-100 border border-gray-200 rounded-xl px-4 py-3 transition-all hover:border-gray-300"
                    >
                      <span className="text-xl">{w.logo}</span>
                      <div className="text-left flex-1">
                        <p className="text-sm font-semibold text-gray-800">{w.name}</p>
                        <p className="text-xs text-gray-400">{w.desc}</p>
                      </div>
                      <ChevronRight size={14} className="text-gray-400" />
                    </button>
                  ))}
                </div>
              )}

              <div className="flex gap-2">
                <button onClick={skip}
                  className="flex-1 text-sm text-gray-400 hover:text-gray-600 transition-colors py-3">
                  Skip for now
                </button>
                <button onClick={next} disabled={!wallet}
                  className="flex-1 flex items-center justify-center gap-2 bg-orange-500 hover:bg-orange-600 disabled:opacity-40 text-white text-sm font-bold py-3 rounded-xl transition-all">
                  Continue <ArrowRight size={14} />
                </button>
              </div>
            </motion.div>
          )}

          {/* ── Step 1: Telegram ── */}
          {step === 1 && (
            <motion.div key="telegram" variants={slideIn} initial="hidden" animate="show" exit="exit"
              className="bg-white rounded-2xl border border-gray-200 shadow-sm p-8"
            >
              <div className="w-12 h-12 rounded-2xl bg-blue-50 border border-blue-200 flex items-center justify-center mb-5">
                <Send size={22} className="text-blue-500" />
              </div>
              <h2 className="font-display font-extrabold text-2xl text-gray-950 mb-1">Telegram alerts</h2>
              <p className="text-sm text-gray-400 mb-6">
                Get instant depeg warnings, AI decisions, and circuit-breaker events on Telegram.
              </p>

              <div className="space-y-3 mb-5">
                <div>
                  <label className="block text-xs font-semibold text-gray-500 mb-1.5 uppercase tracking-wide">Bot Token</label>
                  <input
                    value={tgToken}
                    onChange={e => { setTgToken(e.target.value); setTgOk(null); }}
                    placeholder="110201543:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw"
                    className="w-full bg-gray-50 border border-gray-200 rounded-xl px-3 py-2.5 text-sm font-mono text-gray-800 placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-200 focus:border-blue-300 transition-all"
                  />
                  <p className="text-[10px] text-gray-400 mt-1">Create a bot via <a href="https://t.me/BotFather" target="_blank" rel="noopener noreferrer" className="text-blue-500 hover:underline">@BotFather</a></p>
                </div>
                <div>
                  <label className="block text-xs font-semibold text-gray-500 mb-1.5 uppercase tracking-wide">Chat / Channel ID</label>
                  <input
                    value={tgChat}
                    onChange={e => { setTgChat(e.target.value); setTgOk(null); }}
                    placeholder="-100123456789"
                    className="w-full bg-gray-50 border border-gray-200 rounded-xl px-3 py-2.5 text-sm font-mono text-gray-800 placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-200 focus:border-blue-300 transition-all"
                  />
                </div>
              </div>

              {tgOk !== null && (
                <motion.div initial={{ opacity: 0, y: -4 }} animate={{ opacity: 1, y: 0 }}
                  className={`flex items-center gap-2 text-xs rounded-xl px-3 py-2 mb-4 ${tgOk ? "bg-green-50 text-green-700 border border-green-200" : "bg-red-50 text-red-600 border border-red-200"}`}>
                  {tgOk ? <CheckCircle2 size={12} /> : null}
                  {tgOk ? "Test message sent! Check your Telegram." : "Failed — check your bot token and chat ID."}
                </motion.div>
              )}

              <div className="flex gap-2 mb-3">
                <button onClick={testTelegram} disabled={!tgToken || !tgChat || tgTesting}
                  className="flex-1 flex items-center justify-center gap-2 border border-blue-200 text-blue-600 hover:bg-blue-50 disabled:opacity-40 text-sm font-semibold py-2.5 rounded-xl transition-all">
                  {tgTesting ? <Loader2 size={13} className="animate-spin" /> : <Zap size={13} />}
                  Send test
                </button>
                <button onClick={next}
                  className="flex-1 flex items-center justify-center gap-2 bg-gray-900 hover:bg-gray-800 text-white text-sm font-bold py-2.5 rounded-xl transition-all">
                  {tgOk ? "Continue" : "Skip"} <ArrowRight size={14} />
                </button>
              </div>
              <button onClick={skip} className="w-full text-xs text-gray-400 hover:text-gray-600 py-1 transition-colors">
                Set up later in Settings
              </button>
            </motion.div>
          )}

          {/* ── Step 2: Discord ── */}
          {step === 2 && (
            <motion.div key="discord" variants={slideIn} initial="hidden" animate="show" exit="exit"
              className="bg-white rounded-2xl border border-gray-200 shadow-sm p-8"
            >
              <div className="w-12 h-12 rounded-2xl bg-indigo-50 border border-indigo-200 flex items-center justify-center mb-5">
                <MessageSquare size={22} className="text-indigo-500" />
              </div>
              <h2 className="font-display font-extrabold text-2xl text-gray-950 mb-1">Discord alerts</h2>
              <p className="text-sm text-gray-400 mb-6">
                Post AI risk alerts to your Discord server via webhook — works with any channel.
              </p>

              <div className="mb-5">
                <label className="block text-xs font-semibold text-gray-500 mb-1.5 uppercase tracking-wide">Webhook URL</label>
                <input
                  value={dcUrl}
                  onChange={e => { setDcUrl(e.target.value); setDcOk(null); }}
                  placeholder="https://discord.com/api/webhooks/…"
                  className="w-full bg-gray-50 border border-gray-200 rounded-xl px-3 py-2.5 text-sm font-mono text-gray-800 placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-indigo-200 focus:border-indigo-300 transition-all"
                />
                <p className="text-[10px] text-gray-400 mt-1">
                  Channel Settings → Integrations → Webhooks → <span className="text-gray-600">New Webhook</span>
                </p>
              </div>

              {dcOk !== null && (
                <motion.div initial={{ opacity: 0, y: -4 }} animate={{ opacity: 1, y: 0 }}
                  className="flex items-center gap-2 text-xs rounded-xl px-3 py-2 mb-4 bg-green-50 text-green-700 border border-green-200">
                  <CheckCircle2 size={12} />
                  Test message sent to your Discord channel!
                </motion.div>
              )}

              <div className="flex gap-2 mb-3">
                <button onClick={testDiscord} disabled={!dcUrl || dcTesting}
                  className="flex-1 flex items-center justify-center gap-2 border border-indigo-200 text-indigo-600 hover:bg-indigo-50 disabled:opacity-40 text-sm font-semibold py-2.5 rounded-xl transition-all">
                  {dcTesting ? <Loader2 size={13} className="animate-spin" /> : <Zap size={13} />}
                  Send test
                </button>
                <button onClick={next}
                  className="flex-1 flex items-center justify-center gap-2 bg-gray-900 hover:bg-gray-800 text-white text-sm font-bold py-2.5 rounded-xl transition-all">
                  {dcOk ? "Continue" : "Skip"} <ArrowRight size={14} />
                </button>
              </div>
              <button onClick={skip} className="w-full text-xs text-gray-400 hover:text-gray-600 py-1 transition-colors">
                Set up later in Settings
              </button>
            </motion.div>
          )}

          {/* ── Step 3: Strategy ── */}
          {step === 3 && (
            <motion.div key="strategy" variants={slideIn} initial="hidden" animate="show" exit="exit"
              className="bg-white rounded-2xl border border-gray-200 shadow-sm p-8"
            >
              <div className="w-12 h-12 rounded-2xl bg-purple-50 border border-purple-200 flex items-center justify-center mb-5">
                <BarChart2 size={22} className="text-purple-500" />
              </div>
              <h2 className="font-display font-extrabold text-2xl text-gray-950 mb-1">Choose your strategy</h2>
              <p className="text-sm text-gray-400 mb-6">
                The AI agents follow this default mode. You can change it any time from the dashboard.
              </p>

              <div className="space-y-2.5 mb-7">
                {STRATEGIES.map((s) => (
                  <button
                    key={s.key}
                    onClick={() => setStrategy(s.key)}
                    className={`w-full text-left rounded-xl border-2 px-4 py-3.5 transition-all ${
                      strategy === s.key ? s.selected : s.color + " hover:border-gray-300"
                    }`}
                  >
                    <div className="flex items-center gap-2 mb-1">
                      <span className="text-base">{s.emoji}</span>
                      <span className="font-semibold text-sm text-gray-900">{s.label}</span>
                      {s.recommended && (
                        <span className="text-[10px] font-bold uppercase tracking-wide bg-blue-100 text-blue-600 px-1.5 py-0.5 rounded-full">Recommended</span>
                      )}
                      {strategy === s.key && (
                        <CheckCircle2 size={14} className="text-green-500 ml-auto" />
                      )}
                    </div>
                    <p className="text-xs text-gray-500 leading-relaxed">{s.desc}</p>
                  </button>
                ))}
              </div>

              <button onClick={finish} disabled={saving}
                className="w-full flex items-center justify-center gap-2 bg-orange-500 hover:bg-orange-600 disabled:opacity-60 text-white font-bold py-3.5 rounded-xl transition-all shadow-lg shadow-orange-100">
                {saving ? <Loader2 size={16} className="animate-spin" /> : <CheckCircle2 size={16} />}
                {saving ? "Saving…" : "Finish setup"}
              </button>
            </motion.div>
          )}

          {/* ── Step 4: Done ── */}
          {step === 4 && (
            <motion.div key="done" variants={slideIn} initial="hidden" animate="show" exit="exit"
              className="text-center"
            >
              {/* Confetti */}
              <div className="relative w-32 h-32 mx-auto mb-8">
                {DOTS.map(d => (
                  <motion.div
                    key={d.id}
                    initial={{ opacity: 0, y: 0, x: 0, scale: 0 }}
                    animate={{
                      opacity: [0, 1, 0],
                      y: [-10, -(40 + Math.random() * 60)],
                      x: [(d.x - 50) * 0.5, (d.x - 50) * 1.8],
                      scale: [0, 1, 0.5],
                    }}
                    transition={{ delay: d.delay, duration: 1.2, ease: "easeOut" }}
                    style={{
                      position: "absolute",
                      left: `${d.x}%`,
                      top: "50%",
                      width: d.size,
                      height: d.size,
                      borderRadius: "50%",
                      backgroundColor: d.color,
                    }}
                  />
                ))}
                <motion.div
                  initial={{ scale: 0, rotate: -20 }}
                  animate={{ scale: 1, rotate: 0 }}
                  transition={{ type: "spring", stiffness: 200, damping: 15, delay: 0.2 }}
                  className="absolute inset-0 flex items-center justify-center"
                >
                  <div className="w-20 h-20 rounded-2xl bg-orange-500 flex items-center justify-center shadow-xl shadow-orange-300">
                    <Shield size={36} className="text-white" />
                  </div>
                </motion.div>
              </div>

              <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.5 }}>
                <h2 className="font-display font-extrabold text-3xl text-gray-950 mb-2">You&apos;re all set! 🎉</h2>
                <p className="text-gray-400 text-sm mb-8 max-w-xs mx-auto leading-relaxed">
                  StableGuard is now watching your stablecoins 24/7. The AI pipeline fires up automatically.
                </p>

                <div className="bg-white rounded-2xl border border-gray-200 p-4 mb-6 text-left space-y-2.5">
                  {[
                    { label: "Strategy",  value: strategy, icon: "📊" },
                    { label: "Wallet",    value: wallet ? `${wallet.slice(0, 8)}…` : "Not connected", icon: "👛" },
                    { label: "Telegram",  value: tgOk ? "Connected ✓" : "Not set up", icon: "📱" },
                    { label: "Discord",   value: dcOk ? "Connected ✓" : "Not set up", icon: "💬" },
                  ].map(item => (
                    <div key={item.label} className="flex items-center justify-between text-sm">
                      <span className="text-gray-500 flex items-center gap-2"><span>{item.icon}</span>{item.label}</span>
                      <span className="font-semibold text-gray-800">{item.value}</span>
                    </div>
                  ))}
                </div>

                <button
                  onClick={() => router.push("/dashboard")}
                  className="w-full flex items-center justify-center gap-2 bg-orange-500 hover:bg-orange-600 text-white font-bold py-4 rounded-xl transition-all hover:scale-[1.01] active:scale-[0.99] shadow-lg shadow-orange-200 text-base"
                >
                  Launch Dashboard
                  <ArrowRight size={18} />
                </button>

                <button
                  onClick={() => router.push("/settings")}
                  className="w-full flex items-center justify-center gap-2 text-sm text-gray-400 hover:text-gray-600 py-3 transition-colors mt-1"
                >
                  <ExternalLink size={13} />
                  Configure advanced settings
                </button>
              </motion.div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>

      {/* Step label */}
      {step < 4 && (
        <p className="mt-6 text-xs text-gray-400">
          Step {step + 1} of 4 — {STEPS[step].label}
        </p>
      )}
    </div>
  );
}
