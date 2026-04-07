use anchor_lang::prelude::*;

pub mod errors;
pub mod instructions;
pub mod state;

pub use instructions::*;

declare_id!("GPSJJqicuDSJ6LXhEZpmUboThjzdefG5wZAZkL2hd7es");

#[program]
pub mod stableguard {
    use super::*;

    // ── Core ──────────────────────────────────────────────────────────────
    pub fn initialize_vault(
        ctx: Context<InitializeVault>,
        params: InitializeVaultParams,
    ) -> Result<()> {
        instructions::initialize::handle_initialize_vault(ctx, params)
    }

    /// Register a new token mint into the vault at the given index (0–7).
    /// Must be called once per token before deposits are accepted for that index.
    pub fn register_token(ctx: Context<RegisterToken>, token_index: u8) -> Result<()> {
        instructions::register_token::handle_register_token(ctx, token_index)
    }

    pub fn deposit(ctx: Context<Deposit>, token_index: u8, amount: u64) -> Result<()> {
        instructions::deposit::handle_deposit(ctx, token_index, amount)
    }

    pub fn withdraw(ctx: Context<Withdraw>, token_index: u8, amount: u64) -> Result<()> {
        instructions::withdraw::handle_withdraw(ctx, token_index, amount)
    }

    /// Records a rebalance intent from from_index to to_index.
    pub fn execute_rebalance(
        ctx: Context<ExecuteRebalance>,
        from_index: u8,
        to_index: u8,
        amount: u64,
    ) -> Result<()> {
        instructions::rebalance::handle_rebalance(ctx, from_index, to_index, amount)
    }

    pub fn toggle_pause(ctx: Context<TogglePause>) -> Result<()> {
        instructions::toggle_pause::handle_toggle_pause(ctx)
    }

    pub fn record_decision(
        ctx: Context<RecordDecision>,
        params: RecordDecisionParams,
    ) -> Result<()> {
        instructions::record_decision::handle_record_decision(ctx, params)
    }

    // ── Additional instructions ───────────────────────────────────────────
    pub fn set_strategy(ctx: Context<SetStrategy>, mode: u8) -> Result<()> {
        instructions::set_strategy::handle_set_strategy(ctx, mode)
    }

    pub fn send_payment(ctx: Context<SendPayment>, token_index: u8, amount: u64) -> Result<()> {
        instructions::send_payment::handle_send_payment(ctx, token_index, amount)
    }

    pub fn update_threshold(ctx: Context<UpdateThreshold>, new_threshold: u64) -> Result<()> {
        instructions::update_threshold::handle_update_threshold(ctx, new_threshold)
    }

    /// Emergency withdraw drains all vault token accounts to authority.
    /// Pass vault token accounts [0..N] then authority accounts [N..2N] as remaining_accounts.
    pub fn emergency_withdraw<'info>(
        ctx: Context<'_, '_, '_, 'info, EmergencyWithdraw<'info>>,
    ) -> Result<()> {
        instructions::emergency_withdraw::handle_emergency_withdraw(ctx)
    }

    /// Delegates an agent pubkey to the vault.
    /// Only the vault authority can call this. The agent can then call
    /// execute_rebalance and update_price_and_check on behalf of the vault.
    pub fn delegate_agent(ctx: Context<DelegateAgent>, agent_pubkey: Pubkey) -> Result<()> {
        instructions::delegate_agent::handle_delegate_agent(ctx, agent_pubkey)
    }

    /// Hot-path price update + on-chain circuit breaker.
    /// Can be called by authority or delegated agent every ~400ms.
    /// Auto-pauses vault if price < circuit_breaker_threshold.
    pub fn update_price_and_check(ctx: Context<UpdatePriceAndCheck>, price: u64) -> Result<()> {
        instructions::update_price::handle_update_price_and_check(ctx, price)
    }

    /// Records the result of a real external swap (e.g., Jupiter) as an on-chain receipt.
    /// Balances should already be updated by real SPL movements; this links the
    /// rebalance intent and external swap to immutable on-chain audit data.
    pub fn record_swap_result(
        ctx: Context<RecordSwapResult>,
        params: RecordSwapResultParams,
    ) -> Result<()> {
        instructions::record_swap_result::handle_record_swap_result(ctx, params)
    }

    /// Admin-only: manually set vault.decision_count.
    /// Use when PDA slots are occupied but counter is out of sync (e.g. after vault re-init).
    pub fn admin_reset_decision_count(
        ctx: Context<AdminResetDecisionCount>,
        new_count: u64,
    ) -> Result<()> {
        instructions::admin_reset::handle_admin_reset_decision_count(ctx, new_count)
    }

    /// Demo-only: directly write accounting balances into the vault without real SPL transfers.
    /// Used on devnet to simulate a funded treasury for AI demo flows.
    pub fn set_demo_balances(
        ctx: Context<SetDemoBalances>,
        balances: Vec<u64>,
        total_deposited: u64,
    ) -> Result<()> {
        instructions::set_demo_balances::handle_set_demo_balances(ctx, balances, total_deposited)
    }
}
