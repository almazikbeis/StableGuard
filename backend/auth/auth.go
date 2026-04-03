// Package auth provides JWT-based authentication for StableGuard.
package auth

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mr-tron/base58"
	"golang.org/x/crypto/bcrypt"
)

var secret = func() []byte {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return []byte(s)
	}
	return []byte("stableguard-dev-secret-change-in-prod")
}()

// Claims is the JWT payload.
type Claims struct {
	UserID int64  `json:"uid"`
	Email  string `json:"email"`
	Exp    int64  `json:"exp"`
}

// GenerateToken creates a signed HS256 JWT for the given user.
func GenerateToken(userID int64, email string) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

	claims := Claims{
		UserID: userID,
		Email:  email,
		Exp:    time.Now().Add(30 * 24 * time.Hour).Unix(), // 30 days
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)

	sigInput := header + "." + encodedPayload
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(sigInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return sigInput + "." + sig, nil
}

// ValidateToken parses and verifies a JWT, returning the claims on success.
func ValidateToken(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	sigInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(sigInput))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, errors.New("invalid signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal claims: %w", err)
	}

	if time.Now().Unix() > claims.Exp {
		return nil, errors.New("token expired")
	}

	return &claims, nil
}

// HashPassword bcrypts a plaintext password.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword compares a plaintext password against a bcrypt hash.
func CheckPassword(plain, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// WalletClaims is the JWT payload for wallet-based authentication.
type WalletClaims struct {
	Wallet string `json:"wallet"`
	Exp    int64  `json:"exp"`
}

// GenerateWalletToken creates a signed HS256 JWT for the given Solana wallet address.
func GenerateWalletToken(walletAddress string) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

	claims := WalletClaims{
		Wallet: walletAddress,
		Exp:    time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)

	sigInput := header + "." + encodedPayload
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(sigInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return sigInput + "." + sig, nil
}

// ValidateWalletToken parses and verifies a wallet JWT, returning the wallet address.
func ValidateWalletToken(token string) (*WalletClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	sigInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(sigInput))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, errors.New("invalid signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	var claims WalletClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal claims: %w", err)
	}

	if time.Now().Unix() > claims.Exp {
		return nil, errors.New("token expired")
	}

	return &claims, nil
}

// VerifyWalletSignature verifies a Solana ed25519 signature.
// address is the base58-encoded public key, signature is base64-encoded.
// The message must start with "StableGuard login: " to prevent replay attacks.
func VerifyWalletSignature(address, message, signature string) error {
	if !strings.HasPrefix(message, "StableGuard login: ") {
		return errors.New("invalid message prefix")
	}

	pubkeyBytes, err := base58.Decode(address)
	if err != nil {
		return fmt.Errorf("decode address: %w", err)
	}
	if len(pubkeyBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid pubkey length: %d", len(pubkeyBytes))
	}

	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		// try RawStdEncoding as some wallets omit padding
		sigBytes, err = base64.RawStdEncoding.DecodeString(signature)
		if err != nil {
			return fmt.Errorf("decode signature: %w", err)
		}
	}

	pubkey := ed25519.PublicKey(pubkeyBytes)
	if !ed25519.Verify(pubkey, []byte(message), sigBytes) {
		return errors.New("signature verification failed")
	}

	return nil
}
