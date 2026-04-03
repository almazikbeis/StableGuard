"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { MessageSquare, X, Send, Loader2, Bot, User, Zap, ChevronDown } from "lucide-react";
import { api, ChatMessage, ChatResponse } from "@/lib/api";
import { toast } from "@/lib/toast";

const SUGGESTED = [
  "What's my current risk level?",
  "Why did you make the last decision?",
  "Switch to safe mode",
  "How much have I earned this week?",
];

interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
  action?: ChatResponse["action"];
  ts: number;
}

export function AIChat() {
  const [open, setOpen] = useState(false);
  const [messages, setMessages] = useState<Message[]>([
    {
      id: "welcome",
      role: "assistant",
      content: "Hi! I'm your StableGuard AI. I can explain vault decisions, adjust settings, and answer questions about your portfolio. What would you like to know?",
      ts: Date.now(),
    },
  ]);
  const [input, setInput]     = useState("");
  const [loading, setLoading] = useState(false);
  const [unread, setUnread]   = useState(0);
  const bottomRef             = useRef<HTMLDivElement>(null);
  const inputRef              = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (open) {
      setUnread(0);
      setTimeout(() => inputRef.current?.focus(), 150);
    }
  }, [open]);

  useEffect(() => {
    if (open) bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, open]);

  const send = useCallback(async (text: string) => {
    if (!text.trim() || loading) return;
    setInput("");

    const userMsg: Message = { id: crypto.randomUUID(), role: "user", content: text, ts: Date.now() };
    setMessages(prev => [...prev, userMsg]);
    setLoading(true);

    try {
      const history: ChatMessage[] = messages
        .filter(m => m.id !== "welcome")
        .map(m => ({ role: m.role, content: m.content }));

      const resp = await api.chat(text, history);

      const aiMsg: Message = {
        id: crypto.randomUUID(),
        role: "assistant",
        content: resp.reply,
        action: resp.action,
        ts: Date.now(),
      };
      setMessages(prev => [...prev, aiMsg]);
      if (!open) setUnread(n => n + 1);
    } catch {
      setMessages(prev => [...prev, {
        id: crypto.randomUUID(),
        role: "assistant",
        content: "Sorry, I couldn't connect to the backend. Make sure the Go server is running.",
        ts: Date.now(),
      }]);
    } finally {
      setLoading(false);
    }
  }, [loading, messages, open]);

  async function executeAction(action: ChatResponse["action"]) {
    if (!action) return;
    try {
      if (action.type === "set_strategy") {
        const mode = (action.params.mode as number) ?? 1;
        if (mode === 0) {
          await api.applyControlMode("GUARDED");
          toast.show("success", "Control mode updated", "Switched to GUARDED mode");
        } else if (mode === 2) {
          await api.applyControlMode("YIELD_MAX");
          toast.show("success", "Control mode updated", "Switched to YIELD MAX mode");
        } else {
          await api.applyControlMode("BALANCED");
          toast.show("success", "Control mode updated", "Switched to BALANCED mode");
        }
      } else if (action.type === "set_threshold") {
        await api.setThreshold((action.params.value as number) ?? 10);
        toast.show("success", "Threshold updated");
      } else if (action.type === "pause") {
        await api.pauseVault();
        toast.show("warning", "Vault paused");
      }
      setMessages(prev => [...prev, {
        id: crypto.randomUUID(),
        role: "assistant",
        content: `Done! ${action.label} was applied successfully.`,
        ts: Date.now(),
      }]);
    } catch (e) {
      toast.show("danger", "Action failed", String(e));
    }
  }

  return (
    <>
      {/* Floating button */}
      <div className="fixed bottom-6 right-6 z-50">
        <AnimatePresence>
          {unread > 0 && !open && (
            <motion.div
              initial={{ scale: 0 }} animate={{ scale: 1 }} exit={{ scale: 0 }}
              className="absolute -top-1 -right-1 w-5 h-5 bg-orange-500 text-white text-[10px] font-bold rounded-full flex items-center justify-center z-10"
            >
              {unread}
            </motion.div>
          )}
        </AnimatePresence>
        <motion.button
          whileHover={{ scale: 1.05 }}
          whileTap={{ scale: 0.95 }}
          onClick={() => setOpen(!open)}
          className={`w-14 h-14 rounded-2xl flex items-center justify-center shadow-xl transition-all ${
            open ? "bg-gray-900" : "bg-orange-500 shadow-orange-200"
          }`}
        >
          <AnimatePresence mode="wait">
            {open ? (
              <motion.div key="x" initial={{ rotate: -90, opacity: 0 }} animate={{ rotate: 0, opacity: 1 }} exit={{ rotate: 90, opacity: 0 }}>
                <ChevronDown size={20} className="text-white" />
              </motion.div>
            ) : (
              <motion.div key="chat" initial={{ rotate: 90, opacity: 0 }} animate={{ rotate: 0, opacity: 1 }} exit={{ rotate: -90, opacity: 0 }}>
                <MessageSquare size={20} className="text-white" />
              </motion.div>
            )}
          </AnimatePresence>
        </motion.button>
      </div>

      {/* Chat panel */}
      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: 24, scale: 0.96 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: 24, scale: 0.96 }}
            transition={{ type: "spring", stiffness: 300, damping: 30 }}
            className="fixed bottom-24 right-6 z-50 w-[380px] max-h-[580px] bg-white rounded-2xl border border-gray-200 shadow-2xl flex flex-col overflow-hidden"
          >
            {/* Header */}
            <div className="flex items-center gap-3 px-4 py-3 border-b border-gray-100 bg-gray-50/80">
              <div className="w-8 h-8 rounded-xl bg-orange-500 flex items-center justify-center">
                <Bot size={15} className="text-white" />
              </div>
              <div className="flex-1">
                <p className="text-sm font-semibold text-gray-900">StableGuard AI</p>
                <p className="text-[10px] text-gray-400">Knows your portfolio · powered by Claude</p>
              </div>
              <button onClick={() => setOpen(false)} className="text-gray-400 hover:text-gray-600 transition-colors">
                <X size={16} />
              </button>
            </div>

            {/* Messages */}
            <div className="flex-1 overflow-y-auto px-4 py-3 space-y-3 min-h-0">
              {messages.map(msg => (
                <div key={msg.id} className={`flex gap-2 ${msg.role === "user" ? "flex-row-reverse" : ""}`}>
                  <div className={`w-6 h-6 rounded-full flex items-center justify-center flex-shrink-0 mt-0.5 ${
                    msg.role === "user" ? "bg-gray-900" : "bg-orange-100"
                  }`}>
                    {msg.role === "user"
                      ? <User size={11} className="text-white" />
                      : <Bot size={11} className="text-orange-500" />
                    }
                  </div>
                  <div className={`max-w-[82%] ${msg.role === "user" ? "items-end" : "items-start"} flex flex-col gap-1.5`}>
                    <div className={`px-3 py-2 rounded-xl text-sm leading-relaxed ${
                      msg.role === "user"
                        ? "bg-gray-900 text-white rounded-tr-sm"
                        : "bg-gray-50 border border-gray-100 text-gray-800 rounded-tl-sm"
                    }`}>
                      {msg.content}
                    </div>
                    {msg.action && (
                      <button
                        onClick={() => executeAction(msg.action)}
                        className="flex items-center gap-1.5 text-xs font-semibold text-orange-600 bg-orange-50 border border-orange-200 px-2.5 py-1.5 rounded-lg hover:bg-orange-100 transition-colors"
                      >
                        <Zap size={11} />
                        {msg.action.label}
                      </button>
                    )}
                  </div>
                </div>
              ))}
              {loading && (
                <div className="flex gap-2">
                  <div className="w-6 h-6 rounded-full bg-orange-100 flex items-center justify-center">
                    <Bot size={11} className="text-orange-500" />
                  </div>
                  <div className="bg-gray-50 border border-gray-100 px-3 py-2 rounded-xl rounded-tl-sm">
                    <Loader2 size={14} className="animate-spin text-gray-400" />
                  </div>
                </div>
              )}
              <div ref={bottomRef} />
            </div>

            {/* Suggestions (only at start) */}
            {messages.length <= 1 && (
              <div className="px-4 pb-2 flex flex-wrap gap-1.5">
                {SUGGESTED.map(s => (
                  <button key={s} onClick={() => send(s)}
                    className="text-xs bg-gray-50 border border-gray-200 text-gray-600 hover:border-orange-300 hover:text-orange-600 px-2.5 py-1 rounded-full transition-colors">
                    {s}
                  </button>
                ))}
              </div>
            )}

            {/* Input */}
            <div className="px-3 py-3 border-t border-gray-100 flex gap-2">
              <input
                ref={inputRef}
                value={input}
                onChange={e => setInput(e.target.value)}
                onKeyDown={e => e.key === "Enter" && !e.shiftKey && send(input)}
                placeholder="Ask anything about your vault…"
                className="flex-1 text-sm bg-gray-50 border border-gray-200 rounded-xl px-3 py-2 focus:outline-none focus:ring-2 focus:ring-orange-200 focus:border-orange-300 transition-all"
              />
              <button
                onClick={() => send(input)}
                disabled={!input.trim() || loading}
                className="w-9 h-9 bg-orange-500 hover:bg-orange-600 disabled:opacity-40 rounded-xl flex items-center justify-center transition-colors flex-shrink-0"
              >
                {loading ? <Loader2 size={14} className="animate-spin text-white" /> : <Send size={14} className="text-white" />}
              </button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
}
