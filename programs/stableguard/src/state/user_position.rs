use anchor_lang::prelude::*;

use crate::state::vault::MAX_TOKENS;

#[account]
pub struct UserPosition {
    /// User who owns this position.
    pub owner: Pubkey,
    /// Vault this position belongs to.
    pub vault: Pubkey,
    /// Deposited balances per registered token slot.
    pub balances: [u64; MAX_TOKENS],
    /// Monotonic vault epoch. Incremented on emergency withdraws to invalidate old balances.
    pub epoch: u64,
    /// PDA bump.
    pub bump: u8,
}

impl UserPosition {
    pub const LEN: usize = 8  // discriminator
        + 32                  // owner
        + 32                  // vault
        + 8 * MAX_TOKENS      // balances
        + 8                   // epoch
        + 1; // bump
}
