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

  let mintA: anchor.web3.PublicKey;
  let mintB: anchor.web3.PublicKey;
  let vaultPda: anchor.web3.PublicKey;
  let vaultTokenA: anchor.web3.PublicKey;
  let vaultTokenB: anchor.web3.PublicKey;
  let userTokenA: anchor.web3.PublicKey;
  let userTokenB: anchor.web3.PublicKey;

  before(async () => {
    mintA = await createMint(provider.connection, authority.payer, authority.publicKey, null, 6);
    mintB = await createMint(provider.connection, authority.payer, authority.publicKey, null, 6);

    [vaultPda] = anchor.web3.PublicKey.findProgramAddressSync(
      [Buffer.from("vault"), authority.publicKey.toBuffer()],
      program.programId
    );
    [vaultTokenA] = anchor.web3.PublicKey.findProgramAddressSync(
      [Buffer.from("vault_token_a"), vaultPda.toBuffer()],
      program.programId
    );
    [vaultTokenB] = anchor.web3.PublicKey.findProgramAddressSync(
      [Buffer.from("vault_token_b"), vaultPda.toBuffer()],
      program.programId
    );

    userTokenA = await createAccount(provider.connection, authority.payer, mintA, authority.publicKey);
    userTokenB = await createAccount(provider.connection, authority.payer, mintB, authority.publicKey);

    await mintTo(provider.connection, authority.payer, mintA, userTokenA, authority.publicKey, 10_000_000 * 1e6);
    await mintTo(provider.connection, authority.payer, mintB, userTokenB, authority.publicKey, 10_000_000 * 1e6);
  });

  // ─────────────────────────────────────────────────────────────────────────
  // ORIGINAL 9 TESTS
  // ─────────────────────────────────────────────────────────────────────────

  it("initialize_vault: creates vault with two mints", async () => {
    const tx = await program.methods
      .initializeVault({ rebalanceThreshold: new anchor.BN(70), maxDeposit: new anchor.BN(1_000_000 * 1e6) })
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        mintA,
        mintB,
        vaultTokenA,
        vaultTokenB,
        tokenProgram: TOKEN_PROGRAM_ID,
        systemProgram: anchor.web3.SystemProgram.programId,
        rent: anchor.web3.SYSVAR_RENT_PUBKEY,
      })
      .rpc();
    console.log("initialize_vault tx:", tx);

    const vault = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vault.mintA.toBase58(), mintA.toBase58());
    assert.equal(vault.mintB.toBase58(), mintB.toBase58());
    assert.equal(vault.isPaused, false);
    assert.equal(vault.totalRebalances.toNumber(), 0);
    assert.equal(vault.decisionCount.toNumber(), 0);
    assert.equal(vault.strategyMode, 0, "default strategy should be Safe (0)");
  });

  it("deposit: deposits token A into vault", async () => {
    await program.methods
      .deposit(new anchor.BN(100 * 1e6), true)
      .accounts({
        user: authority.publicKey,
        vault: vaultPda,
        userTokenAccount: userTokenA,
        vaultTokenA,
        vaultTokenB,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .rpc();

    const vaultAccA = await getAccount(provider.connection, vaultTokenA);
    assert.equal(vaultAccA.amount.toString(), (100 * 1e6).toString());
    const vault = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vault.totalDeposited.toNumber(), 100 * 1e6);
  });

  it("deposit: deposits token B into vault", async () => {
    await program.methods
      .deposit(new anchor.BN(200 * 1e6), false)
      .accounts({
        user: authority.publicKey,
        vault: vaultPda,
        userTokenAccount: userTokenB,
        vaultTokenA,
        vaultTokenB,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .rpc();

    const vaultAccB = await getAccount(provider.connection, vaultTokenB);
    assert.equal(vaultAccB.amount.toString(), (200 * 1e6).toString());
  });

  it("execute_rebalance: shifts virtual allocation A→B (direction=0)", async () => {
    const amount = new anchor.BN(50 * 1e6);
    const vaultBefore = await program.account.vaultState.fetch(vaultPda);

    await program.methods
      .executeRebalance(0, amount)
      .accounts({ authority: authority.publicKey, vault: vaultPda })
      .rpc();

    const vaultAfter = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vaultAfter.balanceA.toNumber(), vaultBefore.balanceA.toNumber() - 50 * 1e6);
    assert.equal(vaultAfter.balanceB.toNumber(), vaultBefore.balanceB.toNumber() + 50 * 1e6);
    assert.equal(vaultAfter.totalRebalances.toNumber(), 1);
  });

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
        .deposit(new anchor.BN(10 * 1e6), true)
        .accounts({
          user: authority.publicKey,
          vault: vaultPda,
          userTokenAccount: userTokenA,
          vaultTokenA,
          vaultTokenB,
          tokenProgram: TOKEN_PROGRAM_ID,
        })
        .rpc();
      assert.fail("Should have thrown VaultPaused error");
    } catch (err: any) {
      assert.include(err.message, "VaultPaused");
    }
  });

  it("withdraw: works even when vault is paused", async () => {
    const withdrawAmount = new anchor.BN(10 * 1e6);
    const before = await getAccount(provider.connection, userTokenA);

    await program.methods
      .withdraw(withdrawAmount, true)
      .accounts({
        user: authority.publicKey,
        vault: vaultPda,
        userTokenAccount: userTokenA,
        vaultTokenA,
        vaultTokenB,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .rpc();

    const after = await getAccount(provider.connection, userTokenA);
    assert.equal(
      (BigInt(after.amount) - BigInt(before.amount)).toString(),
      (10 * 1e6).toString()
    );
  });

  it("toggle_pause: unpauses vault", async () => {
    await program.methods
      .togglePause()
      .accounts({ authority: authority.publicKey, vault: vaultPda })
      .rpc();

    const vault = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vault.isPaused, false);
  });

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
  // NEW TESTS: set_strategy
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
  // NEW TESTS: send_payment
  // ─────────────────────────────────────────────────────────────────────────

  it("send_payment: sends token A from vault to recipient", async () => {
    // Create a recipient wallet + their token A account
    const recipient = anchor.web3.Keypair.generate();
    const recipientTokenA = await createAccount(
      provider.connection,
      authority.payer,
      mintA,
      recipient.publicKey
    );

    const payAmount = new anchor.BN(20 * 1e6);
    const vaultBefore = await program.account.vaultState.fetch(vaultPda);
    const balanceABefore = vaultBefore.balanceA.toNumber();

    await program.methods
      .sendPayment(payAmount, true)
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        vaultTokenAccount: vaultTokenA,
        recipientTokenAccount: recipientTokenA,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .rpc();

    // Recipient got the tokens
    const recipientAcc = await getAccount(provider.connection, recipientTokenA);
    assert.equal(recipientAcc.amount.toString(), (20 * 1e6).toString());

    // Vault balance_a decreased
    const vaultAfter = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vaultAfter.balanceA.toNumber(), balanceABefore - 20 * 1e6);
  });

  it("send_payment: fails when balance insufficient", async () => {
    const recipient = anchor.web3.Keypair.generate();
    const recipientTokenA = await createAccount(
      provider.connection,
      authority.payer,
      mintA,
      recipient.publicKey
    );

    try {
      await program.methods
        .sendPayment(new anchor.BN(999_999 * 1e6), true)
        .accounts({
          authority: authority.publicKey,
          vault: vaultPda,
          vaultTokenAccount: vaultTokenA,
          recipientTokenAccount: recipientTokenA,
          tokenProgram: TOKEN_PROGRAM_ID,
        })
        .rpc();
      assert.fail("Should have thrown InsufficientBalance");
    } catch (err: any) {
      assert.include(err.message, "InsufficientBalance");
    }
  });

  // ─────────────────────────────────────────────────────────────────────────
  // NEW TESTS: update_threshold
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
  // NEW TESTS: emergency_withdraw
  // ─────────────────────────────────────────────────────────────────────────

  it("emergency_withdraw: drains all vault tokens to authority", async () => {
    // Read actual SPL balances before withdraw
    const splA = await getAccount(provider.connection, vaultTokenA);
    const splB = await getAccount(provider.connection, vaultTokenB);
    const expectedA = BigInt(splA.amount);
    const expectedB = BigInt(splB.amount);

    const userABefore = await getAccount(provider.connection, userTokenA);
    const userBBefore = await getAccount(provider.connection, userTokenB);

    await program.methods
      .emergencyWithdraw()
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        vaultTokenA,
        vaultTokenB,
        authorityTokenA: userTokenA,
        authorityTokenB: userTokenB,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .rpc();

    // Vault virtual balances are zero
    const vaultAfter = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vaultAfter.balanceA.toNumber(), 0);
    assert.equal(vaultAfter.balanceB.toNumber(), 0);
    assert.equal(vaultAfter.totalDeposited.toNumber(), 0);

    // Vault SPL accounts are drained
    const splAAfter = await getAccount(provider.connection, vaultTokenA);
    const splBAfter = await getAccount(provider.connection, vaultTokenB);
    assert.equal(splAAfter.amount.toString(), "0");
    assert.equal(splBAfter.amount.toString(), "0");

    // Authority received the tokens
    const userAAfter = await getAccount(provider.connection, userTokenA);
    const userBAfter = await getAccount(provider.connection, userTokenB);
    assert.equal(
      (BigInt(userAAfter.amount) - BigInt(userABefore.amount)).toString(),
      expectedA.toString()
    );
    assert.equal(
      (BigInt(userBAfter.amount) - BigInt(userBBefore.amount)).toString(),
      expectedB.toString()
    );
  });

  it("emergency_withdraw: works even when vault is paused", async () => {
    // Re-deposit some tokens
    await program.methods
      .deposit(new anchor.BN(50 * 1e6), true)
      .accounts({
        user: authority.publicKey,
        vault: vaultPda,
        userTokenAccount: userTokenA,
        vaultTokenA,
        vaultTokenB,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .rpc();

    // Pause the vault
    await program.methods
      .togglePause()
      .accounts({ authority: authority.publicKey, vault: vaultPda })
      .rpc();

    const vaultPaused = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vaultPaused.isPaused, true);

    // Emergency withdraw should succeed despite pause
    await program.methods
      .emergencyWithdraw()
      .accounts({
        authority: authority.publicKey,
        vault: vaultPda,
        vaultTokenA,
        vaultTokenB,
        authorityTokenA: userTokenA,
        authorityTokenB: userTokenB,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .rpc();

    const vaultAfter = await program.account.vaultState.fetch(vaultPda);
    assert.equal(vaultAfter.balanceA.toNumber(), 0);
    assert.equal(vaultAfter.balanceB.toNumber(), 0);

    // Unpause for any future tests
    await program.methods
      .togglePause()
      .accounts({ authority: authority.publicKey, vault: vaultPda })
      .rpc();
  });
});
