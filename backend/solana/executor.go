// Package solana handles on-chain interaction with the StableGuard program.
package solana

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// ProgramIDStr is the deployed StableGuard program.
const ProgramIDStr = "GPSJJqicuDSJ6LXhEZpmUboThjzdefG5wZAZkL2hd7es"

// MaxTokens mirrors the on-chain constant.
const MaxTokens = 8

// Executor submits on-chain transactions to the StableGuard program.
type Executor struct {
	rpcClient *rpc.Client
	wallet    solana.PrivateKey
	programID solana.PublicKey
}

// VaultState mirrors the on-chain VaultState account layout (660 bytes after discriminator).
type VaultState struct {
	Authority          [32]byte
	Mints              [MaxTokens][32]byte
	VaultTokens        [MaxTokens][32]byte
	TotalDeposited     uint64
	Balances           [MaxTokens]uint64
	RebalanceThreshold uint64
	MaxDeposit         uint64
	DecisionCount      uint64
	TotalRebalances    uint64
	NumTokens          uint8
	IsPaused           bool
	StrategyMode       uint8
	Bump               uint8
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

	return &Executor{
		rpcClient: client,
		wallet:    privKey,
		programID: programID,
	}, nil
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
	acc, err := e.rpcClient.GetAccountInfo(ctx, vaultPDA)
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
	// = 660 bytes total (+ 8 discriminator = 668 raw)
	const minLen = 8 + 32 + 32*MaxTokens + 32*MaxTokens + 8 + 8*MaxTokens + 8 + 8 + 8 + 8 + 1 + 1 + 1 + 1
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

	return vs, nil
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
		SkipPreflight: false,
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
func (e *Executor) SendRecordDecision(ctx context.Context, action uint8, rationale string, confidence uint8) (string, error) {
	authority := e.wallet.PublicKey()
	vaultPDA, _, err := e.DeriveVaultPDA(authority)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}

	vs, err := e.FetchVaultState(ctx, vaultPDA)
	if err != nil {
		return "", fmt.Errorf("fetch vault state: %w", err)
	}

	decisionPDA, _, err := e.DeriveDecisionPDA(vaultPDA, vs.DecisionCount)
	if err != nil {
		return "", fmt.Errorf("derive decision pda: %w", err)
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

	return e.sendSimpleTx(ctx, data, []*solana.AccountMeta{
		{PublicKey: authority, IsSigner: true, IsWritable: true},
		{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
		{PublicKey: decisionPDA, IsSigner: false, IsWritable: true},
		{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
	})
}

// PubkeyToBase58 converts a raw [32]byte public key to its base58 string.
func PubkeyToBase58(b [32]byte) string {
	return solana.PublicKeyFromBytes(b[:]).String()
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

// SendTogglePause submits a toggle_pause instruction on-chain.
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
