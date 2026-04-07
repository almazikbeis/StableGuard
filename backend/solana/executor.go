// Package solana handles on-chain interaction with the StableGuard program.
package solana

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// ProgramIDStr is the deployed StableGuard program.
const ProgramIDStr = "GPSJJqicuDSJ6LXhEZpmUboThjzdefG5wZAZkL2hd7es"

// MaxTokens mirrors the on-chain constant.
const MaxTokens = 8

// Executor submits on-chain transactions to the StableGuard program.
type Executor struct {
	rpcClient   *rpc.Client
	wallet      solana.PrivateKey
	programID   solana.PublicKey
	authorityPK solana.PublicKey // explicit authority for PDA derivation (may differ from wallet when wallet = agent)

	// recordDecisionMu serializes record_decision calls so the pipeline background
	// loop and manual demo button don't race for the same decision_count PDA slot.
	recordDecisionMu sync.Mutex
}

// VaultState mirrors the on-chain VaultState account layout.
type VaultState struct {
	Authority               [32]byte
	Mints                   [MaxTokens][32]byte
	VaultTokens             [MaxTokens][32]byte
	TotalDeposited          uint64
	Balances                [MaxTokens]uint64
	RebalanceThreshold      uint64
	MaxDeposit              uint64
	DecisionCount           uint64
	TotalRebalances         uint64
	NumTokens               uint8
	IsPaused                bool
	StrategyMode            uint8
	Bump                    uint8
	DelegatedAgent          [32]byte
	CircuitBreakerThreshold uint64
	LastPrice               uint64
	PositionEpoch           uint64
}

// New creates a new Solana executor.
func New(rpcURL, walletKeyPath, programIDStr string) (*Executor, error) {
	client := rpc.New(rpcURL)

	privKey, err := loadWallet(walletKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load wallet: %w", err)
	}

	programID, err := solana.PublicKeyFromBase58(programIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid program id: %w", err)
	}

	e := &Executor{
		rpcClient: client,
		wallet:    privKey,
		programID: programID,
	}
	e.authorityPK = privKey.PublicKey()
	return e, nil
}

// DeriveVaultPDA derives the vault PDA for the given authority.
func (e *Executor) DeriveVaultPDA(authority solana.PublicKey) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress(
		[][]byte{[]byte("vault"), authority.Bytes()},
		e.programID,
	)
}

// DeriveVaultTokenPDA derives the PDA for a registered vault token at tokenIndex.
func (e *Executor) DeriveVaultTokenPDA(vaultPDA solana.PublicKey, tokenIndex uint8) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress(
		[][]byte{[]byte("vault_token"), vaultPDA.Bytes(), {tokenIndex}},
		e.programID,
	)
}

// ExecuteRebalance submits an execute_rebalance instruction on-chain.
// fromIndex/toIndex select the token slots to shift allocation between.
func (e *Executor) ExecuteRebalance(ctx context.Context, fromIndex, toIndex uint8, amount uint64) (string, error) {
	authority := e.wallet.PublicKey()

	vaultPDA, _, err := e.DeriveVaultPDA(authority)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}

	// Instruction data: discriminator(8) + from_index(1) + to_index(1) + amount(8)
	discriminator := anchorDiscriminator("execute_rebalance")
	data := make([]byte, 8+1+1+8)
	copy(data[0:8], discriminator)
	data[8] = fromIndex
	data[9] = toIndex
	binary.LittleEndian.PutUint64(data[10:], amount)

	accounts := []*solana.AccountMeta{
		{PublicKey: authority, IsSigner: true, IsWritable: true},
		{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
	}

	return e.sendSimpleTx(ctx, data, accounts)
}

