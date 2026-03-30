use anchor_lang::prelude::*;
use anchor_spl::token::{Mint, Token, TokenAccount};

use crate::state::vault::VaultState;

#[derive(AnchorSerialize, AnchorDeserialize)]
pub struct InitializeVaultParams {
    pub rebalance_threshold: u64,
    pub max_deposit: u64,
}

#[derive(Accounts)]
pub struct InitializeVault<'info> {
    #[account(mut)]
    pub authority: Signer<'info>,

    #[account(
        init,
        payer = authority,
        space = VaultState::LEN,
        seeds = [b"vault", authority.key().as_ref()],
        bump
    )]
    pub vault: Account<'info, VaultState>,

    pub mint_a: Account<'info, Mint>,
    pub mint_b: Account<'info, Mint>,

    #[account(
        init,
        payer = authority,
        token::mint = mint_a,
        token::authority = vault,
        seeds = [b"vault_token_a", vault.key().as_ref()],
        bump
    )]
    pub vault_token_a: Account<'info, TokenAccount>,

    #[account(
        init,
        payer = authority,
        token::mint = mint_b,
        token::authority = vault,
        seeds = [b"vault_token_b", vault.key().as_ref()],
        bump
    )]
    pub vault_token_b: Account<'info, TokenAccount>,

    pub token_program: Program<'info, Token>,
    pub system_program: Program<'info, System>,
    pub rent: Sysvar<'info, Rent>,
}

pub fn handle_initialize_vault(ctx: Context<InitializeVault>, params: InitializeVaultParams) -> Result<()> {
    let vault = &mut ctx.accounts.vault;

    vault.authority       = ctx.accounts.authority.key();
    vault.mint_a          = ctx.accounts.mint_a.key();
    vault.mint_b          = ctx.accounts.mint_b.key();
    vault.vault_token_a   = ctx.accounts.vault_token_a.key();
    vault.vault_token_b   = ctx.accounts.vault_token_b.key();
    vault.total_deposited = 0;
    vault.balance_a       = 0;
    vault.balance_b       = 0;
    vault.rebalance_threshold = params.rebalance_threshold;
    vault.max_deposit     = params.max_deposit;
    vault.decision_count  = 0;
    vault.total_rebalances = 0;
    vault.is_paused       = false;
    vault.strategy_mode   = 0; // default: Safe
    vault.bump            = ctx.bumps.vault;

    msg!("VaultState initialized: {:?}", vault.key());
    Ok(())
}
