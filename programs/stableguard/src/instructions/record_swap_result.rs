use anchor_lang::prelude::*;

use crate::errors::StableGuardError;
use crate::state::vault::VaultState;

#[event]
pub struct SwapSettled {
    pub vault: Pubkey,
    pub from_index: u8,
    pub to_index: u8,
    pub input_amount: u64,
    pub output_amount: u64,
    pub swap_signature: String,
    pub timestamp: i64,
}

#[derive(AnchorSerialize, AnchorDeserialize)]
pub struct RecordSwapResultParams {
    pub from_index: u8,
    pub to_index: u8,
    pub input_amount: u64,
    pub output_amount: u64,
    /// The base58-encoded signature of the external Jupiter swap transaction.
    pub swap_signature: String,
}

#[derive(Accounts)]
pub struct RecordSwapResult<'info> {
    #[account(mut)]
    pub signer: Signer<'info>,

    #[account(
        mut,
        seeds = [b"vault", vault.authority.as_ref()],
        bump = vault.bump,
    )]
    pub vault: Account<'info, VaultState>,
}

/// Records the settlement of a real external swap (e.g., Jupiter) for the vault.
/// Called after a successful swap TX is confirmed on-chain.
/// This is an immutable audit receipt only; vault balances should already reflect
/// the real source outflow and target inflow through send_payment + deposit.
/// Full trail: execute_rebalance (intent) → external swap TX → record_swap_result (receipt)
pub fn handle_record_swap_result(
    ctx: Context<RecordSwapResult>,
    params: RecordSwapResultParams,
) -> Result<()> {
    require!(
        ctx.accounts
            .vault
            .is_authorized_agent(&ctx.accounts.signer.key()),
        StableGuardError::UnauthorizedAgent
    );
    require!(
        params.from_index != params.to_index,
        StableGuardError::InvalidDirection
    );
    require!(
        (params.from_index as usize) < ctx.accounts.vault.num_tokens as usize,
        StableGuardError::InvalidTokenIndex
    );
    require!(
        (params.to_index as usize) < ctx.accounts.vault.num_tokens as usize,
        StableGuardError::InvalidTokenIndex
    );
    require!(
        params.input_amount > 0,
        StableGuardError::InvalidRebalanceAmount
    );
    require!(
        params.output_amount > 0,
        StableGuardError::InvalidRebalanceAmount
    );
    require!(
        params.swap_signature.len() <= 88,
        StableGuardError::InvalidDepositAmount
    );

    let vault_key = ctx.accounts.vault.key();
    let vault = &mut ctx.accounts.vault;

    vault.total_rebalances = vault
        .total_rebalances
        .checked_add(1)
        .ok_or(StableGuardError::MathOverflow)?;

    let timestamp = Clock::get()?.unix_timestamp;

    emit!(SwapSettled {
        vault: vault_key,
        from_index: params.from_index,
        to_index: params.to_index,
        input_amount: params.input_amount,
        output_amount: params.output_amount,
        swap_signature: params.swap_signature.clone(),
        timestamp,
    });

    msg!(
        "SwapSettled: from={} to={} in={} out={} swap_sig={}",
        params.from_index,
        params.to_index,
        params.input_amount,
        params.output_amount,
        params.swap_signature,
    );

    Ok(())
}