// FetchVaultState fetches and decodes the on-chain VaultState account.
func (e *Executor) FetchVaultState(ctx context.Context, vaultPDA solana.PublicKey) (*VaultState, error) {
	acc, err := e.rpcClient.GetAccountInfoWithOpts(ctx, vaultPDA, &rpc.GetAccountInfoOpts{
		Commitment: rpc.CommitmentConfirmed,
	})
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}
	if acc == nil || acc.Value == nil {
		return nil, fmt.Errorf("vault account not found")
	}

	data := acc.Value.Data.GetBinary()
	// discriminator(8) + authority(32) + mints(256) + vault_tokens(256)
	// + total_deposited(8) + balances(64) + threshold(8) + max_deposit(8)
	// + decision_count(8) + total_rebalances(8) + num_tokens(1) + is_paused(1) + strategy(1) + bump(1)
	// + delegated_agent(32) + circuit_breaker_threshold(8) + last_price(8) + position_epoch(8)
	const minLen = 8 + 32 + 32*MaxTokens + 32*MaxTokens + 8 + 8*MaxTokens + 8 + 8 + 8 + 8 + 1 + 1 + 1 + 1 + 32 + 8 + 8 + 8
	if len(data) < minLen {
		return nil, fmt.Errorf("vault account data too short: %d bytes (need %d)", len(data), minLen)
	}

	offset := 8 // skip 8-byte Anchor discriminator
	vs := &VaultState{}

	copy(vs.Authority[:], data[offset:offset+32])
	offset += 32

	for i := 0; i < MaxTokens; i++ {
		copy(vs.Mints[i][:], data[offset:offset+32])
		offset += 32
	}
	for i := 0; i < MaxTokens; i++ {
		copy(vs.VaultTokens[i][:], data[offset:offset+32])
		offset += 32
	}

	vs.TotalDeposited = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	for i := 0; i < MaxTokens; i++ {
		vs.Balances[i] = binary.LittleEndian.Uint64(data[offset:])
		offset += 8
	}

	vs.RebalanceThreshold = binary.LittleEndian.Uint64(data[offset:])
	offset += 8
	vs.MaxDeposit = binary.LittleEndian.Uint64(data[offset:])
	offset += 8
	vs.DecisionCount = binary.LittleEndian.Uint64(data[offset:])
	offset += 8
	vs.TotalRebalances = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	vs.NumTokens = data[offset]
	offset++
	vs.IsPaused = data[offset] != 0
	offset++
	vs.StrategyMode = data[offset]
	offset++
	vs.Bump = data[offset]
	offset++

	// New fields (added in v2 security upgrade)
	copy(vs.DelegatedAgent[:], data[offset:offset+32])
	offset += 32
	vs.CircuitBreakerThreshold = binary.LittleEndian.Uint64(data[offset:])
	offset += 8
	vs.LastPrice = binary.LittleEndian.Uint64(data[offset:])
	offset += 8
	vs.PositionEpoch = binary.LittleEndian.Uint64(data[offset:])

	return vs, nil
}

