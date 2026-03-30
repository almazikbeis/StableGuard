use anchor_lang::prelude::*;

use crate::errors::StableGuardError;
use crate::state::vault::VaultState;

#[derive(Accounts)]
pub struct TogglePause<'info> {
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

pub fn handle_toggle_pause(ctx: Context<TogglePause>) -> Result<()> {
    let vault = &mut ctx.accounts.vault;
    vault.is_paused = !vault.is_paused;

    msg!("Vault is_paused toggled to: {}", vault.is_paused);
    Ok(())
}
