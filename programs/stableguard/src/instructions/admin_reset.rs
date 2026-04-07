use crate::state::vault::VaultState;
use anchor_lang::prelude::*;

/// AdminResetDecisionCount lets the vault authority manually set decision_count.
/// Used to recover when a PDA slot is occupied but the counter is out of sync.
#[derive(Accounts)]
pub struct AdminResetDecisionCount<'info> {
    #[account(mut)]
    pub authority: Signer<'info>,

    #[account(
        mut,
        seeds = [b"vault", authority.key().as_ref()],
        bump = vault.bump,
        has_one = authority
    )]
    pub vault: Account<'info, VaultState>,
}

pub fn handle_admin_reset_decision_count(
    ctx: Context<AdminResetDecisionCount>,
    new_count: u64,
) -> Result<()> {
    ctx.accounts.vault.decision_count = new_count;
    msg!("Decision count reset to: {}", new_count);
    Ok(())
}
