use anchor_lang::prelude::*;

use crate::errors::StableGuardError;
use crate::state::vault::VaultState;

#[event]
pub struct CircuitBreakerTripped {
    pub vault: Pubkey,
    pub price: u64,
    pub threshold: u64,
    pub timestamp: i64,
}

#[event]
pub struct PriceUpdated {
    pub vault: Pubkey,
    pub price: u64,
    pub timestamp: i64,
}

#[derive(Accounts)]
pub struct UpdatePriceAndCheck<'info> {
    #[account(mut)]
    pub signer: Signer<'info>,

    #[account(
        mut,
        seeds = [b"vault", vault.authority.as_ref()],
        bump = vault.bump,
    )]
    pub vault: Account<'info, VaultState>,
}

/// Hot-path instruction: records the latest price and auto-pauses if below threshold.
/// Can be called ~400ms by the delegated AI agent without authority signature.
/// If circuit_breaker_threshold == 0, no auto-pause logic fires.
pub fn handle_update_price_and_check(ctx: Context<UpdatePriceAndCheck>, price: u64) -> Result<()> {
    require!(
        ctx.accounts
            .vault
            .is_authorized_agent(&ctx.accounts.signer.key()),
        StableGuardError::UnauthorizedAgent
    );

    let vault_key = ctx.accounts.vault.key();
    let vault = &mut ctx.accounts.vault;

    vault.last_price = price;

    let timestamp = Clock::get()?.unix_timestamp;

    if vault.circuit_breaker_threshold > 0 && price < vault.circuit_breaker_threshold {
        vault.is_paused = true;
        emit!(CircuitBreakerTripped {
            vault: vault_key,
            price,
            threshold: vault.circuit_breaker_threshold,
            timestamp,
        });
        msg!(
            "Circuit breaker tripped: price={} < threshold={} — vault paused",
            price,
            vault.circuit_breaker_threshold
        );
    } else {
        emit!(PriceUpdated {
            vault: vault_key,
            price,
            timestamp,
        });
    }

    Ok(())
}
