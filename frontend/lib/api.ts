const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`, { cache: "no-store" });
  if (!res.ok) throw new Error(`API ${path} → ${res.status}`);
  return res.json();
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`API POST ${path} → ${res.status}`);
  return res.json();
}

// ── Types ──────────────────────────────────────────────────────────────────

export interface TokenInfo {
  symbol: string;
  name: string;
  vault_slot: number;
  mainnet_mint: string;
  price: number;
  confidence: number;
  deviation_pct: number;
}

export interface TokensResponse {
  tokens: TokenInfo[];
  fetched_at: string;
  max_deviation: number;
}

export interface PipelineStatus {
  risk: {
    risk_level: number;
    deviation_pct: number;
    trend: number;
    velocity: number;
    volatility: number;
    action: string;
    summary: string;
  };
  decision: {
    action: string;
    from_index: number;
    to_index: number;
    suggested_fraction: number;
    rationale: string;
    confidence: number;
    risk_analysis: string;
    yield_analysis: string;
  } | null;
  last_exec_sig: string;
}

export interface VaultState {
  authority: string;
  num_tokens: number;
  mints: string[];
  balances: number[];
  total_deposited: number;
  rebalance_threshold: number;
  max_deposit: number;
  decision_count: number;
  total_rebalances: number;
  is_paused: boolean;
  strategy_mode: number;
}

export interface DecisionRow {
  id: number;
  ts: number;
  action: string;
  from_index: number;
  to_index: number;
  suggested_fraction: number;
  confidence: number;
  rationale: string;
  risk_analysis: string;
  yield_analysis: string;
  risk_level: number;
  exec_sig: string;
}

export interface RebalanceRow {
  id: number;
  ts: number;
  from_index: number;
  to_index: number;
  amount: number;
  signature: string;
  risk_level: number;
}

export interface PricePoint {
  ts: number;
  price: number;
  conf: number;
}

export interface HistoryStats {
  total_decisions: number;
  total_rebalances: number;
  total_risk_events: number;
  avg_risk_level: number;
  last_decision_ts?: number;
}

// ── API calls ──────────────────────────────────────────────────────────────

export const api = {
  tokens: () => get<TokensResponse>("/tokens"),
  pipeline: () => get<PipelineStatus>("/pipeline/status"),
  vault: () => get<VaultState>("/vault"),
  priceHistory: (symbol: string, limit = 200) =>
    get<{ symbol: string; data: PricePoint[] }>(`/history/prices?symbol=${symbol}&limit=${limit}`),
  decisions: (limit = 20) =>
    get<{ decisions: DecisionRow[] }>(`/history/decisions?limit=${limit}`),
  rebalances: (limit = 20) =>
    get<{ rebalances: RebalanceRow[] }>(`/history/rebalances?limit=${limit}`),
  stats: () => get<HistoryStats>("/history/stats"),
  setStrategy: (mode: number) => post("/strategy", { mode }),
  rebalance: (from_index: number, to_index: number, amount: number) =>
    post("/rebalance", { from_index, to_index, amount }),
};
