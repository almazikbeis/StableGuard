use anchor_lang::prelude::*;

use crate::errors::StableGuardError;
use crate::state::vault::VaultState;

/// Demo-only: lets the vault authority directly write accounting balances
/// without performing real SPL transfers. Used on devnet to set up a realistic
/// vault state for AI demo flows (rebalance, protect, optimize).
///
/// This instruction is intentionally limited to devnet by convention — the
/// authority would never call it on mainnet because it grants no real tokens.
#[derive(Accounts)]
pub struct SetDemoBalances<'info> {
    #[account(mut)]
    pub authority: Signer<'info>,

    #[account(
        mut,
        seeds = [b"vault", authority.key().as_ref()],
        bump = vault.bump,
        has_one = authority @ StableGuardError::Unauthorized,
    )]
    pub vault: Account<'info, VaultState>,
}

pub fn handle_set_demo_balances(
    ctx: Context<SetDemoBalances>,
    balances: Vec<u64>,
    total_deposited: u64,
) -> Result<()> {
    require!(
        balances.len() <= crate::state::vault::MAX_TOKENS,
        StableGuardError::InvalidTokenIndex
    );

    let vault = &mut ctx.accounts.vault;

    for (i, &bal) in balances.iter().enumerate() {
        vault.balances[i] = bal;
    }

    // Update num_tokens to match however many slots are provided with non-zero balance
    let active = balances.iter().filter(|&&b| b > 0).count();
    if active > vault.num_tokens as usize {
        vault.num_tokens = active as u8;
    }

    vault.total_deposited = total_deposited;

    msg!(
        "DemoBalancesSet: slots={} total_deposited={}",
        balances.len(),
        total_deposited,
    );
    Ok(())
}
