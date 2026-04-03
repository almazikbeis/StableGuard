"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Send, CheckCircle, AlertCircle, ExternalLink, Loader2 } from "lucide-react";
import { toast } from "@/lib/toast";

const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

function authHeaders(): HeadersInit {
  if (typeof window === "undefined") return {};
  const token = window.localStorage.getItem("sg_jwt");
  return token ? { Authorization: `Bearer ${token}` } : {};
}

const TOKENS = [
  { label: "USDC", index: 0, decimals: 6 },
  { label: "USDT", index: 1, decimals: 6 },
  { label: "DAI",  index: 2, decimals: 9 },
  { label: "PYUSD",index: 3, decimals: 6 },
];

interface TxResult {
  sig: string;
  token: string;
  amount: string;
  recipient: string;
}

export function DAOPayments() {
  const [recipient, setRecipient] = useState("");
  const [tokenIdx, setTokenIdx] = useState(0);
  const [amount, setAmount] = useState("");
  const [sending, setSending] = useState(false);
  const [lastTx, setLastTx] = useState<TxResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  const selectedToken = TOKENS[tokenIdx];

  async function handleSend(e: React.FormEvent) {
    e.preventDefault();
    setError(null);

    const amountNum = parseFloat(amount);
    if (!recipient.trim() || isNaN(amountNum) || amountNum <= 0) {
      setError("Fill in all fields with valid values");
      return;
    }
    // Basic Solana address length check
    if (recipient.trim().length < 32 || recipient.trim().length > 44) {
      setError("Invalid Solana token account address");
      return;
    }

    // Convert to token units (USDC/USDT = 6 decimals)
    const rawAmount = Math.floor(amountNum * Math.pow(10, selectedToken.decimals));

    setSending(true);
    try {
      const res = await fetch(`${BASE}/send`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({
          token_index: selectedToken.index,
          amount: rawAmount,
          recipient: recipient.trim(),
        }),
      });

      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.error || `HTTP ${res.status}`);
      }

      const data = await res.json();
      const sig = data.signature || data.sig || "confirmed";

      setLastTx({
        sig,
        token: selectedToken.label,
        amount: amountNum.toLocaleString(),
        recipient: recipient.trim(),
      });

      toast.show("success", "Payment sent!", `${amountNum} ${selectedToken.label} → ${recipient.slice(0, 8)}…`);
      setRecipient("");
      setAmount("");
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Unknown error";
      setError(msg);
      toast.show("danger", "Payment failed", msg);
    } finally {
      setSending(false);
    }
  }

  return (
    <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
      {/* Header */}
      <div className="px-4 pt-4 pb-3 border-b border-gray-100 flex items-center gap-2">
        <Send size={14} className="text-blue-500" />
        <span className="text-sm font-semibold text-gray-900">DAO Treasury Payment</span>
        <span className="text-xs text-gray-400">Route stablecoins from vault to any recipient</span>
      </div>

      <div className="p-4 space-y-4">
        {/* Form */}
        <form onSubmit={handleSend} className="space-y-3">
          {/* Recipient */}
          <div>
            <label className="block text-xs font-medium text-gray-500 mb-1">
              Recipient Token Account
            </label>
            <input
              type="text"
              value={recipient}
              onChange={(e) => setRecipient(e.target.value)}
              placeholder="Hx3kF... (Solana token account address)"
              className="w-full text-sm bg-gray-50 border border-gray-200 rounded-lg px-3 py-2 font-mono text-gray-800 placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-200 focus:border-blue-300 transition-all"
            />
          </div>

          {/* Token + Amount row */}
          <div className="flex gap-2">
            {/* Token selector */}
            <div className="w-28 flex-shrink-0">
              <label className="block text-xs font-medium text-gray-500 mb-1">Token</label>
              <div className="flex gap-1 flex-wrap">
                {TOKENS.map((t) => (
                  <button
                    key={t.index}
                    type="button"
                    onClick={() => setTokenIdx(t.index)}
                    className={`text-xs px-2 py-1.5 rounded-lg transition-colors font-medium ${
                      tokenIdx === t.index
                        ? "bg-gray-900 text-white"
                        : "bg-gray-100 text-gray-600 hover:bg-gray-200"
                    }`}
                  >
                    {t.label}
                  </button>
                ))}
              </div>
            </div>

            {/* Amount */}
            <div className="flex-1">
              <label className="block text-xs font-medium text-gray-500 mb-1">
                Amount ({selectedToken.label})
              </label>
              <input
                type="number"
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                placeholder="1000.00"
                min="0"
                step="0.000001"
                className="w-full text-sm bg-gray-50 border border-gray-200 rounded-lg px-3 py-2 font-mono-data text-gray-800 placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-200 focus:border-blue-300 transition-all"
              />
            </div>
          </div>

          {/* Error */}
          <AnimatePresence>
            {error && (
              <motion.div
                initial={{ opacity: 0, height: 0 }}
                animate={{ opacity: 1, height: "auto" }}
                exit={{ opacity: 0, height: 0 }}
                className="flex items-center gap-2 text-xs text-red-600 bg-red-50 rounded-lg px-3 py-2"
              >
                <AlertCircle size={12} />
                {error}
              </motion.div>
            )}
          </AnimatePresence>

          {/* Submit */}
          <button
            type="submit"
            disabled={sending}
            className="w-full flex items-center justify-center gap-2 bg-gray-900 hover:bg-gray-800 disabled:opacity-50 text-white text-sm font-semibold px-4 py-2.5 rounded-xl transition-all"
          >
            {sending ? (
              <><Loader2 size={14} className="animate-spin" /> Sending…</>
            ) : (
              <><Send size={14} /> Send Payment</>
            )}
          </button>
        </form>

        {/* Last transaction */}
        <AnimatePresence>
          {lastTx && (
            <motion.div
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -8 }}
              className="bg-green-50 border border-green-200 rounded-xl p-3"
            >
              <div className="flex items-center gap-2 mb-2">
                <CheckCircle size={14} className="text-green-500" />
                <span className="text-xs font-semibold text-green-700">Transaction confirmed</span>
              </div>
              <div className="grid grid-cols-2 gap-2 text-xs mb-2">
                <div>
                  <span className="text-gray-400">Amount</span>
                  <p className="font-mono font-semibold text-gray-800">
                    {lastTx.amount} {lastTx.token}
                  </p>
                </div>
                <div>
                  <span className="text-gray-400">Recipient</span>
                  <p className="font-mono font-semibold text-gray-800">
                    {lastTx.recipient.slice(0, 12)}…
                  </p>
                </div>
              </div>
              {lastTx.sig !== "confirmed" && (
                <a
                  href={`https://explorer.solana.com/tx/${lastTx.sig}?cluster=devnet`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center gap-1 text-xs text-green-600 hover:text-green-800 font-mono"
                >
                  <ExternalLink size={10} />
                  {lastTx.sig.slice(0, 20)}…
                </a>
              )}
            </motion.div>
          )}
        </AnimatePresence>

        {/* Info note */}
        <p className="text-[10px] text-gray-400 leading-relaxed">
          Sends tokens directly from the vault via SPL Token CPI. Authority wallet signs the transaction.
          Vault must not be paused. Token account must match the selected token mint.
        </p>
      </div>
    </div>
  );
}
