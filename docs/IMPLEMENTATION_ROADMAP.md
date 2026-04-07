# StableGuard Implementation Roadmap

This roadmap reflects the intended production direction of the project:

1. Autonomous stablecoin treasury on Solana with a real policy engine.
2. Hot-path on-chain price update plus circuit breaker.
3. Cold-path AI risk and yield decisions.
4. Yield allocator across Kamino, Marginfi, and Drift.
5. Chat-first strategy control.
6. History, goals, alerts, and payments as a treasury operating system.
7. Live execution through a dedicated custody and execution path.

## Phase 1: Control Plane Hardening

Goal: make the current control plane safe enough to evolve.

- Separate authenticated users from authorized operators.
- Ensure only operator wallets or allowlisted operator emails can mutate vault state.
- Keep backend signing authority isolated from public login.
- Make the current `record_only` execution boundary explicit in API and UI.

Status:

- In progress.
- Operator-only middleware added for vault and settings routes.

Required environment:

- `OPERATOR_WALLETS`:
  comma-separated wallet allowlist for human operators.
- `OPERATOR_EMAILS`:
  comma-separated email allowlist for legacy operator login.

Notes:

- The backend signer wallet remains implicitly authorized.
- Generic wallet login is no longer enough for operator routes.

## Phase 2: Custody Model Redesign

Goal: remove the mismatch between virtual per-slot accounting and withdrawable liquidity.

Work items:

- Choose one custody model and implement it end-to-end:
  - single-owner treasury, or
  - pooled multi-user vault with share accounting.
- Replace slot-based user claims with coherent treasury share accounting if multi-user mode is kept.
- Revisit `execute_rebalance`, `withdraw`, and emergency semantics around real asset movement.
- Add explicit invariant tests for deposits, rebalances, and withdrawals.

Exit criteria:

- No user can end up with a valid claim that cannot be withdrawn due to virtual-only rebalances.

## Phase 3: Real Execution Path

Goal: move from `record_only` to actual protected execution.

Work items:

- Introduce a dedicated execution custody path instead of trying to route swaps directly from program-owned vault token accounts.
- Define trusted swap execution boundaries.
- Add transaction simulation, slippage controls, and fail-closed policy checks.
- Support operator approval mode before fully autonomous execution.

Exit criteria:

- The system can execute a real rebalance safely, not just record a decision.

## Phase 4: Yield Execution

Goal: connect strategy decisions to live external yield allocation.

Work items:

- Finalize strategy wallet and trusted destination accounts.
- Add audited deposit and withdraw flows for Kamino first.
- Add policy gates for entering and exiting yield.
- Add portfolio state reconciliation between internal records and external protocol balances.

Exit criteria:

- Yield positions are opened and closed through controlled live execution with reconciliation.

## Phase 5: AI Operating Layer

Goal: make AI useful without letting it become an unbounded control surface.

Work items:

- Ground AI prompts in live portfolio and execution state only.
- Make control-mode transitions explainable and auditable.
- Restrict chat actions to a typed allowlist with explicit execution policies.
- Add operator confirmation flows where needed.

Exit criteria:

- AI can propose and control bounded actions with clear audit traces.

## Phase 6: Treasury OS Product Layer

Goal: make the system operable as a real treasury console.

Work items:

- Persist settings and operator preferences durably.
- Finish history, goals, alerts, and payment workflows.
- Clean up onboarding so it performs real checks instead of demo-only success paths.
- Add deployment-ready environment validation and health reporting.

Exit criteria:

- The product behaves like a real operator console rather than a stitched demo.
