use anchor_lang::prelude::*;

use crate::errors::StableGuardError;
use crate::state::vault::VaultState;

#[event]
pub struct RebalanceExecuted {
    pub vault: Pubkey,
    pub from_index: u8,
    pub to_index: u8,
    pub amount: u64,
    pub from_balance_after: u64,
    pub to_balance_after: u64,
    pub total_rebalances: u64,
    pub timestamp: i64,
}

#[derive(Accounts)]
pub struct ExecuteRebalance<'info> {
    #[account(mut)]
    pub signer: Signer<'info>,

    #[account(
        mut,
        seeds = [b"vault", vault.authority.as_ref()],
        bump = vault.bump,
    )]
    pub vault: Account<'info, VaultState>,
}

/// Executes an AI-initiated rebalance between two registered token slots.
/// Mutates vault accounting (balances[from] → balances[to]) and increments
/// total_rebalances. The signer can be the vault authority or the delegated AI agent.
pub fn handle_rebalance(
    ctx: Context<ExecuteRebalance>,
    from_index: u8,
    to_index: u8,
    amount: u64,
) -> Result<()> {
    require!(
        ctx.accounts
            .vault
            .is_authorized_agent(&ctx.accounts.signer.key()),
        StableGuardError::UnauthorizedAgent
    );
    require!(!ctx.accounts.vault.is_paused, StableGuardError::VaultPaused);
    require!(amount > 0, StableGuardError::InvalidRebalanceAmount);
    require!(from_index != to_index, StableGuardError::InvalidDirection);
    require!(
        (from_index as usize) < ctx.accounts.vault.num_tokens as usize,
        StableGuardError::InvalidTokenIndex
    );
    require!(
        (to_index as usize) < ctx.accounts.vault.num_tokens as usize,
        StableGuardError::InvalidTokenIndex
    );
    require!(
        ctx.accounts.vault.balances[from_index as usize] >= amount,
        StableGuardError::InsufficientFunds
    );

    let vault = &mut ctx.accounts.vault;
    let fi = from_index as usize;
    let ti = to_index as usize;

    // Mutate vault accounting: move balance from source slot to destination slot.
    vault.balances[fi] = vault.balances[fi]
        .checked_sub(amount)
        .ok_or(StableGuardError::MathOverflow)?;
    vault.balances[ti] = vault.balances[ti]
        .checked_add(amount)
        .ok_or(StableGuardError::MathOverflow)?;
    vault.total_rebalances = vault.total_rebalances.checked_add(1).unwrap_or(u64::MAX);

    let vault_key = vault.key();
    let from_bal = vault.balances[fi];
    let to_bal = vault.balances[ti];
    let total_rebalances = vault.total_rebalances;
    let timestamp = Clock::get()?.unix_timestamp;

    emit!(RebalanceExecuted {
        vault: vault_key,
        from_index,
        to_index,
        amount,
        from_balance_after: from_bal,
        to_balance_after: to_bal,
        total_rebalances,
        timestamp,
    });

    msg!(
        "RebalanceExecuted: from_index={} to_index={} amount={} from_bal={} to_bal={} total_rebalances={}",
        from_index,
        to_index,
        amount,
        from_bal,
        to_bal,
        total_rebalances,
    );
    Ok(())
}
