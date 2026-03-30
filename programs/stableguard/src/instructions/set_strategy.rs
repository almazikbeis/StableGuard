use anchor_lang::prelude::*;

use crate::errors::StableGuardError;
use crate::state::vault::VaultState;

#[event]
pub struct StrategyChanged {
    pub vault: Pubkey,
    pub old_mode: u8,
    pub new_mode: u8,
    pub timestamp: i64,
}

#[derive(Accounts)]
pub struct SetStrategy<'info> {
    #[account(mut)]
    pub authority: Signer<'info>,

    #[account(
        mut,
        seeds = [b"vault", authority.key().as_ref()],
        bump = vault.bump,
        has_one = authority @ StableGuardError::Unauthorized
    )]
    pub vault: Account<'info, VaultState>,
}

pub fn handle_set_strategy(ctx: Context<SetStrategy>, mode: u8) -> Result<()> {
    require!(mode <= 1, StableGuardError::InvalidStrategy);

    let vault = &mut ctx.accounts.vault;
    let old_mode = vault.strategy_mode;
    vault.strategy_mode = mode;

    let timestamp = Clock::get()?.unix_timestamp;
    emit!(StrategyChanged {
        vault: vault.key(),
        old_mode,
        new_mode: mode,
        timestamp,
    });

    msg!(
        "Strategy changed: {} → {} (0=safe, 1=yield)",
        old_mode,
        mode
    );
    Ok(())
}
