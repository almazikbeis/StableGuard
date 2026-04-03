package yield

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const kaminoTxBaseURL = "https://api.kamino.finance"

type kaminoTxClient struct {
	http *http.Client
}

type kaminoTxResponse struct {
	Transaction string `json:"transaction"`
}

type kaminoTxRequest struct {
	Wallet string `json:"wallet"`
	KVault string `json:"kvault"`
	Amount string `json:"amount"`
}

func newKaminoTxClient() *kaminoTxClient {
	return &kaminoTxClient{
		http: &http.Client{Timeout: 12 * time.Second},
	}
}

func (c *kaminoTxClient) DepositTx(ctx context.Context, wallet, kvault, amount string) (string, error) {
	return c.buildTx(ctx, "/ktx/kvault/deposit", wallet, kvault, amount)
}

func (c *kaminoTxClient) WithdrawTx(ctx context.Context, wallet, kvault, amount string) (string, error) {
	return c.buildTx(ctx, "/ktx/kvault/withdraw", wallet, kvault, amount)
}

func BuildKaminoDepositTx(ctx context.Context, wallet, kvault, amount string) (string, error) {
	return newKaminoTxClient().DepositTx(ctx, wallet, kvault, amount)
}

func BuildKaminoWithdrawTx(ctx context.Context, wallet, kvault, amount string) (string, error) {
	return newKaminoTxClient().WithdrawTx(ctx, wallet, kvault, amount)
}

func (c *kaminoTxClient) buildTx(ctx context.Context, path, wallet, kvault, amount string) (string, error) {
	body, err := json.Marshal(kaminoTxRequest{
		Wallet: wallet,
		KVault: kvault,
		Amount: amount,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, kaminoTxBaseURL+path, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("kamino tx API %d: %s", resp.StatusCode, string(raw))
	}

	var out kaminoTxResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.Transaction == "" {
		return "", fmt.Errorf("kamino tx API returned empty transaction")
	}
	return out.Transaction, nil
}
