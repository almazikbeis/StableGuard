package solana

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// AddressLookupTableProgramID is the Solana address lookup table program.
var AddressLookupTableProgramID = solana.MustPublicKeyFromBase58("AddressLookupTab1e1111111111111111111111111")

// CreateLUT creates an on-chain address lookup table pre-populated with the
// given accounts. Returns the LUT address and transaction signature.
func (e *Executor) CreateLUT(ctx context.Context, accounts []solana.PublicKey) (solana.PublicKey, string, error) {
	authority := e.wallet.PublicKey()

	slot, err := e.rpcClient.GetSlot(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return solana.PublicKey{}, "", fmt.Errorf("get slot: %w", err)
	}

	// Derive LUT address: PDA([authority, slot_le8])
	slotBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(slotBytes, slot)
	lutAddr, nonce, err := solana.FindProgramAddress(
		[][]byte{authority.Bytes(), slotBytes},
		AddressLookupTableProgramID,
	)
	if err != nil {
		return solana.PublicKey{}, "", fmt.Errorf("derive lut pda: %w", err)
	}

	// Instruction 0: CreateLookupTable (discriminator = 0)
	createData := make([]byte, 4+8+1)
	binary.LittleEndian.PutUint32(createData[0:], 0) // discriminator
	binary.LittleEndian.PutUint64(createData[4:], slot)
	createData[12] = nonce

	createIx := &solana.GenericInstruction{
		ProgID: AddressLookupTableProgramID,
		AccountValues: []*solana.AccountMeta{
			{PublicKey: lutAddr, IsSigner: false, IsWritable: true},
			{PublicKey: authority, IsSigner: true, IsWritable: true},
			{PublicKey: authority, IsSigner: true, IsWritable: true}, // payer
			{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
		},
		DataBytes: createData,
	}

	// Instruction 1: ExtendLookupTable (discriminator = 2)
	extendData := make([]byte, 4+4+4+len(accounts)*32)
	binary.LittleEndian.PutUint32(extendData[0:], 2)                      // discriminator
	binary.LittleEndian.PutUint32(extendData[4:], 0)                      // padding
	binary.LittleEndian.PutUint32(extendData[8:], uint32(len(accounts))) // count
	for i, pk := range accounts {
		copy(extendData[12+i*32:], pk.Bytes())
	}

	extendIx := &solana.GenericInstruction{
		ProgID: AddressLookupTableProgramID,
		AccountValues: []*solana.AccountMeta{
			{PublicKey: lutAddr, IsSigner: false, IsWritable: true},
			{PublicKey: authority, IsSigner: true, IsWritable: false},
			{PublicKey: authority, IsSigner: true, IsWritable: true}, // payer
			{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
		},
		DataBytes: extendData,
	}

	sig, err := e.sendMultiIxTx(ctx, []solana.Instruction{createIx, extendIx})
	if err != nil {
		return solana.PublicKey{}, "", fmt.Errorf("create+extend lut: %w", err)
	}

	return lutAddr, sig, nil
}

// InitVaultLUT creates a LUT containing the vault PDA and all registered token PDAs.
// This reduces transaction size for frequent hot-path calls.
func (e *Executor) InitVaultLUT(ctx context.Context) (solana.PublicKey, string, error) {
	authority := e.wallet.PublicKey()
	vaultPDA, _, err := e.DeriveVaultPDA(authority)
	if err != nil {
		return solana.PublicKey{}, "", fmt.Errorf("derive vault: %w", err)
	}

	accounts := []solana.PublicKey{vaultPDA, e.programID}
	for i := 0; i < MaxTokens; i++ {
		tokenPDA, _, err := e.DeriveVaultTokenPDA(vaultPDA, uint8(i))
		if err != nil {
			continue
		}
		accounts = append(accounts, tokenPDA)
	}

	return e.CreateLUT(ctx, accounts)
}

// sendV0Tx builds and sends a v0 (versioned) transaction, optionally using an ALT.
func (e *Executor) sendV0Tx(ctx context.Context, instructions []solana.Instruction, lutAddr *solana.PublicKey) (string, error) {
	authority := e.wallet.PublicKey()

	recent, err := e.rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("get blockhash: %w", err)
	}

	tx, err := solana.NewTransaction(
		instructions,
		recent.Value.Blockhash,
		solana.TransactionPayer(authority),
	)
	if err != nil {
		return "", fmt.Errorf("build tx: %w", err)
	}

	// Upgrade to v0 if we have an ALT
	if lutAddr != nil {
		tx.Message.SetVersion(solana.MessageVersionV0)
		tx.Message.AddAddressTableLookup(solana.MessageAddressTableLookup{
			AccountKey: *lutAddr,
		})
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(authority) {
			return &e.wallet
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("sign v0 tx: %w", err)
	}

	sig, err := e.rpcClient.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{
		SkipPreflight: false,
	})
	if err != nil {
		return "", fmt.Errorf("send v0 tx: %w", err)
	}

	return sig.String(), nil
}

// sendMultiIxTx builds, signs, and sends a transaction with multiple instructions.
func (e *Executor) sendMultiIxTx(ctx context.Context, instructions []solana.Instruction) (string, error) {
	authority := e.wallet.PublicKey()

	recent, err := e.rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("get blockhash: %w", err)
	}

	tx, err := solana.NewTransaction(
		instructions,
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
