use anchor_lang::prelude::*;
use anchor_spl::token::{self, Token, TokenAccount, Transfer};

use crate::errors::StableGuardError;
use crate::state::vault::VaultState;

#[event]
pub struct PaymentSent {
    pub vault: Pubkey,
    pub recipient: Pubkey,
    pub token_index: u8,
    pub amount: u64,
    pub timestamp: i64,
}

#[derive(Accounts)]
pub struct SendPayment<'info> {
    #[account(mut)]
    pub authority: Signer<'info>,

    #[account(
        mut,
        seeds = [b"vault", authority.key().as_ref()],
        bump = vault.bump,
        has_one = authority @ StableGuardError::Unauthorized
    )]
    pub vault: Account<'info, VaultState>,

    /// Vault token account to send from (validated in handler against token_index)
    #[account(mut)]
    pub vault_token_account: Account<'info, TokenAccount>,

    /// Recipient's token account (must match same mint)
    #[account(mut)]
    pub recipient_token_account: Account<'info, TokenAccount>,

    pub token_program: Program<'info, Token>,
}

pub fn handle_send_payment(
    ctx: Context<SendPayment>,
    token_index: u8,
    amount: u64,
) -> Result<()> {
    require!(!ctx.accounts.vault.is_paused, StableGuardError::VaultPaused);
    require!(amount > 0, StableGuardError::InvalidDepositAmount);
    require!(
        (token_index as usize) < ctx.accounts.vault.num_tokens as usize,
        StableGuardError::InvalidTokenIndex
    );

    // Validate vault_token_account matches the token_index slot
    let expected_vault_token = ctx.accounts.vault.vault_tokens[token_index as usize];
    require!(
        ctx.accounts.vault_token_account.key() == expected_vault_token,
        StableGuardError::Unauthorized
    );

    let authority_key = ctx.accounts.vault.authority;
    let bump          = ctx.accounts.vault.bump;
    let vault_key     = ctx.accounts.vault.key();
    let recipient_key = ctx.accounts.recipient_token_account.key();

    {
        let vault = &mut ctx.accounts.vault;
        require!(
            vault.balances[token_index as usize] >= amount,
            StableGuardError::InsufficientBalance
        );
        vault.balances[token_index as usize] = vault.balances[token_index as usize]
            .checked_sub(amount)
            .ok_or(StableGuardError::MathOverflow)?;
        vault.total_deposited = vault.total_deposited.saturating_sub(amount);
    }

    let seeds: &[&[u8]] = &[b"vault", authority_key.as_ref(), &[bump]];
    let signer = &[seeds];

    let cpi_ctx = CpiContext::new_with_signer(
        ctx.accounts.token_program.to_account_info(),
        Transfer {
            from:      ctx.accounts.vault_token_account.to_account_info(),
            to:        ctx.accounts.recipient_token_account.to_account_info(),
            authority: ctx.accounts.vault.to_account_info(),
        },
        signer,
    );
    token::transfer(cpi_ctx, amount)?;

    let timestamp = Clock::get()?.unix_timestamp;
    emit!(PaymentSent {
        vault: vault_key,
        recipient: recipient_key,
        token_index,
        amount,
        timestamp,
    });

    msg!(
        "Payment sent: {} tokens (token_index={}) to {}",
        amount,
        token_index,
        recipient_key
    );
    Ok(())
}
