use anchor_lang::prelude::*;

use crate::errors::StableGuardError;
use crate::state::vault::VaultState;

#[event]
pub struct RebalanceExecuted {
    pub vault: Pubkey,
    /// 0 = A→B (USDC→USDT), 1 = B→A (USDT→USDC)
    pub direction: u8,
    pub amount: u64,
    pub timestamp: i64,
}

#[derive(Accounts)]
pub struct ExecuteRebalance<'info> {
    #[account(mut)]
    pub authority: Signer<'info>,

    #[account(
        mut,
        seeds = [b"vault", authority.key().as_ref()],
        bump = vault.bump,
        has_one = authority @ StableGuardError::UnauthorizedRebalance
    )]
    pub vault: Account<'info, VaultState>,
}

/// Rebalance shifts the virtual allocation between token A and token B.
/// This is an internal accounting update — no cross-mint SPL transfer occurs.
/// In production, the off-chain AI agent would trigger a real swap (e.g. via Jupiter)
/// and then call this instruction to record the updated allocation.
pub fn handle_rebalance(ctx: Context<ExecuteRebalance>, direction: u8, amount: u64) -> Result<()> {
    require!(!ctx.accounts.vault.is_paused, StableGuardError::VaultPaused);
    require!(direction <= 1, StableGuardError::InvalidDirection);
    require!(amount > 0, StableGuardError::InvalidRebalanceAmount);

    let vault_key = ctx.accounts.vault.key();
    let vault = &mut ctx.accounts.vault;

    if direction == 0 {
        // A → B: shift allocation from token A to token B
        require!(vault.balance_a >= amount, StableGuardError::InsufficientFunds);
        vault.balance_a = vault.balance_a.checked_sub(amount).ok_or(StableGuardError::MathOverflow)?;
        vault.balance_b = vault.balance_b.checked_add(amount).ok_or(StableGuardError::MathOverflow)?;
    } else {
        // B → A: shift allocation from token B to token A
        require!(vault.balance_b >= amount, StableGuardError::InsufficientFunds);
        vault.balance_b = vault.balance_b.checked_sub(amount).ok_or(StableGuardError::MathOverflow)?;
        vault.balance_a = vault.balance_a.checked_add(amount).ok_or(StableGuardError::MathOverflow)?;
    }

    vault.total_rebalances = vault
        .total_rebalances
        .checked_add(1)
        .ok_or(StableGuardError::MathOverflow)?;

    let timestamp = Clock::get()?.unix_timestamp;

    emit!(RebalanceExecuted {
        vault: vault_key,
        direction,
        amount,
        timestamp,
    });

    msg!(
        "Rebalance executed: direction={} amount={} balance_a={} balance_b={} total_rebalances={}",
        direction,
        amount,
        vault.balance_a,
        vault.balance_b,
        vault.total_rebalances
    );
    Ok(())
}
