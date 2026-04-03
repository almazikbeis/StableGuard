package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AnthropicAPIKey string
	SolanaRPCURL    string
	ProgramID       string
	WalletKeyPath   string
	Port            string
	PythHermesURL   string

	// Multi-token vault mints (registered via register_token on-chain)
	MintA    string
	MintB    string
	VaultPDA string

	// ── Pipeline settings ─────────────────────────────────────────────────
	// StrategyMode: 0=SAFE, 1=BALANCED (default), 2=YIELD
	StrategyMode uint8
	// AutoExecute: if true, the pipeline executes rebalances automatically
	AutoExecute bool
	// AIIntervalSec: minimum seconds between AI agent runs (default 30)
	AIIntervalSec int
	// StreamPollFallbackSec: polling interval when SSE is unavailable (default 5)
	StreamPollFallbackSec int
	// DBPath: path to the SQLite database file (default stableguard.db)
	DBPath string

	// ── Alerts ────────────────────────────────────────────────────────────
	TelegramBotToken  string
	TelegramChatID    string
	DiscordWebhookURL string
	// AlertCooldownSec: minimum seconds between same alert type (default 300)
	AlertCooldownSec int
	// AlertRiskThreshold: risk level that triggers an alert (default 80)
	AlertRiskThreshold float64

	// ── Circuit Breaker ───────────────────────────────────────────────────
	// CircuitBreakerEnabled: enables automatic vault pause on depeg
	CircuitBreakerEnabled bool
	// CircuitBreakerPausePct: deviation % that triggers vault pause (default 1.5)
	CircuitBreakerPausePct float64
	// CircuitBreakerEmergencyPct: deviation % that triggers critical alert (default 3.0)
	CircuitBreakerEmergencyPct float64

	// ── Hot Path / Agent ──────────────────────────────────────────────────
	// HotPathEnabled: if true, pipeline calls update_price_and_check on-chain every tick
	HotPathEnabled bool
	// AgentPubkey: base58 pubkey of the delegated agent wallet (may equal WalletKeyPath pubkey)
	AgentPubkey string
	// VaultLUTAddress: pre-created address lookup table address for v0 transactions
	VaultLUTAddress string

	// ── Yield Optimizer ──────────────────────────────────────────────────
	// YieldEnabled: enables automatic yield deposits on OPTIMIZE signal
	YieldEnabled bool
	// YieldMinAPY: minimum APY % required to deposit (default 4.0)
	YieldMinAPY float64
	// YieldEntryRisk: max risk score to enter a yield position (default 35)
	YieldEntryRisk float64
	// YieldExitRisk: risk score above which to withdraw from yield (default 55)
	YieldExitRisk float64
	// YieldDepositUSDC: amount of USDC-equivalent to deposit in each cycle (default 1000)
	YieldDepositAmount float64
	// Trusted strategy token accounts used by autonomous YIELD_MAX transfers.
	YieldStrategyUSDCAccount  string
	YieldStrategyUSDTAccount  string
	YieldStrategyDAIAccount   string
	YieldStrategyPYUSDAccount string
	// Trusted Kamino Earn vault addresses used by autonomous YIELD_MAX deposits.
	YieldKaminoUSDCVault  string
	YieldKaminoUSDTVault  string
	YieldKaminoDAIVault   string
	YieldKaminoPYUSDVault string
}