// SendInitialize submits an initialize_vault instruction on-chain.
// Creates the vault PDA for the wallet authority.
func (e *Executor) SendInitialize(ctx context.Context, rebalanceThreshold, maxDeposit uint64) (string, error) {
	authority := e.wallet.PublicKey()
	vaultPDA, _, err := e.DeriveVaultPDA(authority)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}

	// instruction data: discriminator(8) + rebalance_threshold(8) + max_deposit(8)
	disc := anchorDiscriminator("initialize_vault")
	data := make([]byte, 8+8+8)
	copy(data[0:8], disc)
	binary.LittleEndian.PutUint64(data[8:], rebalanceThreshold)
	binary.LittleEndian.PutUint64(data[16:], maxDeposit)

	return e.sendSimpleTx(ctx, data, []*solana.AccountMeta{
		{PublicKey: authority, IsSigner: true, IsWritable: true},
		{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
		{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
	})
}

// SendRegisterToken submits a register_token instruction on-chain.
// mint is the base58-encoded mint address to register at tokenIndex.
func (e *Executor) SendRegisterToken(ctx context.Context, mint string, tokenIndex uint8) (string, error) {
	authority := e.wallet.PublicKey()
	vaultPDA, _, err := e.DeriveVaultPDA(authority)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}

	mintPK, err := solana.PublicKeyFromBase58(mint)
	if err != nil {
		return "", fmt.Errorf("invalid mint: %w", err)
	}

	vaultTokenPDA, _, err := e.DeriveVaultTokenPDA(vaultPDA, tokenIndex)
	if err != nil {
		return "", fmt.Errorf("derive vault token pda: %w", err)
	}

	disc := anchorDiscriminator("register_token")
	data := make([]byte, 8+1)
	copy(data[0:8], disc)
	data[8] = tokenIndex

	tokenProgram := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	systemProgram := solana.SystemProgramID
	rent := solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111")

	return e.sendSimpleTx(ctx, data, []*solana.AccountMeta{
		{PublicKey: authority, IsSigner: true, IsWritable: true},
		{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
		{PublicKey: mintPK, IsSigner: false, IsWritable: false},
		{PublicKey: vaultTokenPDA, IsSigner: false, IsWritable: true},
		{PublicKey: tokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: systemProgram, IsSigner: false, IsWritable: false},
		{PublicKey: rent, IsSigner: false, IsWritable: false},
	})
}

// SendSetStrategy submits a set_strategy instruction on-chain.
func (e *Executor) SendSetStrategy(ctx context.Context, mode uint8) (string, error) {
	authority := e.wallet.PublicKey()
	vaultPDA, _, err := e.DeriveVaultPDA(authority)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}

	disc := anchorDiscriminator("set_strategy")
	data := make([]byte, 8+1)
	copy(data[0:8], disc)
	data[8] = mode

	return e.sendSimpleTx(ctx, data, []*solana.AccountMeta{
		{PublicKey: authority, IsSigner: true, IsWritable: true},
		{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
	})
}

// SendUpdateThreshold submits an update_threshold instruction on-chain.
func (e *Executor) SendUpdateThreshold(ctx context.Context, threshold uint64) (string, error) {
	authority := e.wallet.PublicKey()
	vaultPDA, _, err := e.DeriveVaultPDA(authority)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}

	disc := anchorDiscriminator("update_threshold")
	data := make([]byte, 8+8)
	copy(data[0:8], disc)
	binary.LittleEndian.PutUint64(data[8:], threshold)

	return e.sendSimpleTx(ctx, data, []*solana.AccountMeta{
		{PublicKey: authority, IsSigner: true, IsWritable: true},
		{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
	})
}

// SendPayment submits a send_payment instruction on-chain.
// recipient is the base58-encoded public key of the recipient's token account.
func (e *Executor) SendPayment(ctx context.Context, amount uint64, recipientTokenAccount string, tokenIndex uint8) (string, error) {
	authority := e.wallet.PublicKey()
	vaultPDA, _, err := e.DeriveVaultPDA(authority)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}

	vs, err := e.FetchVaultState(ctx, vaultPDA)
	if err != nil {
		return "", fmt.Errorf("fetch vault state: %w", err)
	}
	if int(tokenIndex) >= MaxTokens {
		return "", fmt.Errorf("token_index %d out of range", tokenIndex)
	}

	vaultTokenAccount := solana.PublicKeyFromBytes(vs.VaultTokens[tokenIndex][:])
	recipientPK, err := solana.PublicKeyFromBase58(recipientTokenAccount)
	if err != nil {
		return "", fmt.Errorf("invalid recipient: %w", err)
	}

	disc := anchorDiscriminator("send_payment")
	// token_index(1) + amount(8)
	data := make([]byte, 8+1+8)
	copy(data[0:8], disc)
	data[8] = tokenIndex
	binary.LittleEndian.PutUint64(data[9:], amount)

	tokenProgram := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	return e.sendSimpleTx(ctx, data, []*solana.AccountMeta{
		{PublicKey: authority, IsSigner: true, IsWritable: true},
		{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
		{PublicKey: vaultTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: recipientPK, IsSigner: false, IsWritable: true},
		{PublicKey: tokenProgram, IsSigner: false, IsWritable: false},
	})
}

// SendEmergencyWithdraw submits an emergency_withdraw instruction on-chain.
// authorityTokenAccounts must be in the same index order as the registered vault tokens.
func (e *Executor) SendEmergencyWithdraw(ctx context.Context, authorityTokenAccounts []string) (string, error) {
	authority := e.wallet.PublicKey()
	vaultPDA, _, err := e.DeriveVaultPDA(authority)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}

	vs, err := e.FetchVaultState(ctx, vaultPDA)
	if err != nil {
		return "", fmt.Errorf("fetch vault state: %w", err)
	}

	n := int(vs.NumTokens)
	if len(authorityTokenAccounts) < n {
		return "", fmt.Errorf("need %d authority token accounts, got %d", n, len(authorityTokenAccounts))
	}

	tokenProgram := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")

	// Fixed accounts: authority, vault, token_program
	accounts := []*solana.AccountMeta{
		{PublicKey: authority, IsSigner: true, IsWritable: true},
		{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
		{PublicKey: tokenProgram, IsSigner: false, IsWritable: false},
	}

	// remaining_accounts[0..n) = vault token accounts
	for i := 0; i < n; i++ {
		vtPK := solana.PublicKeyFromBytes(vs.VaultTokens[i][:])
		accounts = append(accounts, &solana.AccountMeta{PublicKey: vtPK, IsSigner: false, IsWritable: true})
	}
	// remaining_accounts[n..2n) = authority token accounts
	for i := 0; i < n; i++ {
		authPK, err := solana.PublicKeyFromBase58(authorityTokenAccounts[i])
		if err != nil {
			return "", fmt.Errorf("invalid authority_token[%d]: %w", i, err)
		}
		accounts = append(accounts, &solana.AccountMeta{PublicKey: authPK, IsSigner: false, IsWritable: true})
	}

	disc := anchorDiscriminator("emergency_withdraw")
	return e.sendSimpleTx(ctx, disc, accounts)
}

// sendSimpleTx builds, signs, and sends a transaction with the given instruction.
func (e *Executor) sendSimpleTx(ctx context.Context, data []byte, accounts []*solana.AccountMeta) (string, error) {
	authority := e.wallet.PublicKey()

	instruction := &solana.GenericInstruction{
		ProgID:        e.programID,
		AccountValues: accounts,
		DataBytes:     data,
	}

	recent, err := e.rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("get blockhash: %w", err)
	}

	tx, err := solana.NewTransaction(
		[]solana.Instruction{instruction},
		recent.Value.Blockhash,
		solana.TransactionPayer(authority),
	)
	if err != nil {
		return "", fmt.Errorf("build tx: %w", err)
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(authority) {
			return &e.wallet
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("sign tx: %w", err)
	}

	sig, err := e.rpcClient.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{
		SkipPreflight:       false,
		PreflightCommitment: rpc.CommitmentConfirmed, // use confirmed not finalized so recent on-chain state is visible
	})
	if err != nil {
		return "", fmt.Errorf("send tx: %w", err)
	}

	return sig.String(), nil
}

