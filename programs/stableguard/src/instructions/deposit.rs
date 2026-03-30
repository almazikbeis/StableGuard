use anchor_lang::prelude::*;
use anchor_spl::token::{self, Token, TokenAccount, Transfer};

use crate::errors::StableGuardError;
use crate::state::vault::VaultState;

#[derive(Accounts)]
pub struct Deposit<'info> {
    #[account(mut)]
    pub user: Signer<'info>,

    #[account(
        mut,
        seeds = [b"vault", vault.authority.as_ref()],
        bump = vault.bump
    )]
    pub vault: Account<'info, VaultState>,

    /// User's source token account (must match whichever token they're depositing)
    #[account(mut)]
    pub user_token_account: Account<'info, TokenAccount>,

    /// Vault's token_a account
    #[account(
        mut,
        address = vault.vault_token_a
    )]
    pub vault_token_a: Account<'info, TokenAccount>,

    /// Vault's token_b account
    #[account(
        mut,
        address = vault.vault_token_b
    )]
    pub vault_token_b: Account<'info, TokenAccount>,

    pub token_program: Program<'info, Token>,
}

pub fn handle_deposit(ctx: Context<Deposit>, amount: u64, is_token_a: bool) -> Result<()> {
    require!(amount > 0, StableGuardError::InvalidDepositAmount);
    require!(!ctx.accounts.vault.is_paused, StableGuardError::VaultPaused);

    let vault = &mut ctx.accounts.vault;

    // Validate user's token account mint matches the chosen side
    let expected_mint = if is_token_a { vault.mint_a } else { vault.mint_b };
    require!(
        ctx.accounts.user_token_account.mint == expected_mint,
        StableGuardError::InvalidDepositAmount
    );

    vault.total_deposited = vault
        .total_deposited
        .checked_add(amount)
        .ok_or(StableGuardError::MathOverflow)?;

    if is_token_a {
        vault.balance_a = vault.balance_a.checked_add(amount).ok_or(StableGuardError::MathOverflow)?;
    } else {
        vault.balance_b = vault.balance_b.checked_add(amount).ok_or(StableGuardError::MathOverflow)?;
    }

    let destination = if is_token_a {
        ctx.accounts.vault_token_a.to_account_info()
    } else {
        ctx.accounts.vault_token_b.to_account_info()
    };

    let cpi_ctx = CpiContext::new(
        ctx.accounts.token_program.to_account_info(),
        Transfer {
            from: ctx.accounts.user_token_account.to_account_info(),
            to: destination,
            authority: ctx.accounts.user.to_account_info(),
        },
    );
    token::transfer(cpi_ctx, amount)?;

    msg!("Deposited {} tokens (is_token_a={}) into vault", amount, is_token_a);
    Ok(())
}
