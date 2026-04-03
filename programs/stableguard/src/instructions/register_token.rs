use anchor_lang::prelude::*;
use anchor_spl::token::{Mint, Token, TokenAccount};

use crate::errors::StableGuardError;
use crate::state::vault::{VaultState, MAX_TOKENS};

#[event]
pub struct TokenRegistered {
    pub vault: Pubkey,
    pub mint: Pubkey,
    pub token_index: u8,
    pub timestamp: i64,
}

#[derive(Accounts)]
#[instruction(token_index: u8)]
pub struct RegisterToken<'info> {
    #[account(mut)]
    pub authority: Signer<'info>,

    #[account(
        mut,
        seeds = [b"vault", authority.key().as_ref()],
        bump = vault.bump,
        has_one = authority @ StableGuardError::Unauthorized
    )]
    pub vault: Account<'info, VaultState>,

    pub mint: Account<'info, Mint>,

    #[account(
        init,
        payer = authority,
        token::mint = mint,
        token::authority = vault,
        seeds = [b"vault_token", vault.key().as_ref(), &[token_index]],
        bump
    )]
    pub vault_token: Account<'info, TokenAccount>,

    pub token_program: Program<'info, Token>,
    pub system_program: Program<'info, System>,
    pub rent: Sysvar<'info, Rent>,
}

pub fn handle_register_token(ctx: Context<RegisterToken>, token_index: u8) -> Result<()> {
    require!(
        (token_index as usize) < MAX_TOKENS,
        StableGuardError::InvalidTokenIndex
    );
    require!(
        ctx.accounts.vault.mints[token_index as usize] == Pubkey::default(),
        StableGuardError::TokenSlotOccupied
    );
    require!(
        ctx.accounts.vault.num_tokens < MAX_TOKENS as u8,
        StableGuardError::MaxTokensReached
    );

    let vault = &mut ctx.accounts.vault;
    vault.mints[token_index as usize] = ctx.accounts.mint.key();
    vault.vault_tokens[token_index as usize] = ctx.accounts.vault_token.key();
    vault.num_tokens += 1;

    let vault_key = vault.key();
    let mint_key = ctx.accounts.mint.key();
    let timestamp = Clock::get()?.unix_timestamp;

    emit!(TokenRegistered {
        vault: vault_key,
        mint: mint_key,
        token_index,
        timestamp,
    });

    msg!("Token registered: index={} mint={}", token_index, mint_key);
    Ok(())
}