// WalletAddress returns the executor's public key.
func (e *Executor) WalletAddress() solana.PublicKey {
	return e.wallet.PublicKey()
}

// anchorDiscriminator computes the 8-byte instruction discriminator.
// Anchor uses sha256("global:<instruction_name>")[0:8].
func anchorDiscriminator(name string) []byte {
	h := sha256.Sum256([]byte("global:" + name))
	return h[:8]
}

// DeriveDecisionPDA derives the PDA for a specific decision log entry.
func (e *Executor) DeriveDecisionPDA(vaultPDA solana.PublicKey, decisionCount uint64) (solana.PublicKey, uint8, error) {
	seq := make([]byte, 8)
	binary.LittleEndian.PutUint64(seq, decisionCount)
	return solana.FindProgramAddress(
		[][]byte{[]byte("decision"), vaultPDA.Bytes(), seq},
		e.programID,
	)
}

// SendRecordDecision submits a record_decision instruction on-chain.
// It fetches the current decision_count from vault state to derive the correct PDA.
// If the PDA slot is already occupied (prior session state mismatch), it retries
// with incremented counts up to 5 times to find an empty slot.
func (e *Executor) SendRecordDecision(ctx context.Context, action uint8, rationale string, confidence uint8) (string, error) {
	// Serialize all record_decision calls: pipeline loop + manual demo button
	// both write to the same sequential PDA slot. Without this lock they race,
	// each reading the same decision_count and then colliding on-chain.
	e.recordDecisionMu.Lock()
	defer e.recordDecisionMu.Unlock()

	authority := e.wallet.PublicKey()
	vaultPDA, _, err := e.DeriveVaultPDA(authority)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}

	vs, err := e.FetchVaultState(ctx, vaultPDA)
	if err != nil {
		return "", fmt.Errorf("fetch vault state: %w", err)
	}

	// Borsh serialization: action(u8) + rationale(u32 len + bytes) + confidence(u8)
	rationaleBytes := []byte(rationale)
	if len(rationaleBytes) > 200 {
		rationaleBytes = rationaleBytes[:200]
	}
	disc := anchorDiscriminator("record_decision")
	data := make([]byte, 8+1+4+len(rationaleBytes)+1)
	copy(data[0:8], disc)
	data[8] = action
	binary.LittleEndian.PutUint32(data[9:], uint32(len(rationaleBytes)))
	copy(data[13:], rationaleBytes)
	data[13+len(rationaleBytes)] = confidence

	// Retry loop: re-fetch vault state on ConstraintSeeds (race condition where
	// another record_decision incremented decision_count between our read and TX).
	// Also skip already-occupied slots by incrementing the counter.
	currentCount := vs.DecisionCount
	for attempt := 0; attempt < 50; attempt++ {
		decisionPDA, _, pdaErr := e.DeriveDecisionPDA(vaultPDA, currentCount)
		if pdaErr != nil {
			return "", fmt.Errorf("derive decision pda: %w", pdaErr)
		}
		sig, txErr := e.sendSimpleTx(ctx, data, []*solana.AccountMeta{
			{PublicKey: authority, IsSigner: true, IsWritable: true},
			{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
			{PublicKey: decisionPDA, IsSigner: false, IsWritable: true},
			{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
		})
		if txErr == nil {
			return sig, nil
		}
		errStr := txErr.Error()
		// PDA slot already used — advance to next sequence number
		if strings.Contains(errStr, "already in use") {
			currentCount++
			continue
		}
		// ConstraintSeeds (0x7d6): on-chain decision_count diverged from our read
		// because another goroutine wrote a record_decision in between.
		// Re-fetch the vault to get the true current count and retry.
		if strings.Contains(errStr, "ConstraintSeeds") || strings.Contains(errStr, "0x7d6") {
			if fresh, fetchErr := e.FetchVaultState(ctx, vaultPDA); fetchErr == nil {
				currentCount = fresh.DecisionCount
			} else {
				currentCount++
			}
			continue
		}
		return "", txErr
	}
	return "", fmt.Errorf("record_decision: failed after 50 attempts (decision_count=%d)", currentCount)
}

// PubkeyToBase58 converts a raw [32]byte public key to its base58 string.
func PubkeyToBase58(b [32]byte) string {
	return solana.PublicKeyFromBytes(b[:]).String()
}

// ParsePublicKey parses a base58-encoded Solana public key.
func ParsePublicKey(s string) (solana.PublicKey, error) {
	return solana.PublicKeyFromBase58(s)
}

func loadWallet(path string) (solana.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var keyBytes []byte
	if err := json.Unmarshal(data, &keyBytes); err != nil {
		// Try as raw base58 string
		return solana.PrivateKeyFromBase58(string(data))
	}

	return solana.PrivateKey(keyBytes), nil
}

// SetAuthorityPK overrides the authority public key used for PDA derivation.
// Use this when the wallet is the delegated agent (not the vault authority).
func (e *Executor) SetAuthorityPK(pk solana.PublicKey) {
	e.authorityPK = pk
}

// SendAdminResetDecisionCount sets vault.decision_count to newCount.
// Use once to fix a stuck PDA slot (e.g. after vault re-init left count at 0 but PDA 0 is occupied).
func (e *Executor) SendAdminResetDecisionCount(ctx context.Context, newCount uint64) (string, error) {
	authority := e.wallet.PublicKey()
	vaultPDA, _, err := e.DeriveVaultPDA(authority)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}
	disc := anchorDiscriminator("admin_reset_decision_count")
	data := make([]byte, 8+8)
	copy(data[0:8], disc)
	binary.LittleEndian.PutUint64(data[8:], newCount)
	return e.sendSimpleTx(ctx, data, []*solana.AccountMeta{
		{PublicKey: authority, IsSigner: true, IsWritable: true},
		{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
	})
}

// SendDelegateAgent submits a delegate_agent instruction on-chain.
// Only the vault authority can call this.
func (e *Executor) SendDelegateAgent(ctx context.Context, agentPubkey solana.PublicKey) (string, error) {
	authority := e.wallet.PublicKey()
	vaultPDA, _, err := e.DeriveVaultPDA(authority)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}

	disc := anchorDiscriminator("delegate_agent")
	// agent_pubkey is a Pubkey (32 bytes)
	data := make([]byte, 8+32)
	copy(data[0:8], disc)
	copy(data[8:], agentPubkey.Bytes())

	return e.sendSimpleTx(ctx, data, []*solana.AccountMeta{
		{PublicKey: authority, IsSigner: true, IsWritable: true},
		{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
	})
}

// SendUpdatePriceAndCheck submits an update_price_and_check instruction on-chain.
// Can be called by either the vault authority or the delegated agent.
// The vault PDA is derived from authorityPK (not necessarily the wallet).
func (e *Executor) SendUpdatePriceAndCheck(ctx context.Context, price uint64) (string, error) {
	signer := e.wallet.PublicKey()
	vaultPDA, _, err := e.DeriveVaultPDA(e.authorityPK)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}

	disc := anchorDiscriminator("update_price_and_check")
	data := make([]byte, 8+8)
	copy(data[0:8], disc)
	binary.LittleEndian.PutUint64(data[8:], price)

	return e.sendSimpleTx(ctx, data, []*solana.AccountMeta{
		{PublicKey: signer, IsSigner: true, IsWritable: true},
		{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
	})
}

// SendDeposit submits a treasury deposit from an authority-owned strategy account back into the vault.
func (e *Executor) SendDeposit(ctx context.Context, amount uint64, authorityTokenAccount string, tokenIndex uint8) (string, error) {
	authority := e.wallet.PublicKey()
	vaultPDA, _, err := e.DeriveVaultPDA(e.authorityPK)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}

	vs, err := e.FetchVaultState(ctx, vaultPDA)
	if err != nil {
		return "", fmt.Errorf("fetch vault state: %w", err)
	}
	if int(tokenIndex) >= MaxTokens {
		return "", fmt.Errorf("token_index %d out of range", tokenIndex)
	}

	authorityTokenPK, err := solana.PublicKeyFromBase58(authorityTokenAccount)
	if err != nil {
		return "", fmt.Errorf("invalid authority token account: %w", err)
	}
	vaultTokenAccount := solana.PublicKeyFromBytes(vs.VaultTokens[tokenIndex][:])

	disc := anchorDiscriminator("deposit")
	data := make([]byte, 8+1+8)
	copy(data[0:8], disc)
	data[8] = tokenIndex
	binary.LittleEndian.PutUint64(data[9:], amount)

	tokenProgram := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	return e.sendSimpleTx(ctx, data, []*solana.AccountMeta{
		{PublicKey: authority, IsSigner: true, IsWritable: true},
		{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
		{PublicKey: authorityTokenPK, IsSigner: false, IsWritable: true},
		{PublicKey: vaultTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: tokenProgram, IsSigner: false, IsWritable: false},
	})
}

// SendExternalTransaction signs an externally-built base64 transaction with the executor wallet and submits it.
func (e *Executor) SendExternalTransaction(ctx context.Context, txBase64 string) (string, error) {
	tx, err := solana.TransactionFromBase64(txBase64)
	if err != nil {
		return "", fmt.Errorf("decode external tx: %w", err)
	}

	signer := e.wallet.PublicKey()
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(signer) {
			return &e.wallet
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("sign external tx: %w", err)
	}

	sig, err := e.rpcClient.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{
		SkipPreflight:       false,
		PreflightCommitment: rpc.CommitmentConfirmed, // use confirmed not finalized so recent on-chain state is visible
	})
	if err != nil {
		return "", fmt.Errorf("send external tx: %w", err)
	}
	return sig.String(), nil
}

