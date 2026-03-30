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
    pub fn initialize_vault(ctx: Context<InitializeVault>, params: InitializeVaultParams) -> Result<()> {
        instructions::initialize::handle_initialize_vault(ctx, params)
    }

    pub fn deposit(ctx: Context<Deposit>, amount: u64, is_token_a: bool) -> Result<()> {
        instructions::deposit::handle_deposit(ctx, amount, is_token_a)
    }

    pub fn withdraw(ctx: Context<Withdraw>, amount: u64, is_token_a: bool) -> Result<()> {
        instructions::withdraw::handle_withdraw(ctx, amount, is_token_a)
    }

    pub fn execute_rebalance(ctx: Context<ExecuteRebalance>, direction: u8, amount: u64) -> Result<()> {
        instructions::rebalance::handle_rebalance(ctx, direction, amount)
    }

    pub fn toggle_pause(ctx: Context<TogglePause>) -> Result<()> {
        instructions::toggle_pause::handle_toggle_pause(ctx)
    }

    pub fn record_decision(ctx: Context<RecordDecision>, params: RecordDecisionParams) -> Result<()> {
        instructions::record_decision::handle_record_decision(ctx, params)
    }

    // ── New instructions ──────────────────────────────────────────────────
    pub fn set_strategy(ctx: Context<SetStrategy>, mode: u8) -> Result<()> {
        instructions::set_strategy::handle_set_strategy(ctx, mode)
    }

    pub fn send_payment(ctx: Context<SendPayment>, amount: u64, is_token_a: bool) -> Result<()> {
        instructions::send_payment::handle_send_payment(ctx, amount, is_token_a)
    }

    pub fn update_threshold(ctx: Context<UpdateThreshold>, new_threshold: u64) -> Result<()> {
        instructions::update_threshold::handle_update_threshold(ctx, new_threshold)
    }

    pub fn emergency_withdraw(ctx: Context<EmergencyWithdraw>) -> Result<()> {
        instructions::emergency_withdraw::handle_emergency_withdraw(ctx)
    }
}
