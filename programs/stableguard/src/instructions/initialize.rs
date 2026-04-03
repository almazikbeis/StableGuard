use anchor_lang::prelude::*;

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

    pub system_program: Program<'info, System>,
}

pub fn handle_initialize_vault(
    ctx: Context<InitializeVault>,
    params: InitializeVaultParams,
) -> Result<()> {
    let vault = &mut ctx.accounts.vault;

    vault.authority = ctx.accounts.authority.key();
    vault.mints = [Pubkey::default(); 8];
    vault.vault_tokens = [Pubkey::default(); 8];
    vault.total_deposited = 0;
    vault.balances = [0u64; 8];
    vault.rebalance_threshold = params.rebalance_threshold;
    vault.max_deposit = params.max_deposit;
    vault.decision_count = 0;
    vault.total_rebalances = 0;
    vault.num_tokens = 0;
    vault.is_paused = false;
    vault.strategy_mode = 0; // default: Safe
    vault.bump = ctx.bumps.vault;
    vault.position_epoch = 0;

    msg!("VaultState initialized: {:?}", vault.key());
    Ok(())
}