// SimulateExternalTransaction signs an externally-built base64 transaction with the executor wallet and runs RPC simulation.
func (e *Executor) SimulateExternalTransaction(ctx context.Context, txBase64 string) (*rpc.SimulateTransactionResult, error) {
	tx, err := solana.TransactionFromBase64(txBase64)
	if err != nil {
		return nil, fmt.Errorf("decode external tx: %w", err)
	}

	signer := e.wallet.PublicKey()
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(signer) {
			return &e.wallet
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("sign external tx for simulation: %w", err)
	}

	out, err := e.rpcClient.SimulateTransactionWithOpts(ctx, tx, &rpc.SimulateTransactionOpts{
		SigVerify:  true,
		Commitment: rpc.CommitmentProcessed,
	})
	if err != nil {
		return nil, fmt.Errorf("simulate external tx: %w", err)
	}
	if out == nil || out.Value == nil {
		return nil, fmt.Errorf("empty simulation response")
	}
	return out.Value, nil
}

// WaitForSignatureConfirmation polls RPC until a signature reaches the requested confirmation status.
func (e *Executor) WaitForSignatureConfirmation(
	ctx context.Context,
	sig string,
	timeout time.Duration,
	required rpc.ConfirmationStatusType,
) (*rpc.SignatureStatusesResult, error) {
	parsed, err := solana.SignatureFromBase58(sig)
	if err != nil {
		return nil, fmt.Errorf("parse signature: %w", err)
	}

	waitCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		out, err := e.rpcClient.GetSignatureStatuses(waitCtx, true, parsed)
		if err != nil {
			return nil, fmt.Errorf("get signature status: %w", err)
		}
		if out != nil && len(out.Value) > 0 && out.Value[0] != nil {
			status := out.Value[0]
			if status.Err != nil {
				return status, fmt.Errorf("transaction execution failed: %v", status.Err)
			}
			if confirmationSatisfied(status, required) {
				return status, nil
			}
		}

		select {
		case <-waitCtx.Done():
			return nil, fmt.Errorf("confirmation timeout: %w", waitCtx.Err())
		case <-ticker.C:
		}
	}
}