type YieldReadiness struct {
	Mode                string
	MainnetRPC          bool
	MissingStrategyATAs []string
	MissingKaminoVaults []string
	ReadyForLive        bool
	Note                string
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

		StrategyMode:          uint8(getInt("STRATEGY_MODE", 1)),
		AutoExecute:           getBool("AUTO_EXECUTE", false),
		AIIntervalSec:         getInt("AI_INTERVAL_SEC", 30),
		StreamPollFallbackSec: getInt("STREAM_POLL_FALLBACK_SEC", 5),
		DBPath:                getOrDefault("DB_PATH", "stableguard.db"),

		TelegramBotToken:   os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:     os.Getenv("TELEGRAM_CHAT_ID"),
		DiscordWebhookURL:  os.Getenv("DISCORD_WEBHOOK_URL"),
		AlertCooldownSec:   getInt("ALERT_COOLDOWN_SEC", 300),
		AlertRiskThreshold: getFloat("ALERT_RISK_THRESHOLD", 80),

		CircuitBreakerEnabled:      getBool("CIRCUIT_BREAKER_ENABLED", true),
		CircuitBreakerPausePct:     getFloat("CIRCUIT_BREAKER_PAUSE_PCT", 1.5),
		CircuitBreakerEmergencyPct: getFloat("CIRCUIT_BREAKER_EMERGENCY_PCT", 3.0),

		HotPathEnabled:  getBool("HOT_PATH_ENABLED", false),
		AgentPubkey:     os.Getenv("AGENT_PUBKEY"),
		VaultLUTAddress: os.Getenv("VAULT_LUT_ADDRESS"),

		YieldEnabled:              getBool("YIELD_ENABLED", false),
		YieldMinAPY:               getFloat("YIELD_MIN_APY", 4.0),
		YieldEntryRisk:            getFloat("YIELD_ENTRY_RISK", 35.0),
		YieldExitRisk:             getFloat("YIELD_EXIT_RISK", 55.0),
		YieldDepositAmount:        getFloat("YIELD_DEPOSIT_AMOUNT", 1000.0),
		YieldStrategyUSDCAccount:  os.Getenv("YIELD_STRATEGY_USDC_ACCOUNT"),
		YieldStrategyUSDTAccount:  os.Getenv("YIELD_STRATEGY_USDT_ACCOUNT"),
		YieldStrategyDAIAccount:   os.Getenv("YIELD_STRATEGY_DAI_ACCOUNT"),
		YieldStrategyPYUSDAccount: os.Getenv("YIELD_STRATEGY_PYUSD_ACCOUNT"),
		YieldKaminoUSDCVault:      os.Getenv("YIELD_KAMINO_USDC_VAULT"),
		YieldKaminoUSDTVault:      os.Getenv("YIELD_KAMINO_USDT_VAULT"),
		YieldKaminoDAIVault:       os.Getenv("YIELD_KAMINO_DAI_VAULT"),
		YieldKaminoPYUSDVault:     os.Getenv("YIELD_KAMINO_PYUSD_VAULT"),
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

func getInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func getBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		switch v {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
	}
	return def
}

func IsMainnetRPC(rpcURL string) bool {
	u := strings.ToLower(rpcURL)
	return strings.Contains(u, "mainnet") || strings.Contains(u, "helius") || strings.Contains(u, "quiknode") || strings.Contains(u, "alchemy")
}

func (c *Config) YieldExecutionReadiness() YieldReadiness {
	if c == nil || !c.YieldEnabled {
		return YieldReadiness{
			Mode:         "disabled",
			MainnetRPC:   false,
			ReadyForLive: false,
			Note:         "Yield automation is disabled.",
		}
	}

	readiness := YieldReadiness{
		Mode:       "strategy_only",
		MainnetRPC: IsMainnetRPC(c.SolanaRPCURL),
	}

	if c.YieldStrategyUSDCAccount == "" {
		readiness.MissingStrategyATAs = append(readiness.MissingStrategyATAs, "YIELD_STRATEGY_USDC_ACCOUNT")
	}
	if c.YieldStrategyUSDTAccount == "" {
		readiness.MissingStrategyATAs = append(readiness.MissingStrategyATAs, "YIELD_STRATEGY_USDT_ACCOUNT")
	}
	if c.YieldStrategyDAIAccount == "" {
		readiness.MissingStrategyATAs = append(readiness.MissingStrategyATAs, "YIELD_STRATEGY_DAI_ACCOUNT")
	}
	if c.YieldStrategyPYUSDAccount == "" {
		readiness.MissingStrategyATAs = append(readiness.MissingStrategyATAs, "YIELD_STRATEGY_PYUSD_ACCOUNT")
	}

	if c.YieldKaminoUSDCVault == "" {
		readiness.MissingKaminoVaults = append(readiness.MissingKaminoVaults, "YIELD_KAMINO_USDC_VAULT")
	}
	if c.YieldKaminoUSDTVault == "" {
		readiness.MissingKaminoVaults = append(readiness.MissingKaminoVaults, "YIELD_KAMINO_USDT_VAULT")
	}
	if c.YieldKaminoDAIVault == "" {
		readiness.MissingKaminoVaults = append(readiness.MissingKaminoVaults, "YIELD_KAMINO_DAI_VAULT")
	}
	if c.YieldKaminoPYUSDVault == "" {
		readiness.MissingKaminoVaults = append(readiness.MissingKaminoVaults, "YIELD_KAMINO_PYUSD_VAULT")
	}

	switch {
	case !readiness.MainnetRPC:
		readiness.Mode = "strategy_only"
		readiness.Note = "Yield mode can move funds into trusted strategy accounts, but live Kamino execution is blocked until the backend uses a mainnet RPC."
	case len(readiness.MissingStrategyATAs) > 0 || len(readiness.MissingKaminoVaults) > 0:
		readiness.Mode = "strategy_only"
		readiness.Note = "Yield mode is missing trusted strategy accounts or Kamino vault configuration, so live execution is not ready."
	default:
		readiness.Mode = "live"
		readiness.ReadyForLive = true
		readiness.Note = "Yield mode is fully configured for live external execution."
	}

	return readiness
}
