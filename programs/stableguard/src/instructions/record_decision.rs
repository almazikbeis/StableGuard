use anchor_lang::prelude::*;

use crate::state::decision::DecisionLog;
use crate::state::vault::VaultState;

#[derive(AnchorSerialize, AnchorDeserialize)]
pub struct RecordDecisionParams {
    pub action: u8,
    pub rationale: String,
    pub confidence_score: u8,
}

#[derive(Accounts)]
#[instruction(params: RecordDecisionParams)]
pub struct RecordDecision<'info> {
    #[account(mut)]
    pub authority: Signer<'info>,

    #[account(
        seeds = [b"vault", authority.key().as_ref()],
        bump = vault.bump,
        has_one = authority
    )]
    pub vault: Account<'info, VaultState>,

    #[account(
        init,
        payer = authority,
        space = DecisionLog::LEN,
        seeds = [b"decision", vault.key().as_ref(), &vault.decision_count.to_le_bytes()],
        bump
    )]
    pub decision_log: Account<'info, DecisionLog>,

    pub system_program: Program<'info, System>,
}

pub fn handle_record_decision(
    ctx: Context<RecordDecision>,
    params: RecordDecisionParams,
) -> Result<()> {
    let vault = &mut ctx.accounts.vault;
    let decision = &mut ctx.accounts.decision_log;

    decision.vault = vault.key();
    decision.sequence = vault.decision_count;
    decision.action = params.action;
    decision.rationale = params.rationale;
    decision.confidence_score = params.confidence_score;
    decision.timestamp = Clock::get()?.unix_timestamp;
    decision.bump = ctx.bumps.decision_log;

    vault.decision_count = vault.decision_count.checked_add(1).unwrap();

    msg!("Decision recorded: seq={}", decision.sequence);
    Ok(())
}
