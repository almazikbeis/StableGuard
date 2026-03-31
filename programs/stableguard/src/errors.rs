use anchor_lang::prelude::*;

#[error_code]
pub enum StableGuardError {
    #[msg("Insufficient funds in vault")]
    InsufficientFunds,

    #[msg("Deposit amount must be greater than zero")]
    InvalidDepositAmount,

    #[msg("Withdrawal amount must be greater than zero")]
    InvalidWithdrawAmount,

    #[msg("Vault is not initialized")]
    VaultNotInitialized,

    #[msg("Unauthorized rebalance attempt")]
    UnauthorizedRebalance,

    #[msg("Rebalance threshold not reached")]
    RebalanceThresholdNotReached,

    #[msg("Math overflow")]
    MathOverflow,

    #[msg("Vault is paused — deposits and rebalances are disabled")]
    VaultPaused,

    #[msg("Invalid rebalance direction — from and to indices must differ")]
    InvalidDirection,

    #[msg("Rebalance amount must be greater than zero")]
    InvalidRebalanceAmount,

    #[msg("Invalid strategy mode — must be 0 (safe) or 1 (yield)")]
    InvalidStrategy,

    #[msg("Invalid threshold — must be between 1 and 100")]
    InvalidThreshold,

    #[msg("Insufficient token balance for this operation")]
    InsufficientBalance,

    #[msg("Caller is not the vault authority")]
    Unauthorized,

    #[msg("Token index out of bounds or token not registered")]
    InvalidTokenIndex,

    #[msg("Token slot already occupied — unregister first")]
    TokenSlotOccupied,

    #[msg("Maximum number of tokens (8) already reached")]
    MaxTokensReached,
}
