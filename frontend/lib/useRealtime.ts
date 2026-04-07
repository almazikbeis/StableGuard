"use client";

import { useCallback, useEffect, useRef, useState } from "react";

const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

export interface FeedMessage {
  ts: number;
  risk: {
    risk_level: number;
    deviation_pct: number;
    trend: number;
    velocity: number;
    volatility: number;
    stable_risk: number;
    volatile_risk: number;
    volatile_prices: Record<string, number>;
    action: string;
    summary: string;
    from_index: number;
    to_index: number;
    suggested_fraction: number;
    window_size: number;
  };
  prices: Record<string, number>;
  max_deviation: number;
  policy?: {
    action_class: string;
    verdict: "allowed" | "blocked" | "requires_approval";
    reason: string;
    control_mode: string;
    auto_execute: boolean;
    yield_enabled: boolean;
    execution_intent: string;
  };
  decision?: {
    action: string;
    from_index: number;
    to_index: number;
    suggested_fraction: number;
    rationale: string;
    confidence: number;
    risk_analysis: string;
    yield_analysis: string;
  };
  yield_position?: {
    protocol: string;
    token: string;
    amount: number;
    entry_apy: number;
    earned: number;
    deposited_at: number;
  };
  exec_sig?: string;
  exec_status?: string;
  exec_note?: string;
  hot_path?: {
    last_price: number;
    tripped: boolean;
  };
  ping?: boolean;
}

interface UseRealtimeOptions {
  onMessage?: (msg: FeedMessage) => void;
  fallbackInterval?: number; // ms, default 10000
}

/**
 * Connects to the SSE /stream endpoint.
 * Falls back to polling every `fallbackInterval` ms if SSE fails 3 times.
 */
export function useRealtime(opts: UseRealtimeOptions = {}) {
  const { onMessage, fallbackInterval = 10000 } = opts;
  const [connected, setConnected] = useState(false);
  const [lastMessage, setLastMessage] = useState<FeedMessage | null>(null);
  const [mode, setMode] = useState<"sse" | "polling" | "connecting">("connecting");
  const failuresRef = useRef(0);
  const esRef = useRef<EventSource | null>(null);
  const pollTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const reconnectRef = useRef<() => void>(() => {});

  const handleMsg = useCallback((msg: FeedMessage) => {
    if (msg.ping) return;
    setLastMessage(msg);
    onMessage?.(msg);
  }, [onMessage]);

  const startPolling = useCallback(() => {
    setMode("polling");
    if (pollTimerRef.current) clearInterval(pollTimerRef.current);
    pollTimerRef.current = setInterval(async () => {
      try {
        const r = await fetch(`${BASE}/pipeline/status`);
        if (!r.ok) return;
        const data = await r.json();
        // Adapt pipeline/status to FeedMessage shape
        const msg: FeedMessage = {
          ts: Date.now() / 1000,
          risk: data.risk ?? {},
          prices: {},
          max_deviation: data.risk?.deviation_pct ?? 0,
          policy: data.policy,
          decision: data.decision,
          exec_sig: data.last_exec_sig,
          exec_status: data.last_exec_status,
          exec_note: data.last_exec_note,
        };
        handleMsg(msg);
      } catch { /* ignore */ }
    }, fallbackInterval);
    setConnected(true);
  }, [fallbackInterval, handleMsg]);

  const connectSSE = useCallback(() => {
    if (esRef.current) {
      esRef.current.close();
    }
    const es = new EventSource(`${BASE}/stream`);
    esRef.current = es;

    es.onopen = () => {
      setConnected(true);
      setMode("sse");
      failuresRef.current = 0;
    };

    es.onmessage = (e) => {
      try {
        const msg: FeedMessage = JSON.parse(e.data);
        handleMsg(msg);
      } catch { /* ignore */ }
    };

    es.onerror = () => {
      es.close();
      esRef.current = null;
      setConnected(false);
      failuresRef.current += 1;
      if (failuresRef.current >= 3) {
        startPolling();
      } else {
        // retry after 3s
        setTimeout(() => {
          reconnectRef.current();
        }, 3000);
      }
    };
  }, [handleMsg, startPolling]);

  useEffect(() => {
    reconnectRef.current = connectSSE;
  }, [connectSSE]);

  useEffect(() => {
    connectSSE();
    return () => {
      esRef.current?.close();
      if (pollTimerRef.current) clearInterval(pollTimerRef.current);
    };
  }, [connectSSE]);

  return { connected, mode, lastMessage };
}
