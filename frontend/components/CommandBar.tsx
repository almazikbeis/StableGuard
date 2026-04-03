"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Search, Zap, Shield, BarChart2, Send, Settings, ExternalLink, Loader2, Check } from "lucide-react";
import { api } from "@/lib/api";
import { toast } from "@/lib/toast";
import { useRouter } from "next/navigation";

interface Command {
  id: string;
  label: string;
  desc: string;
  icon: React.ElementType;
  category: string;
  action: () => Promise<void> | void;
  keywords: string[];
}

export function CommandBar() {
  const router = useRouter();
  const [open, setOpen]     = useState(false);
  const [query, setQuery]   = useState("");
  const [loading, setLoading] = useState<string | null>(null);
  const [done, setDone]     = useState<string | null>(null);
  const [selected, setSelected] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  const commands: Command[] = [
    {
      id: "mode-manual",
      label: "Switch to MANUAL mode",
      desc: "AI monitors and explains, but does not execute actions.",
      icon: Shield,
      category: "Control Modes",
      keywords: ["manual", "observe", "no auto", "control"],
      action: async () => {
        await api.applyControlMode("MANUAL");
        toast.show("success", "Control mode: MANUAL", "AI is now advisory-only");
      },
    },
    {
      id: "mode-guarded",
      label: "Switch to GUARDED mode",
      desc: "AI intervenes only in high-risk or depeg scenarios.",
      icon: Shield,
      category: "Control Modes",
      keywords: ["guarded", "safe", "protect", "conservative"],
      action: async () => {
        await api.applyControlMode("GUARDED");
        toast.show("success", "Control mode: GUARDED", "AI now focuses on capital protection");
      },
    },
    {
      id: "mode-balanced",
      label: "Switch to BALANCED mode",
      desc: "Moderate automation with protection and measured reallocation.",
      icon: BarChart2,
      category: "Control Modes",
      keywords: ["balanced", "moderate", "default", "autopilot"],
      action: async () => {
        await api.applyControlMode("BALANCED");
        toast.show("success", "Control mode: BALANCED", "AI now runs in balanced automation mode");
      },
    },
    {
      id: "mode-yield",
      label: "Switch to YIELD MAX mode",
      desc: "Aggressive automation with yield enabled and higher risk tolerance.",
      icon: Zap,
      category: "Control Modes",
      keywords: ["yield", "max", "aggressive", "apy", "earn"],
      action: async () => {
        await api.applyControlMode("YIELD_MAX");
        toast.show("success", "Control mode: YIELD MAX", "AI now prioritizes yield within configured limits");
      },
    },
    {
      id: "strategy-safe",
      label: "Set on-chain SAFE strategy",
      desc: "Low-level contract strategy only. Product mode remains unchanged.",
      icon: Shield,
      category: "Strategy",
      keywords: ["safe", "protect", "conservative"],
      action: async () => {
        await api.setStrategy(0);
        toast.show("success", "Strategy: SAFE", "Vault is now in capital preservation mode");
      },
    },
    {
      id: "strategy-balanced",
      label: "Set on-chain BALANCED strategy",
      desc: "Low-level contract strategy only. Product mode remains unchanged.",
      icon: BarChart2,
      category: "Strategy",
      keywords: ["balanced", "moderate", "default"],
      action: async () => {
        await api.setStrategy(1);
        toast.show("success", "Strategy: BALANCED", "Vault is now in balanced mode");
      },
    },
    {
      id: "strategy-yield",
      label: "Set on-chain YIELD strategy",
      desc: "Low-level contract strategy only. Product mode remains unchanged.",
      icon: Zap,
      category: "Strategy",
      keywords: ["yield", "max", "aggressive", "apy", "earn"],
      action: async () => {
        await api.setStrategy(2);
        toast.show("success", "Strategy: YIELD MAX", "Vault is now maximizing yield");
      },
    },
    {
      id: "pause",
      label: "Pause the vault",
      desc: "Emergency stop. Halts all automated actions immediately.",
      icon: Shield,
      category: "Emergency",
      keywords: ["pause", "stop", "emergency", "halt"],
      action: async () => {
        await api.pauseVault();
        toast.show("warning", "Vault paused", "All automated actions halted");
      },
    },
    {
      id: "dashboard",
      label: "Go to Dashboard",
      desc: "Open the main monitoring dashboard",
      icon: BarChart2,
      category: "Navigate",
      keywords: ["dashboard", "home", "main"],
      action: () => router.push("/dashboard"),
    },
    {
      id: "settings",
      label: "Go to Settings",
      desc: "Configure alerts, thresholds, and integrations",
      icon: Settings,
      category: "Navigate",
      keywords: ["settings", "config", "telegram", "discord"],
      action: () => router.push("/settings"),
    },
    {
      id: "solana-explorer",
      label: "Open Solana Explorer",
      desc: "View transactions on Solana Devnet",
      icon: ExternalLink,
      category: "External",
      keywords: ["explorer", "solana", "transaction", "devnet"],
      action: () => window.open("https://explorer.solana.com/?cluster=devnet", "_blank"),
    },
    {
      id: "send",
      label: "Send payment",
      desc: "Route stablecoins from vault to a recipient",
      icon: Send,
      category: "Actions",
      keywords: ["send", "payment", "transfer", "dao"],
      action: () => router.push("/dashboard#payments"),
    },
  ];

  const filtered = query.trim()
    ? commands.filter(c =>
        c.label.toLowerCase().includes(query.toLowerCase()) ||
        c.desc.toLowerCase().includes(query.toLowerCase()) ||
        c.keywords.some(k => k.includes(query.toLowerCase()))
      )
    : commands;

  useEffect(() => { setSelected(0); }, [query]);

  const runCommand = useCallback(async (cmd: Command) => {
    setLoading(cmd.id);
    try {
      await cmd.action();
      setDone(cmd.id);
      setTimeout(() => {
        setDone(null);
        setOpen(false);
        setQuery("");
      }, 600);
    } catch (e) {
      toast.show("danger", "Command failed", String(e));
    } finally {
      setLoading(null);
    }
  }, []);

  // Keyboard shortcut: Cmd+K / Ctrl+K
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setOpen(prev => !prev);
        setQuery("");
      }
      if (e.key === "Escape") { setOpen(false); setQuery(""); }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

  useEffect(() => {
    if (open) setTimeout(() => inputRef.current?.focus(), 50);
  }, [open]);

  // Arrow key navigation
  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === "ArrowDown") { e.preventDefault(); setSelected(s => Math.min(s + 1, filtered.length - 1)); }
    if (e.key === "ArrowUp")   { e.preventDefault(); setSelected(s => Math.max(s - 1, 0)); }
    if (e.key === "Enter" && filtered[selected]) { runCommand(filtered[selected]); }
  }

  const categories = [...new Set(filtered.map(c => c.category))];

  return (
    <>
      {/* Trigger hint in header */}
      <button
        onClick={() => setOpen(true)}
        className="hidden sm:flex items-center gap-2 text-xs text-gray-400 hover:text-gray-600 bg-gray-50 border border-gray-200 rounded-lg px-3 py-1.5 transition-colors"
      >
        <Search size={12} />
        <span>Quick actions</span>
        <kbd className="text-[10px] bg-gray-100 border border-gray-200 rounded px-1 py-0.5 font-mono">⌘K</kbd>
      </button>

      <AnimatePresence>
        {open && (
          <>
            {/* Backdrop */}
            <motion.div
              initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
              className="fixed inset-0 bg-black/20 backdrop-blur-sm z-50"
              onClick={() => { setOpen(false); setQuery(""); }}
            />

            {/* Panel */}
            <motion.div
              initial={{ opacity: 0, scale: 0.96, y: -16 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.96, y: -16 }}
              transition={{ type: "spring", stiffness: 300, damping: 28 }}
              className="fixed top-[20vh] left-1/2 -translate-x-1/2 z-50 w-full max-w-[560px] bg-white rounded-2xl border border-gray-200 shadow-2xl overflow-hidden"
            >
              {/* Search input */}
              <div className="flex items-center gap-3 px-4 py-3.5 border-b border-gray-100">
                <Search size={16} className="text-gray-400 flex-shrink-0" />
                <input
                  ref={inputRef}
                  value={query}
                  onChange={e => setQuery(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder="Search commands… (strategy, pause, navigate)"
                  className="flex-1 text-sm text-gray-900 bg-transparent focus:outline-none placeholder-gray-400"
                />
                <kbd className="text-[10px] text-gray-400 bg-gray-100 border border-gray-200 rounded px-1.5 py-0.5 font-mono">ESC</kbd>
              </div>

              {/* Results */}
              <div className="max-h-[360px] overflow-y-auto py-2">
                {filtered.length === 0 ? (
                  <p className="text-center text-sm text-gray-400 py-8">No commands found for &ldquo;{query}&rdquo;</p>
                ) : (
                  categories.map(cat => (
                    <div key={cat}>
                      <p className="text-[10px] font-semibold text-gray-400 uppercase tracking-wider px-4 py-1.5">{cat}</p>
                      {filtered.filter(c => c.category === cat).map((cmd, idx) => {
                        const globalIdx = filtered.indexOf(cmd);
                        const Icon = cmd.icon;
                        const isSelected = globalIdx === selected;
                        const isLoading = loading === cmd.id;
                        const isDone = done === cmd.id;
                        return (
                          <button
                            key={cmd.id}
                            onClick={() => runCommand(cmd)}
                            onMouseEnter={() => setSelected(globalIdx)}
                            className={`w-full flex items-center gap-3 px-4 py-2.5 text-left transition-colors ${
                              isSelected ? "bg-orange-50" : "hover:bg-gray-50"
                            }`}
                          >
                            <div className={`w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0 ${
                              isSelected ? "bg-orange-100" : "bg-gray-100"
                            }`}>
                              {isDone
                                ? <Check size={14} className="text-green-500" />
                                : isLoading
                                ? <Loader2 size={14} className="animate-spin text-orange-500" />
                                : <Icon size={14} className={isSelected ? "text-orange-600" : "text-gray-500"} />
                              }
                            </div>
                            <div className="flex-1 min-w-0">
                              <p className={`text-sm font-medium truncate ${isSelected ? "text-orange-700" : "text-gray-900"}`}>{cmd.label}</p>
                              <p className="text-xs text-gray-400 truncate">{cmd.desc}</p>
                            </div>
                            {isSelected && (
                              <kbd className="text-[10px] text-gray-400 bg-gray-100 border border-gray-200 rounded px-1.5 py-0.5 font-mono flex-shrink-0">↵</kbd>
                            )}
                          </button>
                        );
                      })}
                    </div>
                  ))
                )}
              </div>

              <div className="border-t border-gray-100 px-4 py-2 flex items-center gap-4 text-[10px] text-gray-400">
                <span>↑↓ navigate</span>
                <span>↵ execute</span>
                <span>ESC close</span>
              </div>
            </motion.div>
          </>
        )}
      </AnimatePresence>
    </>
  );
}
