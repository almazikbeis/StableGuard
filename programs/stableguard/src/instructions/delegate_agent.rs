use anchor_lang::prelude::*;

use crate::errors::StableGuardError;
use crate::state::vault::VaultState;

#[event]
pub struct AgentDelegated {
    pub vault: Pubkey,
    pub agent: Pubkey,
    pub timestamp: i64,
}

#[derive(Accounts)]
pub struct DelegateAgent<'info> {
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

/// Sets the delegated agent pubkey on the vault.
/// Only the vault authority can delegate; the agent can then call
/// execute_rebalance and update_price_and_check without being the authority.
pub fn handle_delegate_agent(ctx: Context<DelegateAgent>, agent_pubkey: Pubkey) -> Result<()> {
    let vault_key = ctx.accounts.vault.key();
    let vault = &mut ctx.accounts.vault;

    vault.delegated_agent = agent_pubkey;

    let timestamp = Clock::get()?.unix_timestamp;

    emit!(AgentDelegated {
        vault: vault_key,
        agent: agent_pubkey,
        timestamp,
    });

    msg!("Agent delegated: {} on vault {}", agent_pubkey, vault_key);
    Ok(())
}
