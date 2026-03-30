use anchor_lang::prelude::*;
use anchor_spl::token::{self, Token, TokenAccount, Transfer};

use crate::errors::StableGuardError;
use crate::state::vault::VaultState;

#[event]
pub struct EmergencyWithdrawEvent {
    pub vault: Pubkey,
    pub amount_a: u64,
    pub amount_b: u64,
    pub timestamp: i64,
}

#[derive(Accounts)]
pub struct EmergencyWithdraw<'info> {
    #[account(mut)]
    pub authority: Signer<'info>,

    #[account(
        mut,
        seeds = [b"vault", authority.key().as_ref()],
        bump = vault.bump,
        has_one = authority @ StableGuardError::Unauthorized
    )]
    pub vault: Account<'info, VaultState>,

    #[account(
        mut,
        address = vault.vault_token_a
    )]
    pub vault_token_a: Account<'info, TokenAccount>,

    #[account(
        mut,
        address = vault.vault_token_b
    )]
    pub vault_token_b: Account<'info, TokenAccount>,

    /// Authority's token A account (receives all token A)
    #[account(mut)]
    pub authority_token_a: Account<'info, TokenAccount>,

    /// Authority's token B account (receives all token B)
    #[account(mut)]
    pub authority_token_b: Account<'info, TokenAccount>,

    pub token_program: Program<'info, Token>,
}

/// Emergency withdraw drains ALL vault tokens to authority.
/// Works even when vault is paused — this is the emergency exit.
pub fn handle_emergency_withdraw(ctx: Context<EmergencyWithdraw>) -> Result<()> {
    // Collect values before mutable borrow
    let authority_key = ctx.accounts.vault.authority;
    let bump = ctx.accounts.vault.bump;
    let vault_key = ctx.accounts.vault.key();

    // Use actual SPL balances (not virtual) — they may differ due to virtual rebalances.
    // Emergency withdraw drains everything physically in the vault.
    let amount_a = ctx.accounts.vault_token_a.amount;
    let amount_b = ctx.accounts.vault_token_b.amount;

    let vault_token_a_info = ctx.accounts.vault_token_a.to_account_info();
    let vault_token_b_info = ctx.accounts.vault_token_b.to_account_info();
    let auth_token_a_info = ctx.accounts.authority_token_a.to_account_info();
    let auth_token_b_info = ctx.accounts.authority_token_b.to_account_info();
    let token_prog_info = ctx.accounts.token_program.to_account_info();
    let vault_info = ctx.accounts.vault.to_account_info();

    let seeds: &[&[u8]] = &[b"vault", authority_key.as_ref(), &[bump]];
    let signer = &[seeds];

    // Transfer all of token A
    if amount_a > 0 {
        let cpi_ctx = CpiContext::new_with_signer(
            token_prog_info.clone(),
            Transfer {
                from: vault_token_a_info,
                to: auth_token_a_info,
                authority: vault_info.clone(),
            },
            signer,
        );
        token::transfer(cpi_ctx, amount_a)?;
    }

    // Transfer all of token B
    if amount_b > 0 {
        let cpi_ctx = CpiContext::new_with_signer(
            token_prog_info,
            Transfer {
                from: vault_token_b_info,
                to: auth_token_b_info,
                authority: vault_info,
            },
            signer,
        );
        token::transfer(cpi_ctx, amount_b)?;
    }

    // Zero out vault balances
    let vault = &mut ctx.accounts.vault;
    vault.balance_a = 0;
    vault.balance_b = 0;
    vault.total_deposited = 0;

    let timestamp = Clock::get()?.unix_timestamp;
    emit!(EmergencyWithdrawEvent {
        vault: vault_key,
        amount_a,
        amount_b,
        timestamp,
    });

    msg!(
        "Emergency withdraw: {} token_a + {} token_b drained to authority",
        amount_a,
        amount_b
    );
    Ok(())
}
