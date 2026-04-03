"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { Shield, ArrowLeft, Send, CheckCircle, XCircle, ExternalLink, Bell, Zap, ChevronRight } from "lucide-react";

const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

function authHeaders(): HeadersInit {
  if (typeof window === "undefined") return {};
  const token = window.localStorage.getItem("sg_jwt");
  return token ? { Authorization: `Bearer ${token}` } : {};
}

async function post(path: string, body: unknown) {
  const r = await fetch(`${BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...authHeaders() },
    body: JSON.stringify(body),
  });
  return r.json();
}

// ── Onboarding steps ───────────────────────────────────────────────────────

const STEPS = [
  { id: "wallet", label: "Connect Wallet", icon: Shield },
  { id: "telegram", label: "Telegram Alerts", icon: Bell },
  { id: "discord", label: "Discord Alerts", icon: Zap },
  { id: "done", label: "All Set", icon: CheckCircle },
];

export default function SettingsPage() {
  const [step, setStep] = useState(0);

  // Telegram
  const [tgToken, setTgToken] = useState("");
  const [tgChatID, setTgChatID] = useState("");
  const [tgStatus, setTgStatus] = useState<"idle" | "saving" | "ok" | "error">("idle");
  const [tgMsg, setTgMsg] = useState("");

  // Discord
  const [dcWebhook, setDcWebhook] = useState("");
  const [dcStatus, setDcStatus] = useState<"idle" | "saving" | "ok" | "error">("idle");
  const [dcMsg, setDcMsg] = useState("");

  // Test alert
  const [testStatus, setTestStatus] = useState<"idle" | "sending" | "ok" | "error">("idle");
  const [testMsg, setTestMsg] = useState("");

  async function saveTelegram() {
    if (!tgToken || !tgChatID) return;
    setTgStatus("saving");
    try {
      const r = await post("/settings/telegram", { bot_token: tgToken, chat_id: tgChatID });
      setTgStatus(r.ok ? "ok" : "error");
      setTgMsg(r.message ?? r.error ?? "");
    } catch {
      setTgStatus("error");
      setTgMsg("Request failed");
    }
  }

  async function saveDiscord() {
    if (!dcWebhook) return;
    setDcStatus("saving");
    try {
      const r = await post("/settings/discord", { webhook_url: dcWebhook });
      setDcStatus(r.ok ? "ok" : "error");
      setDcMsg(r.message ?? r.error ?? "");
    } catch {
      setDcStatus("error");
      setDcMsg("Request failed");
    }
  }

  async function sendTestAlert() {
    setTestStatus("sending");
    try {
      const r = await post("/settings/test-alert", {});
      setTestStatus(r.ok ? "ok" : "error");
      setTestMsg(r.message ?? r.error ?? "");
    } catch {
      setTestStatus("error");
      setTestMsg("Request failed");
    }
  }

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <header className="bg-white border-b border-gray-200 sticky top-0 z-50">
        <div className="max-w-3xl mx-auto px-4 sm:px-6 h-14 flex items-center gap-3">
          <Link href="/" className="p-1.5 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors">
            <ArrowLeft size={16} />
          </Link>
          <div className="w-6 h-6 rounded-md bg-orange-500 flex items-center justify-center">
            <Shield size={12} className="text-white" />
          </div>
          <span className="font-semibold text-gray-900 text-sm">Settings</span>
        </div>
      </header>

      <main className="max-w-3xl mx-auto px-4 sm:px-6 py-8 space-y-6">

        {/* Progress steps */}
        <div className="flex items-center gap-2">
          {STEPS.map((s, i) => (
            <div key={s.id} className="flex items-center gap-2">
              <button
                onClick={() => setStep(i)}
                className={`flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-full transition-colors ${
                  step === i
                    ? "bg-gray-900 text-white"
                    : i < step
                    ? "bg-green-100 text-green-700"
                    : "bg-gray-100 text-gray-500"
                }`}
              >
                {i < step ? <CheckCircle size={11} /> : <s.icon size={11} />}
                <span className="hidden sm:inline">{s.label}</span>
              </button>
              {i < STEPS.length - 1 && <ChevronRight size={12} className="text-gray-300 flex-shrink-0" />}
            </div>
          ))}
        </div>

        {/* Step 0: Wallet */}
        {step === 0 && (
          <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
            <div className="px-6 py-5 border-b border-gray-100">
              <h2 className="font-semibold text-gray-900">Connect Your Wallet</h2>
              <p className="text-sm text-gray-500 mt-1">The backend manages the vault with a server-side keypair.</p>
            </div>
            <div className="p-6 space-y-4">
              <WalletInfo />
              <div className="bg-blue-50 rounded-lg p-4 text-sm text-blue-700">
                <p className="font-medium mb-1">How it works</p>
                <p className="text-blue-600 leading-relaxed">
                  StableGuard uses a dedicated server wallet to execute on-chain transactions.
                  Your wallet address is configured via <code className="bg-blue-100 px-1 rounded">WALLET_KEY_PATH</code> in the backend <code className="bg-blue-100 px-1 rounded">.env</code> file.
                  The AI agents monitor prices and rebalance automatically based on your strategy settings.
                </p>
              </div>
              <button
                onClick={() => setStep(1)}
                className="w-full py-2.5 bg-gray-900 text-white rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors"
              >
                Continue to Alerts setup
              </button>
            </div>
          </div>
        )}

        {/* Step 1: Telegram */}
        {step === 1 && (
          <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
            <div className="px-6 py-5 border-b border-gray-100">
              <h2 className="font-semibold text-gray-900">Telegram Alerts</h2>
              <p className="text-sm text-gray-500 mt-1">
                Get notified when risk spikes or a depeg is detected.
              </p>
            </div>
            <div className="p-6 space-y-5">

              {/* Setup guide */}
              <div className="bg-gray-50 rounded-lg p-4 text-sm text-gray-600 space-y-2">
                <p className="font-medium text-gray-800">Setup guide</p>
                <ol className="list-decimal list-inside space-y-1.5 text-gray-600">
                  <li>
                    Open Telegram and message{" "}
                    <a href="https://t.me/BotFather" target="_blank" rel="noreferrer"
                       className="text-blue-600 hover:underline inline-flex items-center gap-0.5">
                      @BotFather <ExternalLink size={11} />
                    </a>
                  </li>
                  <li>Send <code className="bg-gray-200 px-1 rounded">/newbot</code> and follow the steps to create your bot</li>
                  <li>Copy the <strong>Bot Token</strong> you receive</li>
                  <li>
                    Start your bot, then message{" "}
                    <a href="https://t.me/userinfobot" target="_blank" rel="noreferrer"
                       className="text-blue-600 hover:underline inline-flex items-center gap-0.5">
                      @userinfobot <ExternalLink size={11} />
                    </a>{" "}
                    to get your <strong>Chat ID</strong>
                  </li>
                </ol>
              </div>

              <div className="space-y-3">
                <div>
                  <label className="block text-xs font-medium text-gray-700 mb-1.5">Bot Token</label>
                  <input
                    type="text"
                    placeholder="1234567890:ABCdefGHIjklMNOpqrsTUVwxyz"
                    value={tgToken}
                    onChange={(e) => setTgToken(e.target.value)}
                    className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-orange-500/30 focus:border-orange-400"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-gray-700 mb-1.5">Chat ID</label>
                  <input
                    type="text"
                    placeholder="123456789"
                    value={tgChatID}
                    onChange={(e) => setTgChatID(e.target.value)}
                    className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-orange-500/30 focus:border-orange-400"
                  />
                </div>
              </div>

              {tgMsg && (
                <div className={`flex items-center gap-2 text-sm rounded-lg px-3 py-2 ${
                  tgStatus === "ok" ? "bg-green-50 text-green-700" : "bg-red-50 text-red-700"
                }`}>
                  {tgStatus === "ok" ? <CheckCircle size={14} /> : <XCircle size={14} />}
                  {tgMsg}
                </div>
              )}

              <div className="flex gap-2">
                <button
                  onClick={saveTelegram}
                  disabled={!tgToken || !tgChatID || tgStatus === "saving"}
                  className="flex-1 py-2.5 bg-gray-900 text-white rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors disabled:opacity-40"
                >
                  {tgStatus === "saving" ? "Saving…" : "Save Telegram"}
                </button>
                <button
                  onClick={() => setStep(2)}
                  className="px-4 py-2.5 border border-gray-200 rounded-lg text-sm text-gray-600 hover:bg-gray-50 transition-colors"
                >
                  Skip
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Step 2: Discord */}
        {step === 2 && (
          <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
            <div className="px-6 py-5 border-b border-gray-100">
              <h2 className="font-semibold text-gray-900">Discord Alerts</h2>
              <p className="text-sm text-gray-500 mt-1">Post risk alerts to your Discord channel.</p>
            </div>
            <div className="p-6 space-y-5">

              <div className="bg-gray-50 rounded-lg p-4 text-sm text-gray-600 space-y-2">
                <p className="font-medium text-gray-800">Setup guide</p>
                <ol className="list-decimal list-inside space-y-1.5">
                  <li>Go to your Discord server → channel settings</li>
                  <li>Integrations → Webhooks → New Webhook</li>
                  <li>Copy the Webhook URL</li>
                </ol>
              </div>

              <div>
                <label className="block text-xs font-medium text-gray-700 mb-1.5">Discord Webhook URL</label>
                <input
                  type="text"
                  placeholder="https://discord.com/api/webhooks/..."
                  value={dcWebhook}
                  onChange={(e) => setDcWebhook(e.target.value)}
                  className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-orange-500/30 focus:border-orange-400"
                />
              </div>

              {dcMsg && (
                <div className={`flex items-center gap-2 text-sm rounded-lg px-3 py-2 ${
                  dcStatus === "ok" ? "bg-green-50 text-green-700" : "bg-red-50 text-red-700"
                }`}>
                  {dcStatus === "ok" ? <CheckCircle size={14} /> : <XCircle size={14} />}
                  {dcMsg}
                </div>
              )}

              <div className="flex gap-2">
                <button
                  onClick={saveDiscord}
                  disabled={!dcWebhook || dcStatus === "saving"}
                  className="flex-1 py-2.5 bg-gray-900 text-white rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors disabled:opacity-40"
                >
                  {dcStatus === "saving" ? "Saving…" : "Save Discord"}
                </button>
                <button
                  onClick={() => setStep(3)}
                  className="px-4 py-2.5 border border-gray-200 rounded-lg text-sm text-gray-600 hover:bg-gray-50 transition-colors"
                >
                  Skip
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Step 3: Done + test */}
        {step === 3 && (
          <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
            <div className="px-6 py-5 border-b border-gray-100">
              <h2 className="font-semibold text-gray-900">All Set!</h2>
              <p className="text-sm text-gray-500 mt-1">Your StableGuard is configured and monitoring.</p>
            </div>
            <div className="p-6 space-y-5">

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                {[
                  { label: "Real-time monitoring", desc: "USDC, USDT, DAI, PYUSD" },
                  { label: "AI risk analysis", desc: "Every 30s or on risk jump" },
                  { label: "Circuit breaker", desc: "Auto-pause on depeg > 1.5%" },
                ].map((f) => (
                  <div key={f.label} className="bg-green-50 rounded-lg p-3 border border-green-100">
                    <div className="flex items-center gap-1.5 text-green-700 text-xs font-medium mb-1">
                      <CheckCircle size={12} />
                      {f.label}
                    </div>
                    <p className="text-xs text-green-600">{f.desc}</p>
                  </div>
                ))}
              </div>

              {/* Test alert */}
              <div className="border border-gray-200 rounded-lg p-4">
                <p className="text-sm font-medium text-gray-800 mb-1">Test your alerts</p>
                <p className="text-xs text-gray-500 mb-3">
                  Send a test notification to verify Telegram/Discord is configured correctly.
                </p>
                <button
                  onClick={sendTestAlert}
                  disabled={testStatus === "sending"}
                  className="flex items-center gap-2 px-4 py-2 bg-orange-500 text-white rounded-lg text-sm font-medium hover:bg-orange-600 transition-colors disabled:opacity-40"
                >
                  <Send size={13} />
                  {testStatus === "sending" ? "Sending…" : "Send Test Alert"}
                </button>
                {testMsg && (
                  <div className={`flex items-center gap-2 text-sm mt-2 ${
                    testStatus === "ok" ? "text-green-600" : "text-red-600"
                  }`}>
                    {testStatus === "ok" ? <CheckCircle size={13} /> : <XCircle size={13} />}
                    {testMsg}
                  </div>
                )}
              </div>

              {/* Alert thresholds info */}
              <div className="bg-gray-50 rounded-lg p-4 text-xs text-gray-600 space-y-2">
                <p className="font-medium text-gray-800 text-sm">Circuit Breaker thresholds</p>
                <div className="grid grid-cols-2 gap-2">
                  {[
                    { label: "Depeg warning alert", value: "> 0.5%" },
                    { label: "Vault auto-pause", value: "> 1.5%" },
                    { label: "Emergency alert", value: "> 3.0%" },
                    { label: "Risk alert", value: "level > 80" },
                  ].map((t) => (
                    <div key={t.label} className="flex justify-between bg-white rounded p-2 border border-gray-100">
                      <span>{t.label}</span>
                      <span className="font-mono font-medium text-gray-800">{t.value}</span>
                    </div>
                  ))}
                </div>
                <p className="text-gray-400">Configure in backend <code className="bg-gray-200 px-1 rounded">.env</code> via CIRCUIT_BREAKER_* vars</p>
              </div>

              <Link
                href="/"
                className="flex items-center justify-center gap-2 w-full py-2.5 bg-gray-900 text-white rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors"
              >
                Go to Dashboard
              </Link>
            </div>
          </div>
        )}

      </main>
    </div>
  );
}

function WalletInfo() {
  const [wallet, setWallet] = useState<string | null>(null);

  useEffect(() => {
    fetch(`${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1"}/vault`)
      .then(r => r.json())
      .then(d => setWallet(d.authority))
      .catch(() => {});
  }, []);

  if (!wallet) return (
    <div className="border border-gray-200 rounded-lg p-4 text-sm text-gray-400">
      Could not connect to backend wallet — is the server running?
    </div>
  );

  return (
    <div className="border border-green-200 bg-green-50 rounded-lg p-4">
      <div className="flex items-center gap-2 text-green-700 text-xs font-medium mb-1.5">
        <CheckCircle size={13} />
        Wallet Connected
      </div>
      <p className="text-xs font-mono text-green-800 break-all">{wallet}</p>
      <a
        href={`https://explorer.solana.com/address/${wallet}?cluster=devnet`}
        target="_blank" rel="noreferrer"
        className="inline-flex items-center gap-1 text-xs text-green-600 hover:text-green-800 mt-2"
      >
        View on Explorer <ExternalLink size={10} />
      </a>
    </div>
  );
}