func confirmationSatisfied(status *rpc.SignatureStatusesResult, required rpc.ConfirmationStatusType) bool {
	if status == nil {
		return false
	}
	switch required {
	case rpc.ConfirmationStatusFinalized:
		return status.ConfirmationStatus == rpc.ConfirmationStatusFinalized
	case rpc.ConfirmationStatusConfirmed:
		return status.ConfirmationStatus == rpc.ConfirmationStatusConfirmed || status.ConfirmationStatus == rpc.ConfirmationStatusFinalized
	default:
		return status.ConfirmationStatus == rpc.ConfirmationStatusProcessed ||
			status.ConfirmationStatus == rpc.ConfirmationStatusConfirmed ||
			status.ConfirmationStatus == rpc.ConfirmationStatusFinalized
	}
}

// TokenAccountBalance returns the raw amount in base units for the given SPL token account.
func (e *Executor) TokenAccountBalance(ctx context.Context, account string) (uint64, error) {
	pk, err := solana.PublicKeyFromBase58(account)
	if err != nil {
		return 0, fmt.Errorf("invalid token account: %w", err)
	}
	out, err := e.rpcClient.GetTokenAccountBalance(ctx, pk, rpc.CommitmentConfirmed)
	if err != nil {
		return 0, fmt.Errorf("get token account balance: %w", err)
	}
	if out == nil || out.Value == nil {
		return 0, fmt.Errorf("empty token account balance response")
	}
	raw := out.Value.Amount
	if raw == "" {
		return 0, nil
	}
	amt, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse token account balance: %w", err)
	}
	return amt, nil
}

