use anchor_lang::prelude::*;

#[account]
pub struct VaultState {
    /// Authority who controls this vault
    pub authority: Pubkey,
    /// Token A mint (e.g. USDC)
    pub mint_a: Pubkey,
    /// Token B mint (e.g. USDT)
    pub mint_b: Pubkey,
    /// Vault token account for mint_a
    pub vault_token_a: Pubkey,
    /// Vault token account for mint_b
    pub vault_token_b: Pubkey,
    /// Total tokens deposited across both (in smallest unit)
    pub total_deposited: u64,
    /// Virtual balance tracking for token A (updated on deposit/withdraw/rebalance)
    pub balance_a: u64,
    /// Virtual balance tracking for token B (updated on deposit/withdraw/rebalance)
    pub balance_b: u64,
    /// Threshold (1–100) at which rebalancing is triggered
    pub rebalance_threshold: u64,
    /// Maximum allowed deposit per transaction
    pub max_deposit: u64,
    /// Number of recorded AI decisions
    pub decision_count: u64,
    /// Number of rebalances executed
    pub total_rebalances: u64,
    /// Emergency pause flag
    pub is_paused: bool,
    /// Strategy mode: 0 = Safe, 1 = Yield
    pub strategy_mode: u8,
    /// PDA bump
    pub bump: u8,
}

impl VaultState {
    pub const LEN: usize = 8    // discriminator
        + 32   // authority
        + 32   // mint_a
        + 32   // mint_b
        + 32   // vault_token_a
        + 32   // vault_token_b
        + 8    // total_deposited
        + 8    // balance_a
        + 8    // balance_b
        + 8    // rebalance_threshold
        + 8    // max_deposit
        + 8    // decision_count
        + 8    // total_rebalances
        + 1    // is_paused
        + 1    // strategy_mode
        + 1;   // bump
    // Total: 227 bytes

    pub fn get_total_value(&self) -> u64 {
        self.balance_a.saturating_add(self.balance_b)
    }

    /// Returns (pct_a, pct_b) allocation percentages (0–100 each)
    pub fn get_allocation_pct(&self) -> (u8, u8) {
        let total = self.get_total_value();
        if total == 0 {
            return (0, 0);
        }
        let pct_a = ((self.balance_a as u128 * 100) / total as u128) as u8;
        let pct_b = 100u8.saturating_sub(pct_a);
        (pct_a, pct_b)
    }

    pub fn strategy_name(&self) -> &str {
        match self.strategy_mode {
            0 => "safe",
            1 => "yield",
            _ => "unknown",
        }
    }
}
