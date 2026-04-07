use anchor_lang::prelude::*;
use anchor_spl::token::{self, Token, TokenAccount, Transfer};

use crate::errors::StableGuardError;
use crate::state::vault::VaultState;

#[derive(Accounts)]
pub struct Deposit<'info> {
    #[account(mut)]
    pub authority: Signer<'info>,

    #[account(
        mut,
        seeds = [b"vault", authority.key().as_ref()],
        bump = vault.bump,
        has_one = authority @ StableGuardError::Unauthorized
    )]
    pub vault: Box<Account<'info, VaultState>>,

    /// Authority-owned source token account that funds the treasury.
    #[account(mut)]
    pub authority_token_account: Box<Account<'info, TokenAccount>>,

    /// Vault's token account for this token_index (validated in handler)
    #[account(mut)]
    pub vault_token_account: Box<Account<'info, TokenAccount>>,

    pub token_program: Program<'info, Token>,
}

pub fn handle_deposit(ctx: Context<Deposit>, token_index: u8, amount: u64) -> Result<()> {
    require!(amount > 0, StableGuardError::InvalidDepositAmount);
    require!(!ctx.accounts.vault.is_paused, StableGuardError::VaultPaused);
    require!(
        (token_index as usize) < ctx.accounts.vault.num_tokens as usize,
        StableGuardError::InvalidTokenIndex
    );
    // Validate the vault token account matches the registered slot
    require!(
        ctx.accounts.vault_token_account.key()
            == ctx.accounts.vault.vault_tokens[token_index as usize],
        StableGuardError::InvalidDepositAmount
    );
    require!(
        ctx.accounts.authority_token_account.owner == ctx.accounts.authority.key(),
        StableGuardError::Unauthorized
    );
    require!(
        ctx.accounts.authority_token_account.mint == ctx.accounts.vault_token_account.mint,
        StableGuardError::InvalidDepositAmount
    );

    let vault = &mut ctx.accounts.vault;

    vault.total_deposited = vault
        .total_deposited
        .checked_add(amount)
        .ok_or(StableGuardError::MathOverflow)?;
    vault.balances[token_index as usize] = vault.balances[token_index as usize]
        .checked_add(amount)
        .ok_or(StableGuardError::MathOverflow)?;

    let cpi_ctx = CpiContext::new(
        ctx.accounts.token_program.to_account_info(),
        Transfer {
            from: ctx.accounts.authority_token_account.to_account_info(),
            to: ctx.accounts.vault_token_account.to_account_info(),
            authority: ctx.accounts.authority.to_account_info(),
        },
    );
    token::transfer(cpi_ctx, amount)?;

    msg!(
        "Treasury deposit: {} tokens (token_index={}) into vault",
        amount,
        token_index
    );
    Ok(())
}
