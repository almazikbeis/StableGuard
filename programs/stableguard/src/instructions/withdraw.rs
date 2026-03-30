use anchor_lang::prelude::*;
use anchor_spl::token::{self, Token, TokenAccount, Transfer};

use crate::errors::StableGuardError;
use crate::state::vault::VaultState;

#[derive(Accounts)]
pub struct Withdraw<'info> {
    #[account(mut)]
    pub user: Signer<'info>,

    #[account(
        mut,
        seeds = [b"vault", vault.authority.as_ref()],
        bump = vault.bump
    )]
    pub vault: Account<'info, VaultState>,

    /// User's destination token account
    #[account(mut)]
    pub user_token_account: Account<'info, TokenAccount>,

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

    pub token_program: Program<'info, Token>,
}

pub fn handle_withdraw(ctx: Context<Withdraw>, amount: u64, is_token_a: bool) -> Result<()> {
    // Withdraw always works — no pause check (users must be able to exit)
    require!(amount > 0, StableGuardError::InvalidWithdrawAmount);
    require!(
        ctx.accounts.vault.total_deposited >= amount,
        StableGuardError::InsufficientFunds
    );

    let vault = &mut ctx.accounts.vault;

    // Validate user's token account mint
    let expected_mint = if is_token_a { vault.mint_a } else { vault.mint_b };
    require!(
        ctx.accounts.user_token_account.mint == expected_mint,
        StableGuardError::InvalidWithdrawAmount
    );

    let authority_key = vault.authority;
    let bump = vault.bump;

    vault.total_deposited = vault
        .total_deposited
        .checked_sub(amount)
        .ok_or(StableGuardError::MathOverflow)?;

    if is_token_a {
        vault.balance_a = vault.balance_a.checked_sub(amount).ok_or(StableGuardError::MathOverflow)?;
    } else {
        vault.balance_b = vault.balance_b.checked_sub(amount).ok_or(StableGuardError::MathOverflow)?;
    }

    let seeds: &[&[u8]] = &[b"vault", authority_key.as_ref(), &[bump]];
    let signer = &[seeds];

    let source = if is_token_a {
        ctx.accounts.vault_token_a.to_account_info()
    } else {
        ctx.accounts.vault_token_b.to_account_info()
    };

    let cpi_ctx = CpiContext::new_with_signer(
        ctx.accounts.token_program.to_account_info(),
        Transfer {
            from: source,
            to: ctx.accounts.user_token_account.to_account_info(),
            authority: ctx.accounts.vault.to_account_info(),
        },
        signer,
    );
    token::transfer(cpi_ctx, amount)?;

    msg!("Withdrew {} tokens (is_token_a={}) from vault", amount, is_token_a);
    Ok(())
}
