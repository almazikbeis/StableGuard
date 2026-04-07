use anchor_lang::prelude::*;
use anchor_spl::token::{self, Token, TokenAccount, Transfer};

use crate::errors::StableGuardError;
use crate::state::vault::VaultState;

#[derive(Accounts)]
pub struct Withdraw<'info> {
    #[account(mut)]
    pub authority: Signer<'info>,

    #[account(
        mut,
        seeds = [b"vault", authority.key().as_ref()],
        bump = vault.bump,
        has_one = authority @ StableGuardError::Unauthorized
    )]
    pub vault: Box<Account<'info, VaultState>>,

    /// Authority-controlled destination token account.
    #[account(mut)]
    pub authority_token_account: Box<Account<'info, TokenAccount>>,

    /// Vault's token account for this token_index (validated in handler)
    #[account(mut)]
    pub vault_token_account: Box<Account<'info, TokenAccount>>,

    pub token_program: Program<'info, Token>,
}

pub fn handle_withdraw(ctx: Context<Withdraw>, token_index: u8, amount: u64) -> Result<()> {
    // Treasury withdraw always works, even while paused, so operators can recover capital.
    require!(amount > 0, StableGuardError::InvalidWithdrawAmount);
    require!(
        (token_index as usize) < ctx.accounts.vault.num_tokens as usize,
        StableGuardError::InvalidTokenIndex
    );
    require!(
        ctx.accounts.vault_token_account.key()
            == ctx.accounts.vault.vault_tokens[token_index as usize],
        StableGuardError::InvalidWithdrawAmount
    );
    require!(
        ctx.accounts.authority_token_account.owner == ctx.accounts.authority.key(),
        StableGuardError::Unauthorized
    );
    require!(
        ctx.accounts.authority_token_account.mint == ctx.accounts.vault_token_account.mint,
        StableGuardError::InvalidWithdrawAmount
    );
    require!(
        ctx.accounts.vault.balances[token_index as usize] >= amount,
        StableGuardError::InsufficientFunds
    );

    let authority_key = ctx.accounts.vault.authority;
    let bump = ctx.accounts.vault.bump;

    let vault = &mut ctx.accounts.vault;

    vault.total_deposited = vault
        .total_deposited
        .checked_sub(amount)
        .ok_or(StableGuardError::MathOverflow)?;
    vault.balances[token_index as usize] = vault.balances[token_index as usize]
        .checked_sub(amount)
        .ok_or(StableGuardError::MathOverflow)?;

    let seeds: &[&[u8]] = &[b"vault", authority_key.as_ref(), &[bump]];
    let signer = &[seeds];

    let cpi_ctx = CpiContext::new_with_signer(
        ctx.accounts.token_program.to_account_info(),
        Transfer {
            from: ctx.accounts.vault_token_account.to_account_info(),
            to: ctx.accounts.authority_token_account.to_account_info(),
            authority: ctx.accounts.vault.to_account_info(),
        },
        signer,
    );
    token::transfer(cpi_ctx, amount)?;

    msg!(
        "Treasury withdraw: {} tokens (token_index={}) from vault",
        amount,
        token_index
    );
    Ok(())
}