// SendRecordSwapResult submits a record_swap_result instruction on-chain.
// Called after a successful external Jupiter swap to settle vault accounting.
// This provides a verifiable on-chain receipt of the rebalance:
//
//	execute_rebalance (intent) → external swap TX → record_swap_result (settlement)
func (e *Executor) SendRecordSwapResult(ctx context.Context, fromIndex, toIndex uint8, inputAmount, outputAmount uint64, swapSig string) (string, error) {
	signer := e.wallet.PublicKey()
	vaultPDA, _, err := e.DeriveVaultPDA(e.authorityPK)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}

	// Borsh: from_index(1) + to_index(1) + input_amount(8) + output_amount(8) + swap_sig len(4) + swap_sig bytes
	sigBytes := []byte(swapSig)
	if len(sigBytes) > 88 {
		sigBytes = sigBytes[:88]
	}
	disc := anchorDiscriminator("record_swap_result")
	data := make([]byte, 8+1+1+8+8+4+len(sigBytes))
	copy(data[0:8], disc)
	data[8] = fromIndex
	data[9] = toIndex
	binary.LittleEndian.PutUint64(data[10:], inputAmount)
	binary.LittleEndian.PutUint64(data[18:], outputAmount)
	binary.LittleEndian.PutUint32(data[26:], uint32(len(sigBytes)))
	copy(data[30:], sigBytes)

	return e.sendSimpleTx(ctx, data, []*solana.AccountMeta{
		{PublicKey: signer, IsSigner: true, IsWritable: true},
		{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
	})
}

