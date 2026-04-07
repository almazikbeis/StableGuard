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
  asset_type: string; // "stable" | "volatile"
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
    stable_risk: number;
    volatile_risk: number;
    volatile_prices: Record<string, number>;
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
  asset_type: "stable" | "volatile";
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
  ai_agent_model?: string;
  ai_decision_profile?: "cautious" | "balanced" | "aggressive";
  yield_enabled: boolean;
  yield_live_mode: "disabled" | "strategy_only" | "live";
  yield_live_note: string;
  yield_entry_risk: number;
  yield_exit_risk: number;
  circuit_breaker_pause_pct: number;
  alert_risk_threshold: number;
  execution_mode: "record_only" | "custody_scaffold";
  custody_model: "program_owned_vault_accounts" | "external_execution_custody_with_onchain_audit";
  execution_note: string;
  execution_mainnet_rpc?: boolean;
  execution_ready_for_staging?: boolean;
  execution_ready_for_auto_swap?: boolean;
  execution_approval_mode?: "manual" | "auto";
  execution_auto_enabled?: boolean;
  execution_missing_custody_accounts?: string[];
  growth_sleeve_enabled?: boolean;
  growth_sleeve_mode?: "disabled" | "paper" | "live";
  growth_sleeve_budget_pct?: number;
  growth_sleeve_max_asset_pct?: number;
  growth_sleeve_allowed_assets?: string[];
  growth_sleeve_live_execution?: boolean;
  growth_sleeve_ready_for_live?: boolean;
  growth_sleeve_note?: string;
  mode_readiness?: Partial<Record<ControlMode, {
    mode: ControlMode;
    ready: boolean;
    risk: "low" | "medium" | "high" | "unknown";
    summary: string;
    blockers: string[];
  }>>;
  settings_persisted?: boolean;
  settings_updated_at?: string;
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

export interface ExecutionJob {
  id: number;
  ts: number;
  updated_ts: number;
  from_index: number;
  to_index: number;
  amount: number;
  stage: string;
  funding_sig: string;
  swap_sig: string;
  settlement_sig: string;
  settled_amount: number;
  source_symbol: string;
  target_symbol: string;
  custody_account: string;
  target_custody_account: string;
  quote_out_amount: string;
  min_out_amount: string;
  price_impact_pct: string;
  source_balance_before: number;
  source_balance_after: number;
  target_balance_before: number;
  target_balance_after: number;
  simulation_units: number;
  note: string;
  can_submit_swap?: boolean;
  can_settle?: boolean;
  slippage_bps?: number;
  swap_amount?: number;
  route_hops?: number;
  swap_transaction?: string;
  last_valid_block_height?: number;
  prioritization_fee_lamports?: number;
  source_delta?: number;
  target_delta?: number;
  reconciled_amount?: number;
  available_amount_before_settlement?: number;
  explorer?: string;
  message?: string;
}

export interface TelegramNotificationStatus {
  configured: boolean;
  confirmed: boolean;
  telegram_handle?: string;
  phone?: string;
  chat_linked?: boolean;
}

export interface TelegramNotificationLink extends TelegramNotificationStatus {
  deep_link?: string;
  link_token?: string;
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
  executionJobs: (limit = 20) =>
    get<{ execution_jobs: ExecutionJob[] }>(`/history/execution-jobs?limit=${limit}`),
  executionJob: (id: number) =>
    get<ExecutionJob>(`/execution/jobs/${id}`),
  buildExecutionSwap: (id: number, body?: { slippage_bps?: number; amount?: number }) =>
    post<ExecutionJob>(`/execution/jobs/${id}/build-swap`, body ?? {}),
  executeExecutionSwap: (id: number, body?: { slippage_bps?: number; amount?: number }) =>
    post<ExecutionJob>(`/execution/jobs/${id}/execute-swap`, body ?? {}),
  submitExecutionSwap: (id: number, swap_transaction: string) =>
    post<ExecutionJob>(`/execution/jobs/${id}/submit-swap`, { swap_transaction }),
  settleExecutionJob: (id: number, amount = 0) =>
    post<ExecutionJob>(`/execution/jobs/${id}/settle`, { amount }),
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
  telegramNotificationStatus: () =>
    get<TelegramNotificationStatus>("/notifications/telegram/status"),
  telegramNotificationLink: () =>
    get<TelegramNotificationLink>("/notifications/telegram/link"),
  registerTelegramNotification: (body: { telegram_handle?: string; phone?: string }) =>
    post<TelegramNotificationLink & { ok: boolean; message: string }>("/notifications/telegram/register", body),
  sendUserNotificationTest: () =>
    post<{ ok: boolean; message: string }>("/notifications/test-alert", {}),

  setThreshold: (rebalance_threshold: number) =>
    post("/threshold", { rebalance_threshold }),
  pauseVault: () => post("/emergency", {}),
};
