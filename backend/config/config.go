package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AnthropicAPIKey string
	SolanaRPCURL    string
	ProgramID       string
	WalletKeyPath   string
	Port            string
	PythHermesURL   string
	// Devnet mint addresses (set after vault is initialized)
	MintA    string
	MintB    string
	VaultPDA string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	return &Config{
		AnthropicAPIKey: mustGet("ANTHROPIC_API_KEY"),
		SolanaRPCURL:    getOrDefault("SOLANA_RPC_URL", "https://api.devnet.solana.com"),
		ProgramID:       getOrDefault("PROGRAM_ID", "GPSJJqicuDSJ6LXhEZpmUboThjzdefG5wZAZkL2hd7es"),
		WalletKeyPath:   getOrDefault("WALLET_KEY_PATH", os.Getenv("HOME")+"/.config/solana/devnet.json"),
		Port:            getOrDefault("PORT", "8080"),
		PythHermesURL:   getOrDefault("PYTH_HERMES_URL", "https://hermes.pyth.network"),
		MintA:           os.Getenv("MINT_A"),
		MintB:           os.Getenv("MINT_B"),
		VaultPDA:        os.Getenv("VAULT_PDA"),
	}
}

func mustGet(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Printf("WARNING: required env var %s not set", key)
	}
	return v
}

func getOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