// SendTogglePause submits a toggle_pause instruction on-chain.
// MintTokensTo mints `amount` tokens (in base units) to `destATA`.
// Requires the executor wallet to be the mint authority of `mintAddr`.
// Used on devnet as a swap fallback when Jupiter is unavailable.
func (e *Executor) MintTokensTo(ctx context.Context, mintAddr, destATA string, amount uint64) (string, error) {
	mintPK, err := solana.PublicKeyFromBase58(mintAddr)
	if err != nil {
		return "", fmt.Errorf("invalid mint: %w", err)
	}
	destPK, err := solana.PublicKeyFromBase58(destATA)
	if err != nil {
		return "", fmt.Errorf("invalid dest ATA: %w", err)
	}
	authority := e.wallet.PublicKey()
	tokenProgram := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")

	// SPL Token MintTo instruction: index=7, amount u64 LE
	data := make([]byte, 9)
	data[0] = 7
	binary.LittleEndian.PutUint64(data[1:], amount)

	instr := &solana.GenericInstruction{
		ProgID: tokenProgram,
		AccountValues: []*solana.AccountMeta{
			{PublicKey: mintPK, IsSigner: false, IsWritable: true},
			{PublicKey: destPK, IsSigner: false, IsWritable: true},
			{PublicKey: authority, IsSigner: true, IsWritable: false},
		},
		DataBytes: data,
	}

	recent, err := e.rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("get blockhash: %w", err)
	}
	tx, err := solana.NewTransaction([]solana.Instruction{instr}, recent.Value.Blockhash, solana.TransactionPayer(authority))
	if err != nil {
		return "", fmt.Errorf("build mint tx: %w", err)
	}
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(authority) {
			return &e.wallet
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("sign mint tx: %w", err)
	}
	sig, err := e.rpcClient.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{
		SkipPreflight:       false,
		PreflightCommitment: rpc.CommitmentConfirmed,
	})
	if err != nil {
		return "", fmt.Errorf("send mint tx: %w", err)
	}
	return sig.String(), nil
}

func (e *Executor) SendTogglePause(ctx context.Context) (string, error) {
	authority := e.wallet.PublicKey()
	vaultPDA, _, err := e.DeriveVaultPDA(authority)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}
	disc := anchorDiscriminator("toggle_pause")
	return e.sendSimpleTx(ctx, disc, []*solana.AccountMeta{
		{PublicKey: authority, IsSigner: true, IsWritable: true},
		{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
	})
}

// SendSetDemoBalances calls set_demo_balances on devnet to write accounting balances
// directly into the vault without real SPL transfers. Demo/devnet only.
func (e *Executor) SendSetDemoBalances(ctx context.Context, balances []uint64, totalDeposited uint64) (string, error) {
	authority := e.wallet.PublicKey()
	vaultPDA, _, err := e.DeriveVaultPDA(authority)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}

	disc := anchorDiscriminator("set_demo_balances")

	// Borsh-encode: Vec<u64> = 4-byte length prefix + 8 bytes per element, then u64 total_deposited
	n := len(balances)
	data := make([]byte, 8+4+n*8+8)
	copy(data[0:8], disc)
	binary.LittleEndian.PutUint32(data[8:12], uint32(n))
	for i, b := range balances {
		binary.LittleEndian.PutUint64(data[12+i*8:], b)
	}
	binary.LittleEndian.PutUint64(data[12+n*8:], totalDeposited)

	return e.sendSimpleTx(ctx, data, []*solana.AccountMeta{
		{PublicKey: authority, IsSigner: true, IsWritable: true},
		{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
	})
}
