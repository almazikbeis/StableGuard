use anchor_lang::prelude::*;

use crate::errors::StableGuardError;
use crate::state::vault::VaultState;

#[event]
pub struct RebalanceExecuted {
    pub vault: Pubkey,
    pub from_index: u8,
    pub to_index: u8,
    pub amount: u64,
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

/// Rebalance shifts the virtual allocation between two registered token slots.
/// This is an internal accounting update — no cross-mint SPL transfer occurs.
/// The signer can be either the vault authority or the delegated AI agent.
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

    let vault_key = ctx.accounts.vault.key();
    let vault = &mut ctx.accounts.vault;

    vault.balances[from_index as usize] = vault.balances[from_index as usize]
        .checked_sub(amount)
        .ok_or(StableGuardError::MathOverflow)?;
    vault.balances[to_index as usize] = vault.balances[to_index as usize]
        .checked_add(amount)
        .ok_or(StableGuardError::MathOverflow)?;
    vault.total_rebalances = vault
        .total_rebalances
        .checked_add(1)
        .ok_or(StableGuardError::MathOverflow)?;

    let timestamp = Clock::get()?.unix_timestamp;

    emit!(RebalanceExecuted {
        vault: vault_key,
        from_index,
        to_index,
        amount,
        timestamp,
    });

    msg!(
        "Rebalance: from_index={} to_index={} amount={} total_rebalances={}",
        from_index,
        to_index,
        amount,
        vault.total_rebalances
    );
    Ok(())
}
