use anchor_lang::prelude::*;

/// Represents a single AI-agent decision logged on-chain
#[account]
pub struct DecisionLog {
    /// Vault this decision belongs to
    pub vault: Pubkey,
    /// Monotonically increasing sequence number
    pub sequence: u64,
    /// Encoded action (e.g. 0=hold, 1=rebalance, 2=withdraw)
    pub action: u8,
    /// Human-readable rationale from the AI agent (max 200 chars)
    pub rationale: String,
    /// Agent confidence score 0–100
    pub confidence_score: u8,
    /// Unix timestamp of when the decision was recorded
    pub timestamp: i64,
    /// PDA bump
    pub bump: u8,
}

impl DecisionLog {
    pub const MAX_RATIONALE_LEN: usize = 200;

    pub const LEN: usize = 8              // discriminator
        + 32                              // vault
        + 8                               // sequence
        + 1                               // action
        + 4 + Self::MAX_RATIONALE_LEN     // rationale (string prefix + bytes)
        + 1                               // confidence_score
        + 8                               // timestamp
        + 1;                              // bump
}
