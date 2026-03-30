use anchor_lang::prelude::*;

use crate::errors::StableGuardError;
use crate::state::vault::VaultState;

#[event]
pub struct ThresholdUpdated {
    pub vault: Pubkey,
    pub old_threshold: u64,
    pub new_threshold: u64,
    pub timestamp: i64,
}

#[derive(Accounts)]
pub struct UpdateThreshold<'info> {
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

pub fn handle_update_threshold(ctx: Context<UpdateThreshold>, new_threshold: u64) -> Result<()> {
    require!(
        new_threshold >= 1 && new_threshold <= 100,
        StableGuardError::InvalidThreshold
    );

    let vault = &mut ctx.accounts.vault;
    let old_threshold = vault.rebalance_threshold;
    vault.rebalance_threshold = new_threshold;

    let timestamp = Clock::get()?.unix_timestamp;
    emit!(ThresholdUpdated {
        vault: vault.key(),
        old_threshold,
        new_threshold,
        timestamp,
    });

    msg!(
        "Threshold updated: {} → {}",
        old_threshold,
        new_threshold
    );
    Ok(())
}
