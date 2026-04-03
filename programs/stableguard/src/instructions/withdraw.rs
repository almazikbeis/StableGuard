use anchor_lang::prelude::*;
use anchor_spl::token::{self, Token, TokenAccount, Transfer};

use crate::errors::StableGuardError;
use crate::state::user_position::UserPosition;
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
    pub vault: Box<Account<'info, VaultState>>,

    /// User's destination token account
    #[account(mut)]
    pub user_token_account: Box<Account<'info, TokenAccount>>,

    /// Vault's token account for this token_index (validated in handler)
    #[account(mut)]
    pub vault_token_account: Box<Account<'info, TokenAccount>>,

    #[account(
        mut,
        seeds = [b"user_position", vault.key().as_ref(), user.key().as_ref()],
        bump = user_position.bump,
        has_one = owner @ StableGuardError::Unauthorized,
        constraint = user_position.vault == vault.key() @ StableGuardError::Unauthorized
    )]
    pub user_position: Box<Account<'info, UserPosition>>,

    /// CHECK: validated via `has_one = owner` on user_position
    pub owner: UncheckedAccount<'info>,

    pub token_program: Program<'info, Token>,
}

pub fn handle_withdraw(ctx: Context<Withdraw>, token_index: u8, amount: u64) -> Result<()> {
    // Withdraw always works — no pause check (users must be able to exit)
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
        ctx.accounts.owner.key() == ctx.accounts.user.key(),
        StableGuardError::Unauthorized
    );
    require!(
        ctx.accounts.user_token_account.owner == ctx.accounts.user.key(),
        StableGuardError::Unauthorized
    );
    require!(
        ctx.accounts.user_token_account.mint == ctx.accounts.vault_token_account.mint,
        StableGuardError::InvalidWithdrawAmount
    );
    require!(
        ctx.accounts.vault.balances[token_index as usize] >= amount,
        StableGuardError::InsufficientFunds
    );
    require!(
        ctx.accounts.user_position.epoch == ctx.accounts.vault.position_epoch,
        StableGuardError::VaultNotInitialized
    );
    require!(
        ctx.accounts.user_position.balances[token_index as usize] >= amount,
        StableGuardError::InsufficientBalance
    );

    let authority_key = ctx.accounts.vault.authority;
    let bump = ctx.accounts.vault.bump;

    let vault = &mut ctx.accounts.vault;
    let user_position = &mut ctx.accounts.user_position;

    vault.total_deposited = vault
        .total_deposited
        .checked_sub(amount)
        .ok_or(StableGuardError::MathOverflow)?;
    vault.balances[token_index as usize] = vault.balances[token_index as usize]
        .checked_sub(amount)
        .ok_or(StableGuardError::MathOverflow)?;
    user_position.balances[token_index as usize] = user_position.balances[token_index as usize]
        .checked_sub(amount)
        .ok_or(StableGuardError::MathOverflow)?;

    let seeds: &[&[u8]] = &[b"vault", authority_key.as_ref(), &[bump]];
    let signer = &[seeds];

    let cpi_ctx = CpiContext::new_with_signer(
        ctx.accounts.token_program.to_account_info(),
        Transfer {
            from: ctx.accounts.vault_token_account.to_account_info(),
            to: ctx.accounts.user_token_account.to_account_info(),
            authority: ctx.accounts.vault.to_account_info(),
        },
        signer,
    );
    token::transfer(cpi_ctx, amount)?;

    msg!(
        "Withdrew {} tokens (token_index={}) from vault",
        amount,
        token_index
    );
    Ok(())
}
