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

// Executor submits on-chain transactions to the StableGuard program.
type Executor struct {
	rpcClient *rpc.Client
	wallet    solana.PrivateKey
	programID solana.PublicKey
}

// VaultState mirrors the on-chain VaultState account fields we care about.
type VaultState struct {
	Authority          [32]byte
	MintA              [32]byte
	MintB              [32]byte
	VaultTokenA        [32]byte
	VaultTokenB        [32]byte
	TotalDeposited     uint64
	BalanceA           uint64
	BalanceB           uint64
	RebalanceThreshold uint64
	MaxDeposit         uint64
	DecisionCount      uint64
	TotalRebalances    uint64
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

// ExecuteRebalance submits an execute_rebalance instruction on-chain.
// direction: 0 = A→B, 1 = B→A
func (e *Executor) ExecuteRebalance(ctx context.Context, direction uint8, amount uint64) (string, error) {
	authority := e.wallet.PublicKey()

	vaultPDA, _, err := e.DeriveVaultPDA(authority)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}

	// Build instruction data: 8-byte discriminator + direction (u8) + amount (u64 le)
	discriminator := anchorDiscriminator("execute_rebalance")
	data := make([]byte, 8+1+8)
	copy(data[0:8], discriminator)
	data[8] = direction
	binary.LittleEndian.PutUint64(data[9:], amount)

	accounts := []*solana.AccountMeta{
		{PublicKey: authority, IsSigner: true, IsWritable: true},
		{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
	}

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
	minLen := 8 + 32*5 + 8*7 + 1 + 1 // discriminator + 5 pubkeys + 7 u64s + bool + bump
	if len(data) < minLen {
		return nil, fmt.Errorf("vault account data too short: %d bytes (need %d)", len(data), minLen)
	}

	offset := 8 // skip 8-byte Anchor discriminator
	vs := &VaultState{}

	copy(vs.Authority[:], data[offset:offset+32]); offset += 32
	copy(vs.MintA[:], data[offset:offset+32]); offset += 32
	copy(vs.MintB[:], data[offset:offset+32]); offset += 32
	copy(vs.VaultTokenA[:], data[offset:offset+32]); offset += 32
	copy(vs.VaultTokenB[:], data[offset:offset+32]); offset += 32

	vs.TotalDeposited = binary.LittleEndian.Uint64(data[offset:]); offset += 8
	vs.BalanceA = binary.LittleEndian.Uint64(data[offset:]); offset += 8
	vs.BalanceB = binary.LittleEndian.Uint64(data[offset:]); offset += 8
	vs.RebalanceThreshold = binary.LittleEndian.Uint64(data[offset:]); offset += 8
	vs.MaxDeposit = binary.LittleEndian.Uint64(data[offset:]); offset += 8
	vs.DecisionCount = binary.LittleEndian.Uint64(data[offset:]); offset += 8
	vs.TotalRebalances = binary.LittleEndian.Uint64(data[offset:]); offset += 8

	vs.IsPaused = data[offset] != 0; offset++
	vs.StrategyMode = data[offset]; offset++
	vs.Bump = data[offset]

	return vs, nil
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
func (e *Executor) SendPayment(ctx context.Context, amount uint64, recipientTokenAccount string, isTokenA bool) (string, error) {
	authority := e.wallet.PublicKey()
	vaultPDA, _, err := e.DeriveVaultPDA(authority)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}

	vs, err := e.FetchVaultState(ctx, vaultPDA)
	if err != nil {
		return "", fmt.Errorf("fetch vault state: %w", err)
	}

	var vaultTokenAccount solana.PublicKey
	if isTokenA {
		vaultTokenAccount = solana.PublicKeyFromBytes(vs.VaultTokenA[:])
	} else {
		vaultTokenAccount = solana.PublicKeyFromBytes(vs.VaultTokenB[:])
	}

	recipientPK, err := solana.PublicKeyFromBase58(recipientTokenAccount)
	if err != nil {
		return "", fmt.Errorf("invalid recipient: %w", err)
	}

	disc := anchorDiscriminator("send_payment")
	data := make([]byte, 8+8+1)
	copy(data[0:8], disc)
	binary.LittleEndian.PutUint64(data[8:], amount)
	if isTokenA {
		data[16] = 1
	}

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
func (e *Executor) SendEmergencyWithdraw(ctx context.Context, authorityTokenA, authorityTokenB string) (string, error) {
	authority := e.wallet.PublicKey()
	vaultPDA, _, err := e.DeriveVaultPDA(authority)
	if err != nil {
		return "", fmt.Errorf("derive vault pda: %w", err)
	}

	vs, err := e.FetchVaultState(ctx, vaultPDA)
	if err != nil {
		return "", fmt.Errorf("fetch vault state: %w", err)
	}

	authTokenAPK, err := solana.PublicKeyFromBase58(authorityTokenA)
	if err != nil {
		return "", fmt.Errorf("invalid authority_token_a: %w", err)
	}
	authTokenBPK, err := solana.PublicKeyFromBase58(authorityTokenB)
	if err != nil {
		return "", fmt.Errorf("invalid authority_token_b: %w", err)
	}

	vaultTokenAPK := solana.PublicKeyFromBytes(vs.VaultTokenA[:])
	vaultTokenBPK := solana.PublicKeyFromBytes(vs.VaultTokenB[:])
	tokenProgram := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")

	disc := anchorDiscriminator("emergency_withdraw")

	return e.sendSimpleTx(ctx, disc, []*solana.AccountMeta{
		{PublicKey: authority, IsSigner: true, IsWritable: true},
		{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
		{PublicKey: vaultTokenAPK, IsSigner: false, IsWritable: true},
		{PublicKey: vaultTokenBPK, IsSigner: false, IsWritable: true},
		{PublicKey: authTokenAPK, IsSigner: false, IsWritable: true},
		{PublicKey: authTokenBPK, IsSigner: false, IsWritable: true},
		{PublicKey: tokenProgram, IsSigner: false, IsWritable: false},
	})
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
