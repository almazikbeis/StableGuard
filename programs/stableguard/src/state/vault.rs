use anchor_lang::prelude::*;

pub const MAX_TOKENS: usize = 8;

#[account]
pub struct VaultState {
    /// Authority who controls this vault
    pub authority: Pubkey,
    /// Token mints (up to MAX_TOKENS); unused slots are Pubkey::default()
    pub mints: [Pubkey; 8],
    /// Vault token accounts (up to MAX_TOKENS); unused slots are Pubkey::default()
    pub vault_tokens: [Pubkey; 8],
    /// Total tokens deposited across all registered tokens (in smallest unit)
    pub total_deposited: u64,
    /// Accounted balances per token slot. These change only when real SPL
    /// transfers enter or leave the vault.
    pub balances: [u64; 8],
    /// Threshold (1–100) at which rebalancing is triggered
    pub rebalance_threshold: u64,
    /// Maximum allowed deposit per transaction
    pub max_deposit: u64,
    /// Number of recorded AI decisions
    pub decision_count: u64,
    /// Number of rebalances executed
    pub total_rebalances: u64,
    /// How many token slots are active (registered)
    pub num_tokens: u8,
    /// Emergency pause flag
    pub is_paused: bool,
    /// Strategy mode: 0 = Safe, 1 = Yield
    pub strategy_mode: u8,
    /// PDA bump
    pub bump: u8,
    /// Delegated agent allowed to call rebalance/update_price without being authority
    pub delegated_agent: Pubkey,
    /// On-chain circuit breaker threshold (e.g. 998000 = $0.998 with 6 decimals); 0 = disabled
    pub circuit_breaker_threshold: u64,
    /// Last price pushed by the hot path (in 6-decimal fixed point)
    pub last_price: u64,
    /// Monotonic epoch for invalidating stale user positions after emergency exits.
    pub position_epoch: u64,
}

impl VaultState {
    pub const LEN: usize = 8       // discriminator
        + 32                        // authority
        + 32 * 8                    // mints
        + 32 * 8                    // vault_tokens
        + 8                         // total_deposited
        + 8 * 8                     // balances
        + 8                         // rebalance_threshold
        + 8                         // max_deposit
        + 8                         // decision_count
        + 8                         // total_rebalances
        + 1                         // num_tokens
        + 1                         // is_paused
        + 1                         // strategy_mode
        + 1                         // bump
        + 32                        // delegated_agent
        + 8                         // circuit_breaker_threshold
        + 8                         // last_price
        + 8; // position_epoch
             // Total: 716 bytes

    pub fn get_total_value(&self) -> u64 {
        self.balances[..self.num_tokens as usize]
            .iter()
            .fold(0u64, |acc, &b| acc.saturating_add(b))
    }

    /// Returns allocation percentages for each registered token (0–100).
    /// Remaining slots (beyond num_tokens) are 0.
    pub fn get_allocation_pct(&self) -> [u8; MAX_TOKENS] {
        let total = self.get_total_value();
        let mut pcts = [0u8; MAX_TOKENS];
        if total == 0 {
            return pcts;
        }
        let n = self.num_tokens as usize;
        let mut assigned: u16 = 0;
        for i in 0..n.saturating_sub(1) {
            let p = ((self.balances[i] as u128 * 100) / total as u128) as u8;
            pcts[i] = p;
            assigned += p as u16;
        }
        if n > 0 {
            pcts[n - 1] = (100u16.saturating_sub(assigned)) as u8;
        }
        pcts
    }

    pub fn strategy_name(&self) -> &str {
        match self.strategy_mode {
            0 => "safe",
            1 => "yield",
            _ => "unknown",
        }
    }

    /// Returns true if the given key is either the vault authority or the delegated agent.
    pub fn is_authorized_agent(&self, key: &Pubkey) -> bool {
        key == &self.authority || key == &self.delegated_agent
    }
}
