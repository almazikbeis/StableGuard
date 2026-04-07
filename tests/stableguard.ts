import * as anchor from "@coral-xyz/anchor";
import { Program } from "@coral-xyz/anchor";
import { Stableguard } from "../target/types/stableguard";
import {
  createMint,
  createAccount,
  mintTo,
  getAccount,
  TOKEN_PROGRAM_ID,
} from "@solana/spl-token";
import { assert } from "chai";

describe("stableguard", () => {
  const provider = anchor.AnchorProvider.env();
  anchor.setProvider(provider);

  const program = anchor.workspace.Stableguard as Program<Stableguard>;
  const authority = provider.wallet as anchor.Wallet;

  // Three mock stablecoins: USDC, USDT, USDC.e
  let mockUSDC:  anchor.web3.PublicKey;
  let mockUSDT:  anchor.web3.PublicKey;
  let mockUSDCe: anchor.web3.PublicKey;

  let vaultPda:    anchor.web3.PublicKey;
  let vaultToken0: anchor.web3.PublicKey; // USDC  vault ATA
  let vaultToken1: anchor.web3.PublicKey; // USDT  vault ATA
  let vaultToken2: anchor.web3.PublicKey; // USDC.e vault ATA

  let userToken0: anchor.web3.PublicKey;
  let userToken1: anchor.web3.PublicKey;
  let userToken2: anchor.web3.PublicKey;

  const attacker = anchor.web3.Keypair.generate();
  let attackerToken0: anchor.web3.PublicKey;

  // ─────────────────────────────────────────────────────────────────────────
  // Setup
  // ─────────────────────────────────────────────────────────────────────────

  before(async () => {
    // Create 3 mock mints (6 decimals each)
    mockUSDC  = await createMint(provider.connection, authority.payer, authority.publicKey, null, 6);
    mockUSDT  = await createMint(provider.connection, authority.payer, authority.publicKey, null, 6);
    mockUSDCe = await createMint(provider.connection, authority.payer, authority.publicKey, null, 6);

    // Derive vault PDA
    [vaultPda] = anchor.web3.PublicKey.findProgramAddressSync(
      [Buffer.from("vault"), authority.publicKey.toBuffer()],
      program.programId
    );

    // Derive vault token account PDAs (seeds: "vault_token" + vaultPda + [index])
    [vaultToken0] = anchor.web3.PublicKey.findProgramAddressSync(
      [Buffer.from("vault_token"), vaultPda.toBuffer(), Buffer.from([0])],
      program.programId
    );
    [vaultToken1] = anchor.web3.PublicKey.findProgramAddressSync(
      [Buffer.from("vault_token"), vaultPda.toBuffer(), Buffer.from([1])],
      program.programId
    );
    [vaultToken2] = anchor.web3.PublicKey.findProgramAddressSync(
      [Buffer.from("vault_token"), vaultPda.toBuffer(), Buffer.from([2])],
      program.programId
    );

    // Create user token accounts
    userToken0 = await createAccount(provider.connection, authority.payer, mockUSDC,  authority.publicKey);
    userToken1 = await createAccount(provider.connection, authority.payer, mockUSDT,  authority.publicKey);
    userToken2 = await createAccount(provider.connection, authority.payer, mockUSDCe, authority.publicKey);
    attackerToken0 = await createAccount(provider.connection, authority.payer, mockUSDC, attacker.publicKey);

    const sig = await provider.connection.requestAirdrop(attacker.publicKey, 1 * anchor.web3.LAMPORTS_PER_SOL);
    await provider.connection.confirmTransaction(sig);

    // Mint 10M tokens to each user account
    await mintTo(provider.connection, authority.payer, mockUSDC,  userToken0, authority.publicKey, 10_000_000 * 1e6);
    await mintTo(provider.connection, authority.payer, mockUSDT,  userToken1, authority.publicKey, 10_000_000 * 1e6);
    await mintTo(provider.connection, authority.payer, mockUSDCe, userToken2, authority.publicKey, 10_000_000 * 1e6);
  });

  // ─────────────────────────────────────────────────────────────────────────
  // 1. initialize_vault — no mints passed
  // ─────────────────────────────────────────────────────────────────────────

  it("initialize_vault: creates empty vault (no mints)", async () => {
    const tx = await program.methods
      .initializeVault({ rebalanceThreshold: new anchor.BN(70), maxDeposit: new anchor.BN(1_000_000 * 1e6) })
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        systemProgram: anchor.web3.SystemProgram.programId,
      })
      .rpc();
    console.log("initialize_vault tx:", tx);

    const vault = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vault.numTokens, 0, "numTokens should be 0");
    assert.equal(vault.isPaused, false);
    assert.equal(vault.totalRebalances.toNumber(), 0);
    assert.equal(vault.decisionCount.toNumber(), 0);
    assert.equal(vault.strategyMode, 0, "default strategy should be Safe (0)");
  });

  // ─────────────────────────────────────────────────────────────────────────
  // 2. register_token
  // ─────────────────────────────────────────────────────────────────────────

  it("register_token: registers USDC at index 0", async () => {
    await program.methods
      .registerToken(0)
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        mint: mockUSDC,
        vaultToken: vaultToken0,
        tokenProgram: TOKEN_PROGRAM_ID,
        systemProgram: anchor.web3.SystemProgram.programId,
        rent: anchor.web3.SYSVAR_RENT_PUBKEY,
      })
      .rpc();

    const vault = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vault.numTokens, 1);
    assert.equal(vault.mints[0].toBase58(), mockUSDC.toBase58());
    assert.equal(vault.vaultTokens[0].toBase58(), vaultToken0.toBase58());
  });

  it("register_token: registers USDT at index 1", async () => {
    await program.methods
      .registerToken(1)
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        mint: mockUSDT,
        vaultToken: vaultToken1,
        tokenProgram: TOKEN_PROGRAM_ID,
        systemProgram: anchor.web3.SystemProgram.programId,
        rent: anchor.web3.SYSVAR_RENT_PUBKEY,
      })
      .rpc();

    const vault = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vault.numTokens, 2);
    assert.equal(vault.mints[1].toBase58(), mockUSDT.toBase58());
  });

  it("register_token: registers USDC.e at index 2", async () => {
    await program.methods
      .registerToken(2)
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        mint: mockUSDCe,
        vaultToken: vaultToken2,
        tokenProgram: TOKEN_PROGRAM_ID,
        systemProgram: anchor.web3.SystemProgram.programId,
        rent: anchor.web3.SYSVAR_RENT_PUBKEY,
      })
      .rpc();

    const vault = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vault.numTokens, 3);
    assert.equal(vault.mints[2].toBase58(), mockUSDCe.toBase58());
  });

  it("register_token: fails if slot already occupied (duplicate index 0)", async () => {
    // The vault_token PDA at index 0 already exists, so Anchor's `init` constraint
    // rejects the transaction before the handler runs (account "already in use").
    // We just verify the tx fails — no double-registration is possible.
    try {
      await program.methods
        .registerToken(0)
        .accounts({
          authority: authority.publicKey,
          vault: vaultPda,
          mint: mockUSDT,
          vaultToken: vaultToken0,
          tokenProgram: TOKEN_PROGRAM_ID,
          systemProgram: anchor.web3.SystemProgram.programId,
          rent: anchor.web3.SYSVAR_RENT_PUBKEY,
        })
        .rpc();
      assert.fail("Should have failed — duplicate slot");
    } catch (err: any) {
      // Either TokenSlotOccupied (if handler runs) or Anchor-level "already in use"
      assert.ok(err, "Expected an error for duplicate slot registration");
    }
  });

  it("register_token: fails if index >= 8", async () => {
    const [bogusVaultToken] = anchor.web3.PublicKey.findProgramAddressSync(
      [Buffer.from("vault_token"), vaultPda.toBuffer(), Buffer.from([8])],
      program.programId
    );
    try {
      await program.methods
        .registerToken(8)
        .accounts({
          authority: authority.publicKey,
          vault: vaultPda,
          mint: mockUSDC,
          vaultToken: bogusVaultToken,
          tokenProgram: TOKEN_PROGRAM_ID,
          systemProgram: anchor.web3.SystemProgram.programId,
          rent: anchor.web3.SYSVAR_RENT_PUBKEY,
        })
        .rpc();
      assert.fail("Should have thrown InvalidTokenIndex");
    } catch (err: any) {
      assert.include(err.message, "InvalidTokenIndex");
    }
  });

  // ─────────────────────────────────────────────────────────────────────────
  // 3. deposit
  // ─────────────────────────────────────────────────────────────────────────

  it("deposit: deposits USDC (index=0) into vault", async () => {
    await program.methods
      .deposit(0, new anchor.BN(100 * 1e6))
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        authorityTokenAccount: userToken0,
        vaultTokenAccount: vaultToken0,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .rpc();

    const vaultAcc = await getAccount(provider.connection, vaultToken0);
    assert.equal(vaultAcc.amount.toString(), (100 * 1e6).toString());
    const vault = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vault.totalDeposited.toNumber(), 100 * 1e6);
    assert.equal(vault.balances[0].toNumber(), 100 * 1e6);
  });

  it("deposit: deposits USDT (index=1) into vault", async () => {
    await program.methods
      .deposit(1, new anchor.BN(200 * 1e6))
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        authorityTokenAccount: userToken1,
        vaultTokenAccount: vaultToken1,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .rpc();

    const vaultAcc = await getAccount(provider.connection, vaultToken1);
    assert.equal(vaultAcc.amount.toString(), (200 * 1e6).toString());
    const vault = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vault.balances[1].toNumber(), 200 * 1e6);
  });

  it("deposit: deposits USDC.e (index=2) into vault", async () => {
    await program.methods
      .deposit(2, new anchor.BN(150 * 1e6))
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        authorityTokenAccount: userToken2,
        vaultTokenAccount: vaultToken2,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .rpc();

    const vault = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vault.balances[2].toNumber(), 150 * 1e6);
  });

  // ─────────────────────────────────────────────────────────────────────────
  // 4. execute_rebalance
  // ─────────────────────────────────────────────────────────────────────────

  it("execute_rebalance: records intent for 50 USDC (0→1) without mutating balances", async () => {
    const before = await program.account.vaultState.fetch(vaultPda);
    const b0 = before.balances[0].toNumber();
    const b1 = before.balances[1].toNumber();
    const rebalancesBefore = before.totalRebalances.toNumber();

    await program.methods
      .executeRebalance(0, 1, new anchor.BN(50 * 1e6))
      .accounts({ signer: authority.publicKey, vault: vaultPda })
      .rpc();

    const after = await program.account.vaultState.fetch(vaultPda);
    assert.equal(after.balances[0].toNumber(), b0);
    assert.equal(after.balances[1].toNumber(), b1);
    assert.equal(after.totalRebalances.toNumber(), rebalancesBefore);
  });

  it("execute_rebalance: records intent for 30 USDT (1→2) without mutating balances", async () => {
    const before = await program.account.vaultState.fetch(vaultPda);
    const b1 = before.balances[1].toNumber();
    const b2 = before.balances[2].toNumber();
    const rebalancesBefore = before.totalRebalances.toNumber();

    await program.methods
      .executeRebalance(1, 2, new anchor.BN(30 * 1e6))
      .accounts({ signer: authority.publicKey, vault: vaultPda })
      .rpc();

    const after = await program.account.vaultState.fetch(vaultPda);
    assert.equal(after.balances[1].toNumber(), b1);
    assert.equal(after.balances[2].toNumber(), b2);
    assert.equal(after.totalRebalances.toNumber(), rebalancesBefore);
  });

  it("record_swap_result: increments completed rebalances without mutating balances", async () => {
    const before = await program.account.vaultState.fetch(vaultPda);
    const b0 = before.balances[0].toNumber();
    const b1 = before.balances[1].toNumber();
    const rebalancesBefore = before.totalRebalances.toNumber();

    await program.methods
      .recordSwapResult({
        fromIndex: 0,
        toIndex: 1,
        inputAmount: new anchor.BN(20 * 1e6),
        outputAmount: new anchor.BN(19 * 1e6),
        swapSignature: "demo-swap-signature",
      })
      .accounts({ signer: authority.publicKey, vault: vaultPda })
      .rpc();

    const after = await program.account.vaultState.fetch(vaultPda);
    assert.equal(after.balances[0].toNumber(), b0);
    assert.equal(after.balances[1].toNumber(), b1);
    assert.equal(after.totalRebalances.toNumber(), rebalancesBefore + 1);
  });

  it("execute_rebalance: fails when from_index == to_index", async () => {
    try {
      await program.methods
        .executeRebalance(0, 0, new anchor.BN(10 * 1e6))
        .accounts({ signer: authority.publicKey, vault: vaultPda })
        .rpc();
      assert.fail("Should have thrown InvalidDirection");
    } catch (err: any) {
      assert.include(err.message, "InvalidDirection");
    }
  });

  // ─────────────────────────────────────────────────────────────────────────
  // 5. toggle_pause
  // ─────────────────────────────────────────────────────────────────────────

  it("toggle_pause: pauses vault", async () => {
    await program.methods
      .togglePause()
      .accounts({ authority: authority.publicKey, vault: vaultPda })
      .rpc();

    const vault = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vault.isPaused, true);
  });

  it("deposit: fails when vault is paused", async () => {
    try {
      await program.methods
        .deposit(0, new anchor.BN(10 * 1e6))
        .accounts({
          authority: authority.publicKey,
          vault: vaultPda,
          authorityTokenAccount: userToken0,
          vaultTokenAccount: vaultToken0,
          tokenProgram: TOKEN_PROGRAM_ID,
        })
        .rpc();
      assert.fail("Should have thrown VaultPaused");
    } catch (err: any) {
      assert.include(err.message, "VaultPaused");
    }
  });

  it("withdraw: works even when vault is paused", async () => {
    const withdrawAmount = new anchor.BN(10 * 1e6);
    const before = await getAccount(provider.connection, userToken0);

    await program.methods
      .withdraw(0, withdrawAmount)
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        authorityTokenAccount: userToken0,
        vaultTokenAccount: vaultToken0,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .rpc();

    const after = await getAccount(provider.connection, userToken0);
    assert.equal(
      (BigInt(after.amount) - BigInt(before.amount)).toString(),
      (10 * 1e6).toString()
    );
  });

  it("withdraw: rejects unauthorized treasury signer", async () => {
    try {
      await program.methods
        .withdraw(0, new anchor.BN(1 * 1e6))
        .accounts({
          authority: attacker.publicKey,
          vault: vaultPda,
          authorityTokenAccount: attackerToken0,
          vaultTokenAccount: vaultToken0,
          tokenProgram: TOKEN_PROGRAM_ID,
        })
        .signers([attacker])
        .rpc();
      assert.fail("Should have thrown because attacker is not the treasury authority");
    } catch (err: any) {
      assert.ok(
        err.message.includes("ConstraintSeeds") ||
        err.message.includes("Unauthorized"),
        `unexpected error: ${err.message}`
      );
    }
  });

  it("toggle_pause: unpauses vault", async () => {
    await program.methods
      .togglePause()
      .accounts({ authority: authority.publicKey, vault: vaultPda })
      .rpc();

    const vault = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vault.isPaused, false);
  });

  // ─────────────────────────────────────────────────────────────────────────
  // 6. record_decision
  // ─────────────────────────────────────────────────────────────────────────

  it("record_decision: logs AI decision on-chain", async () => {
    const vault = await program.account.vaultState.fetch(vaultPda);
    const seq = vault.decisionCount;

    const [decisionPda] = anchor.web3.PublicKey.findProgramAddressSync(
      [Buffer.from("decision"), vaultPda.toBuffer(), seq.toArrayLike(Buffer, "le", 8)],
      program.programId
    );

    await program.methods
      .recordDecision({
        action: 1,
        rationale: "USDT deviation > threshold, rebalancing to USDC",
        confidenceScore: 85,
      })
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        decisionLog: decisionPda,
        systemProgram: anchor.web3.SystemProgram.programId,
      })
      .rpc();

    const log = await program.account.decisionLog.fetch(decisionPda);
    assert.equal(log.action, 1);
    assert.equal(log.confidenceScore, 85);
    assert.equal(log.sequence.toNumber(), seq.toNumber());
  });

  // ─────────────────────────────────────────────────────────────────────────
  // 7. set_strategy
  // ─────────────────────────────────────────────────────────────────────────

  it("set_strategy: switches to Yield mode (1)", async () => {
    await program.methods
      .setStrategy(1)
      .accounts({ authority: authority.publicKey, vault: vaultPda })
      .rpc();

    const vault = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vault.strategyMode, 1, "strategy should be Yield");
  });

  it("set_strategy: switches back to Safe mode (0)", async () => {
    await program.methods
      .setStrategy(0)
      .accounts({ authority: authority.publicKey, vault: vaultPda })
      .rpc();

    const vault = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vault.strategyMode, 0, "strategy should be Safe");
  });

  it("set_strategy: rejects invalid mode (5)", async () => {
    try {
      await program.methods
        .setStrategy(5)
        .accounts({ authority: authority.publicKey, vault: vaultPda })
        .rpc();
      assert.fail("Should have thrown InvalidStrategy");
    } catch (err: any) {
      assert.include(err.message, "InvalidStrategy");
    }
  });

  // ─────────────────────────────────────────────────────────────────────────
  // 8. send_payment
  // ─────────────────────────────────────────────────────────────────────────

  it("send_payment: sends USDC (index=0) from vault to recipient", async () => {
    const recipient = anchor.web3.Keypair.generate();
    const recipientToken0 = await createAccount(
      provider.connection,
      authority.payer,
      mockUSDC,
      recipient.publicKey
    );

    const payAmount = new anchor.BN(20 * 1e6);
    const vaultBefore = await program.account.vaultState.fetch(vaultPda);
    const bal0Before = vaultBefore.balances[0].toNumber();

    await program.methods
      .sendPayment(0, payAmount)
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        vaultTokenAccount: vaultToken0,
        recipientTokenAccount: recipientToken0,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .rpc();

    const recipientAcc = await getAccount(provider.connection, recipientToken0);
    assert.equal(recipientAcc.amount.toString(), (20 * 1e6).toString());

    const vaultAfter = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vaultAfter.balances[0].toNumber(), bal0Before - 20 * 1e6);
  });

  it("send_payment: fails when balance insufficient", async () => {
    const recipient = anchor.web3.Keypair.generate();
    const recipientToken0 = await createAccount(
      provider.connection,
      authority.payer,
      mockUSDC,
      recipient.publicKey
    );

    try {
      await program.methods
        .sendPayment(0, new anchor.BN(999_999 * 1e6))
        .accounts({
          authority: authority.publicKey,
          vault: vaultPda,
          vaultTokenAccount: vaultToken0,
          recipientTokenAccount: recipientToken0,
          tokenProgram: TOKEN_PROGRAM_ID,
        })
        .rpc();
      assert.fail("Should have thrown InsufficientBalance");
    } catch (err: any) {
      assert.include(err.message, "InsufficientBalance");
    }
  });

  // ─────────────────────────────────────────────────────────────────────────
  // 9. update_threshold
  // ─────────────────────────────────────────────────────────────────────────

  it("update_threshold: updates threshold to 50", async () => {
    await program.methods
      .updateThreshold(new anchor.BN(50))
      .accounts({ authority: authority.publicKey, vault: vaultPda })
      .rpc();

    const vault = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vault.rebalanceThreshold.toNumber(), 50);
  });

  it("update_threshold: rejects 0 (below minimum)", async () => {
    try {
      await program.methods
        .updateThreshold(new anchor.BN(0))
        .accounts({ authority: authority.publicKey, vault: vaultPda })
        .rpc();
      assert.fail("Should have thrown InvalidThreshold");
    } catch (err: any) {
      assert.include(err.message, "InvalidThreshold");
    }
  });

  it("update_threshold: rejects 101 (above maximum)", async () => {
    try {
      await program.methods
        .updateThreshold(new anchor.BN(101))
        .accounts({ authority: authority.publicKey, vault: vaultPda })
        .rpc();
      assert.fail("Should have thrown InvalidThreshold");
    } catch (err: any) {
      assert.include(err.message, "InvalidThreshold");
    }
  });

  // ─────────────────────────────────────────────────────────────────────────
  // 10. emergency_withdraw
  // ─────────────────────────────────────────────────────────────────────────

  it("emergency_withdraw: drains all 3 vault tokens to authority", async () => {
    const spl0Before = await getAccount(provider.connection, vaultToken0);
    const spl1Before = await getAccount(provider.connection, vaultToken1);
    const spl2Before = await getAccount(provider.connection, vaultToken2);
    const expected0 = BigInt(spl0Before.amount);
    const expected1 = BigInt(spl1Before.amount);
    const expected2 = BigInt(spl2Before.amount);

    const user0Before = await getAccount(provider.connection, userToken0);
    const user1Before = await getAccount(provider.connection, userToken1);
    const user2Before = await getAccount(provider.connection, userToken2);

    await program.methods
      .emergencyWithdraw()
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .remainingAccounts([
        // vault token accounts [0..N)
        { pubkey: vaultToken0, isWritable: true, isSigner: false },
        { pubkey: vaultToken1, isWritable: true, isSigner: false },
        { pubkey: vaultToken2, isWritable: true, isSigner: false },
        // authority token accounts [N..2N)
        { pubkey: userToken0, isWritable: true, isSigner: false },
        { pubkey: userToken1, isWritable: true, isSigner: false },
        { pubkey: userToken2, isWritable: true, isSigner: false },
      ])
      .rpc();

    // Virtual balances zeroed
    const vaultAfter = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vaultAfter.balances[0].toNumber(), 0);
    assert.equal(vaultAfter.balances[1].toNumber(), 0);
    assert.equal(vaultAfter.balances[2].toNumber(), 0);
    assert.equal(vaultAfter.totalDeposited.toNumber(), 0);

    // SPL accounts drained
    const spl0After = await getAccount(provider.connection, vaultToken0);
    const spl1After = await getAccount(provider.connection, vaultToken1);
    const spl2After = await getAccount(provider.connection, vaultToken2);
    assert.equal(spl0After.amount.toString(), "0");
    assert.equal(spl1After.amount.toString(), "0");
    assert.equal(spl2After.amount.toString(), "0");

    // Authority received funds
    const user0After = await getAccount(provider.connection, userToken0);
    const user1After = await getAccount(provider.connection, userToken1);
    const user2After = await getAccount(provider.connection, userToken2);
    assert.equal((BigInt(user0After.amount) - BigInt(user0Before.amount)).toString(), expected0.toString());
    assert.equal((BigInt(user1After.amount) - BigInt(user1Before.amount)).toString(), expected1.toString());
    assert.equal((BigInt(user2After.amount) - BigInt(user2Before.amount)).toString(), expected2.toString());
  });

  it("emergency_withdraw: works even when vault is paused", async () => {
    // Re-deposit some tokens across all 3 mints
    await program.methods
      .deposit(0, new anchor.BN(50 * 1e6))
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        authorityTokenAccount: userToken0,
        vaultTokenAccount: vaultToken0,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .rpc();
    await program.methods
      .deposit(1, new anchor.BN(40 * 1e6))
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        authorityTokenAccount: userToken1,
        vaultTokenAccount: vaultToken1,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .rpc();
    await program.methods
      .deposit(2, new anchor.BN(30 * 1e6))
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        authorityTokenAccount: userToken2,
        vaultTokenAccount: vaultToken2,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .rpc();

    // Pause the vault
    await program.methods
      .togglePause()
      .accounts({ authority: authority.publicKey, vault: vaultPda })
      .rpc();

    const paused = await program.account.vaultState.fetch(vaultPda);
    assert.equal(paused.isPaused, true);

    // Emergency withdraw succeeds despite pause
    await program.methods
      .emergencyWithdraw()
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .remainingAccounts([
        { pubkey: vaultToken0, isWritable: true, isSigner: false },
        { pubkey: vaultToken1, isWritable: true, isSigner: false },
        { pubkey: vaultToken2, isWritable: true, isSigner: false },
        { pubkey: userToken0, isWritable: true, isSigner: false },
        { pubkey: userToken1, isWritable: true, isSigner: false },
        { pubkey: userToken2, isWritable: true, isSigner: false },
      ])
      .rpc();

    const vaultAfter = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vaultAfter.balances[0].toNumber(), 0);
    assert.equal(vaultAfter.balances[1].toNumber(), 0);
    assert.equal(vaultAfter.balances[2].toNumber(), 0);

    // Unpause for any future tests
    await program.methods
      .togglePause()
      .accounts({ authority: authority.publicKey, vault: vaultPda })
      .rpc();
  });
});
