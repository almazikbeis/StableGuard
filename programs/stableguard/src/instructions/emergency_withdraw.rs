use anchor_lang::prelude::*;
use anchor_spl::token::{self, Token, Transfer};

use crate::errors::StableGuardError;
use crate::state::vault::VaultState;

#[event]
pub struct EmergencyWithdrawEvent {
    pub vault: Pubkey,
    pub total_drained: u64,
    pub num_tokens: u8,
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

    pub token_program: Program<'info, Token>,
}

/// Emergency withdraw drains ALL vault tokens to authority.
/// Works even when vault is paused — this is the emergency exit.
///
/// remaining_accounts layout (num_tokens = N):
///   [0..N)   — vault token accounts (in index order)
///   [N..2N)  — authority token accounts (same order)
pub fn handle_emergency_withdraw<'info>(
    ctx: Context<'_, '_, '_, 'info, EmergencyWithdraw<'info>>,
) -> Result<()> {
    let authority_key = ctx.accounts.vault.authority;
    let bump = ctx.accounts.vault.bump;
    let vault_key = ctx.accounts.vault.key();
    let num_tokens = ctx.accounts.vault.num_tokens as usize;

    require!(
        ctx.remaining_accounts.len() >= 2 * num_tokens,
        StableGuardError::InvalidTokenIndex
    );

    let seeds: &[&[u8]] = &[b"vault", authority_key.as_ref(), &[bump]];
    let signer = &[seeds];

    let token_prog_info = ctx.accounts.token_program.to_account_info();
    let vault_info = ctx.accounts.vault.to_account_info();

    let mut total_drained: u64 = 0;

    for i in 0..num_tokens {
        let vault_token_info = &ctx.remaining_accounts[i];
        let auth_token_info = &ctx.remaining_accounts[num_tokens + i];

        // Read on-chain SPL balance without taking ownership
        let amount = {
            let data = vault_token_info.try_borrow_data()?;
            // SPL Token account: amount field at byte offset 64..72
            if data.len() < 72 {
                return err!(StableGuardError::InvalidTokenIndex);
            }
            u64::from_le_bytes(data[64..72].try_into().unwrap())
        };

        if amount > 0 {
            let cpi_ctx = CpiContext::new_with_signer(
                token_prog_info.clone(),
                Transfer {
                    from: vault_token_info.clone(),
                    to: auth_token_info.clone(),
                    authority: vault_info.clone(),
                },
                signer,
            );
            token::transfer(cpi_ctx, amount)?;
            total_drained = total_drained.saturating_add(amount);
        }
    }

    // Zero out all balances
    let vault = &mut ctx.accounts.vault;
    for i in 0..num_tokens {
        vault.balances[i] = 0;
    }
    vault.total_deposited = 0;
    vault.position_epoch = vault
        .position_epoch
        .checked_add(1)
        .ok_or(StableGuardError::MathOverflow)?;

    let timestamp = Clock::get()?.unix_timestamp;
    emit!(EmergencyWithdrawEvent {
        vault: vault_key,
        total_drained,
        num_tokens: num_tokens as u8,
        timestamp,
    });

    msg!(
        "Emergency withdraw: {} total tokens drained across {} token types",
        total_drained,
        num_tokens
    );
    Ok(())
}
