package config

import (
	"log"
	"os"
	"strconv"

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

		TelegramBotToken:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:    os.Getenv("TELEGRAM_CHAT_ID"),
		DiscordWebhookURL: os.Getenv("DISCORD_WEBHOOK_URL"),
		AlertCooldownSec:  getInt("ALERT_COOLDOWN_SEC", 300),
		AlertRiskThreshold: getFloat("ALERT_RISK_THRESHOLD", 80),

		CircuitBreakerEnabled:      getBool("CIRCUIT_BREAKER_ENABLED", true),
		CircuitBreakerPausePct:     getFloat("CIRCUIT_BREAKER_PAUSE_PCT", 1.5),
		CircuitBreakerEmergencyPct: getFloat("CIRCUIT_BREAKER_EMERGENCY_PCT", 3.0),

		YieldEnabled:       getBool("YIELD_ENABLED", false),
		YieldMinAPY:        getFloat("YIELD_MIN_APY", 4.0),
		YieldEntryRisk:     getFloat("YIELD_ENTRY_RISK", 35.0),
		YieldExitRisk:      getFloat("YIELD_EXIT_RISK", 55.0),
		YieldDepositAmount: getFloat("YIELD_DEPOSIT_AMOUNT", 1000.0),
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
