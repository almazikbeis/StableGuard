const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

function authHeaders(): HeadersInit {
  if (typeof window === "undefined") return {};
  const token = window.localStorage.getItem("sg_jwt");
  return token ? { Authorization: `Bearer ${token}` } : {};
}

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    cache: "no-store",
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`API ${path} → ${res.status}`);
  return res.json();
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...authHeaders() },
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
  policy: {
    action_class: string;
    verdict: "allowed" | "blocked" | "requires_approval";
    reason: string;
    control_mode: string;
    auto_execute: boolean;
    yield_enabled: boolean;
    execution_intent: string;
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
  last_exec_status?: string;
  last_exec_note?: string;
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

export interface YieldOpportunity {
  protocol: "kamino" | "marginfi" | "drift";
  display_name: string;
  url: string;
  token: string;
  supply_apy: number;
  borrow_apy: number;
  tvl_millions: number;
  util_rate: number;
  updated_at: number;
  is_live: boolean;
}

export interface YieldPosition {
  id: number;
  protocol: string;
  token: string;
  amount: number;
  entry_apy: number;
  deposited_at: string;
  earned: number;
  deposit_sig: string;
  is_active: boolean;
}

export interface Goal {
  id: number;
  name: string;
  goal_type: string;
  target: number;
  progress: number;
  currency: string;
  deadline?: number;
  created_at: number;
  completed_at?: number;
  is_active: boolean;
}

export interface ChatMessage {
  role: "user" | "assistant";
  content: string;
}

export interface ChatResponse {
  reply: string;
  action?: {
    type: string;
    params: Record<string, unknown>;
    confirm: boolean;
    label: string;
  };
  tokens_used?: number;
}

export interface IntentConfig {
  strategy_mode: number;
  risk_threshold: number;
  yield_entry_risk: number;
  yield_exit_risk: number;
  circuit_breaker_pct: number;
  explanation: string;
  strategy_name: string;
}

export type ControlMode = "MANUAL" | "GUARDED" | "BALANCED" | "YIELD_MAX";

export interface SettingsResponse {
  alerts_enabled: boolean;
  circuit_breaker_enabled: boolean;
  pipeline_running: boolean;
  control_mode: ControlMode | "UNKNOWN";
  strategy_mode: number;
  strategy_name: string;
  auto_execute: boolean;
  yield_enabled: boolean;
  yield_live_mode: "disabled" | "strategy_only" | "live";
  yield_live_note: string;
  yield_entry_risk: number;
  yield_exit_risk: number;
  circuit_breaker_pause_pct: number;
  alert_risk_threshold: number;
  execution_mode: "record_only";
  custody_model: "program_owned_vault_accounts";
  execution_note: string;
  hub_subscribers: number;
}

export interface AppliedAutopilot {
  ok: boolean;
  control_mode: ControlMode | "UNKNOWN";
  strategy_mode: number;
  strategy_name: string;
  auto_execute: boolean;
  yield_enabled: boolean;
  yield_entry_risk: number;
  yield_exit_risk: number;
  circuit_breaker_pct: number;
  risk_threshold: number;
  set_strategy_sig: string;
  update_threshold_sig: string;
  description?: string;
}

// ── API calls ──────────────────────────────────────────────────────────────

async function patch<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json", ...authHeaders() },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`API PATCH ${path} → ${res.status}`);
  return res.json();
}

async function del<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: "DELETE",
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`API DELETE ${path} → ${res.status}`);
  return res.json();
}

export const api = {
  tokens: () => get<TokensResponse>("/tokens"),
  pipeline: () => get<PipelineStatus>("/pipeline/status"),
  vault: () => get<VaultState>("/vault"),
  settings: () => get<SettingsResponse>("/settings"),
  priceHistory: (symbol: string, limit = 200) =>
    get<{ symbol: string; data: PricePoint[] }>(`/history/prices?symbol=${symbol}&limit=${limit}`),
  decisions: (limit = 20) =>
    get<{ decisions: DecisionRow[] }>(`/history/decisions?limit=${limit}`),
  rebalances: (limit = 20) =>
    get<{ rebalances: RebalanceRow[] }>(`/history/rebalances?limit=${limit}`),
  stats: () => get<HistoryStats>("/history/stats"),
  setStrategy: (mode: number) => post("/strategy", { mode }),
  applyControlMode: (mode: ControlMode) =>
    post<AppliedAutopilot>("/settings/control-mode", { mode }),
  applyAutopilot: (body: {
    strategy_mode: number;
    risk_threshold: number;
    yield_entry_risk: number;
    yield_exit_risk: number;
    circuit_breaker_pct: number;
    auto_execute?: boolean;
    yield_enabled?: boolean;
  }) => post<AppliedAutopilot>("/settings/autopilot", body),
  rebalance: (from_index: number, to_index: number, amount: number) =>
    post("/rebalance", { from_index, to_index, amount }),

  // Yield optimizer
  yieldOpportunities: () =>
    get<{ opportunities: YieldOpportunity[]; count: number; updated_at: number }>("/yield/opportunities"),
  yieldPosition: () =>
    get<{ position: YieldPosition | null }>("/yield/position"),
  yieldHistory: (limit = 20) =>
    get<{ positions: YieldPosition[] }>(`/yield/history?limit=${limit}`),

  // Goals
  goals: () => get<{ goals: Goal[]; total_earned: number }>("/goals"),
  createGoal: (name: string, goal_type: string, target: number, deadline?: number) =>
    post<{ id: number }>("/goals", { name, goal_type, target, deadline }),
  updateGoalProgress: (id: number, progress: number) =>
    patch<{ ok: boolean }>(`/goals/${id}/progress`, { progress }),
  deleteGoal: (id: number) => del<{ ok: boolean }>(`/goals/${id}`),

  // AI Chat & Intent
  chat: (message: string, history: ChatMessage[]) =>
    post<ChatResponse>("/chat", { message, history }),
  parseIntent: (intent: string) =>
    post<IntentConfig>("/intent", { intent }),

  setThreshold: (rebalance_threshold: number) =>
    post("/threshold", { rebalance_threshold }),
  pauseVault: () => post("/emergency", {}),
};
