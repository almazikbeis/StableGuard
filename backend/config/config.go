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
	OperatorWallets []string
	OperatorEmails  []string

	// Multi-token vault mints (registered via register_token on-chain)
	MintA     string // USDC devnet mint
	MintB     string // USDT devnet mint
	MintBTC   string
	MintETH   string
	MintSOL   string
	MintDAI   string
	MintPYUSD string
	VaultPDA  string

	// ── Pipeline settings ─────────────────────────────────────────────────
	// StrategyMode: 0=SAFE, 1=BALANCED (default), 2=YIELD
	StrategyMode uint8
	// AutoExecute: if true, the pipeline executes rebalances automatically
	AutoExecute bool
	// AIIntervalSec: minimum seconds between AI agent runs (default 30)
	AIIntervalSec int
	// AIAgentModel: Anthropic model identifier used by the decision agents.
	AIAgentModel string
	// AIDecisionProfile: "cautious" | "balanced" | "aggressive"
	AIDecisionProfile string
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

	// ── Execution Custody ────────────────────────────────────────────────
	// Trusted execution custody accounts used to stage real market rebalances.
	ExecutionCustodyUSDCAccount  string
	ExecutionCustodyUSDTAccount  string
	ExecutionCustodyETHAccount   string
	ExecutionCustodySOLAccount   string
	ExecutionCustodyBTCAccount   string
	ExecutionCustodyDAIAccount   string
	ExecutionCustodyPYUSDAccount string
	// Supported values: "manual" | "auto"
	ExecutionApprovalMode string
	// Jupiter/autonomous execution guardrails.
	ExecutionMaxSlippageBps    int
	ExecutionMaxPriceImpactPct float64
	ExecutionMaxRouteHops      int
	ExecutionAutoSettle        bool

	// ── Growth Sleeve ────────────────────────────────────────────────────
	// GrowthSleeveEnabled enables a capped high-upside sleeve outside the
	// stablecoin core.
	GrowthSleeveEnabled bool
	// GrowthSleeveBudgetPct is the max treasury allocation percentage for
	// higher-beta assets.
	GrowthSleeveBudgetPct float64
	// GrowthSleeveMaxAssetPct caps any single growth asset within the sleeve.
	GrowthSleeveMaxAssetPct float64
	// GrowthSleeveAllowedAssets is the operator-approved symbol universe.
	GrowthSleeveAllowedAssets []string
	// GrowthSleeveLiveExecution requires explicit operator opt-in even when
	// the market execution path is otherwise ready.
	GrowthSleeveLiveExecution bool

	// ExecutionDevnetMode bypasses the mainnet RPC requirement for execution staging.
	// Enables full execution flow on devnet for demo/testing. Jupiter swaps will
	// fail on devnet due to missing liquidity, but staging and settlement work.
	ExecutionDevnetMode bool
}

type YieldReadiness struct {
	Mode                string
	MainnetRPC          bool
	MissingStrategyATAs []string
	MissingKaminoVaults []string
	ReadyForLive        bool
	Note                string
}

type ExecutionReadiness struct {
	Mode                   string
	MainnetRPC             bool
	MissingCustodyAccounts []string
	ReadyForStaging        bool
	ReadyForAutoSwap       bool
	ApprovalMode           string
	AutoExecutionEnabled   bool
	Note                   string
}

type GrowthReadiness struct {
	Mode             string
	BudgetPct        float64
	MaxAssetPct      float64
	AllowedAssets    []string
	ExecutionReady   bool
	ReadyForLive     bool
	LiveExecution    bool
	WithinBudget     bool
	HasAllowedAssets bool
	RequiresOperator bool
	Note             string
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
		OperatorWallets: parseCSV("OPERATOR_WALLETS"),
		OperatorEmails:  parseCSV("OPERATOR_EMAILS"),
		MintA:           os.Getenv("MINT_A"),
		MintB:           os.Getenv("MINT_B"),
		MintBTC:         os.Getenv("MINT_BTC"),
		MintETH:         os.Getenv("MINT_ETH"),
		MintSOL:         os.Getenv("MINT_SOL"),
		MintDAI:         os.Getenv("MINT_DAI"),
		MintPYUSD:       os.Getenv("MINT_PYUSD"),
		VaultPDA:        os.Getenv("VAULT_PDA"),

		StrategyMode:          uint8(getInt("STRATEGY_MODE", 1)),
		AutoExecute:           getBool("AUTO_EXECUTE", false),
		AIIntervalSec:         getInt("AI_INTERVAL_SEC", 30),
		AIAgentModel:          getOrDefault("AI_AGENT_MODEL", "claude-haiku-4-5"),
		AIDecisionProfile:     getAIDecisionProfile("AI_DECISION_PROFILE", "balanced"),
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

		YieldEnabled:                 getBool("YIELD_ENABLED", false),
		YieldMinAPY:                  getFloat("YIELD_MIN_APY", 4.0),
		YieldEntryRisk:               getFloat("YIELD_ENTRY_RISK", 35.0),
		YieldExitRisk:                getFloat("YIELD_EXIT_RISK", 55.0),
		YieldDepositAmount:           getFloat("YIELD_DEPOSIT_AMOUNT", 1000.0),
		YieldStrategyUSDCAccount:     os.Getenv("YIELD_STRATEGY_USDC_ACCOUNT"),
		YieldStrategyUSDTAccount:     os.Getenv("YIELD_STRATEGY_USDT_ACCOUNT"),
		YieldStrategyDAIAccount:      os.Getenv("YIELD_STRATEGY_DAI_ACCOUNT"),
		YieldStrategyPYUSDAccount:    os.Getenv("YIELD_STRATEGY_PYUSD_ACCOUNT"),
		YieldKaminoUSDCVault:         os.Getenv("YIELD_KAMINO_USDC_VAULT"),
		YieldKaminoUSDTVault:         os.Getenv("YIELD_KAMINO_USDT_VAULT"),
		YieldKaminoDAIVault:          os.Getenv("YIELD_KAMINO_DAI_VAULT"),
		YieldKaminoPYUSDVault:        os.Getenv("YIELD_KAMINO_PYUSD_VAULT"),
		ExecutionCustodyUSDCAccount:  os.Getenv("EXECUTION_CUSTODY_USDC_ACCOUNT"),
		ExecutionCustodyUSDTAccount:  os.Getenv("EXECUTION_CUSTODY_USDT_ACCOUNT"),
		ExecutionCustodyETHAccount:   os.Getenv("EXECUTION_CUSTODY_ETH_ACCOUNT"),
		ExecutionCustodySOLAccount:   os.Getenv("EXECUTION_CUSTODY_SOL_ACCOUNT"),
		ExecutionCustodyBTCAccount:   os.Getenv("EXECUTION_CUSTODY_BTC_ACCOUNT"),
		ExecutionCustodyDAIAccount:   os.Getenv("EXECUTION_CUSTODY_DAI_ACCOUNT"),
		ExecutionCustodyPYUSDAccount: os.Getenv("EXECUTION_CUSTODY_PYUSD_ACCOUNT"),
		ExecutionApprovalMode:        getExecutionApprovalMode("EXECUTION_APPROVAL_MODE", "manual"),
		ExecutionMaxSlippageBps:      getInt("EXECUTION_MAX_SLIPPAGE_BPS", 50),
		ExecutionMaxPriceImpactPct:   getFloat("EXECUTION_MAX_PRICE_IMPACT_PCT", 1.5),
		ExecutionMaxRouteHops:        getInt("EXECUTION_MAX_ROUTE_HOPS", 3),
		ExecutionAutoSettle:          getBool("EXECUTION_AUTO_SETTLE", true),
		GrowthSleeveEnabled:          getBool("GROWTH_SLEEVE_ENABLED", false),
		GrowthSleeveBudgetPct:        getFloat("GROWTH_SLEEVE_BUDGET_PCT", 0),
		GrowthSleeveMaxAssetPct:      getFloat("GROWTH_SLEEVE_MAX_ASSET_PCT", 0),
		GrowthSleeveAllowedAssets:    parseCSV("GROWTH_SLEEVE_ALLOWED_ASSETS"),
		GrowthSleeveLiveExecution:    getBool("GROWTH_SLEEVE_LIVE_EXECUTION", false),
		ExecutionDevnetMode:          getBool("EXECUTION_DEVNET_MODE", false),
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

func parseCSV(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	return ParseCSVString(raw)
}

func ParseCSVString(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func getExecutionApprovalMode(key, def string) string {
	mode := strings.ToLower(strings.TrimSpace(getOrDefault(key, def)))
	switch mode {
	case "auto":
		return "auto"
	default:
		return "manual"
	}
}

func getAIDecisionProfile(key, def string) string {
	profile := strings.ToLower(strings.TrimSpace(getOrDefault(key, def)))
	switch profile {
	case "cautious":
		return "cautious"
	case "aggressive":
		return "aggressive"
	default:
		return "balanced"
	}
}

func IsMainnetRPC(rpcURL string) bool {
	u := strings.ToLower(rpcURL)
	return strings.Contains(u, "mainnet") || strings.Contains(u, "helius") || strings.Contains(u, "quiknode") || strings.Contains(u, "alchemy")
}

func IsLocalnetRPC(rpcURL string) bool {
	u := strings.ToLower(rpcURL)
	return strings.Contains(u, "127.0.0.1") || strings.Contains(u, "localhost")
}

// ExplorerClusterParam returns the Solana Explorer cluster query parameter for a given RPC URL.
func ExplorerClusterParam(rpcURL string) string {
	if IsMainnetRPC(rpcURL) {
		return ""
	}
	if IsLocalnetRPC(rpcURL) {
		return "?cluster=custom&customUrl=http%3A%2F%2F127.0.0.1%3A8899"
	}
	return "?cluster=devnet"
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

func (c *Config) ExecutionReadiness() ExecutionReadiness {
	readiness := ExecutionReadiness{
		Mode:         "record_only",
		ApprovalMode: "manual",
		Note:         "Market execution is unavailable under the current custody architecture: configure trusted execution custody accounts before staging funds into manual external swap execution.",
	}
	if c == nil {
		return readiness
	}
	readiness.MainnetRPC = IsMainnetRPC(c.SolanaRPCURL)
	if c.ExecutionApprovalMode != "" {
		readiness.ApprovalMode = c.ExecutionApprovalMode
	}

	if c.ExecutionCustodyUSDCAccount == "" {
		readiness.MissingCustodyAccounts = append(readiness.MissingCustodyAccounts, "EXECUTION_CUSTODY_USDC_ACCOUNT")
	}
	if c.ExecutionCustodyUSDTAccount == "" {
		readiness.MissingCustodyAccounts = append(readiness.MissingCustodyAccounts, "EXECUTION_CUSTODY_USDT_ACCOUNT")
	}
	if c.ExecutionCustodyETHAccount == "" {
		readiness.MissingCustodyAccounts = append(readiness.MissingCustodyAccounts, "EXECUTION_CUSTODY_ETH_ACCOUNT")
	}
	if c.ExecutionCustodySOLAccount == "" {
		readiness.MissingCustodyAccounts = append(readiness.MissingCustodyAccounts, "EXECUTION_CUSTODY_SOL_ACCOUNT")
	}
	if c.ExecutionCustodyBTCAccount == "" {
		readiness.MissingCustodyAccounts = append(readiness.MissingCustodyAccounts, "EXECUTION_CUSTODY_BTC_ACCOUNT")
	}
	if c.ExecutionCustodyDAIAccount == "" {
		readiness.MissingCustodyAccounts = append(readiness.MissingCustodyAccounts, "EXECUTION_CUSTODY_DAI_ACCOUNT")
	}
	if c.ExecutionCustodyPYUSDAccount == "" {
		readiness.MissingCustodyAccounts = append(readiness.MissingCustodyAccounts, "EXECUTION_CUSTODY_PYUSD_ACCOUNT")
	}

	devnetMode := c.ExecutionDevnetMode
	if len(readiness.MissingCustodyAccounts) == 0 {
		readiness.Mode = "custody_scaffold"
		readiness.ReadyForStaging = true
		if readiness.MainnetRPC || devnetMode {
			readiness.ReadyForAutoSwap = true
			readiness.AutoExecutionEnabled = readiness.ApprovalMode == "auto"
			if readiness.AutoExecutionEnabled {
				readiness.Note = "Trusted execution custody accounts are configured on a mainnet-capable RPC. Market rebalances can stage source assets, simulate and submit a Jupiter swap, and settle the output token back into treasury autonomously."
			} else {
				readiness.Note = "Trusted execution custody accounts are configured on a mainnet-capable RPC. Market rebalances can stage source assets, build and submit a Jupiter swap, and settle the output token back into treasury."
			}
		} else {
			readiness.Note = "Trusted execution custody accounts are configured, but the backend is not using a mainnet-capable RPC. Funds can be staged and settled manually, but Jupiter swap execution is not available yet."
		}
	}

	return readiness
}

// DevnetMintBySymbol returns the devnet SPL mint address for a given token symbol.
// Returns "" if not configured. Used as fallback when Jupiter is unavailable.
func (c *Config) DevnetMintBySymbol(symbol string) string {
	if c == nil {
		return ""
	}
	switch symbol {
	case "USDC":
		return c.MintA
	case "USDT":
		return c.MintB
	case "SOL":
		return c.MintSOL
	case "BTC":
		return c.MintBTC
	case "ETH":
		return c.MintETH
	case "DAI":
		return c.MintDAI
	case "PYUSD":
		return c.MintPYUSD
	}
	return ""
}

func (c *Config) GrowthReadiness() GrowthReadiness {
	readiness := GrowthReadiness{
		Mode:             "disabled",
		RequiresOperator: true,
		Note:             "Growth sleeve is disabled. StableGuard remains focused on the stablecoin treasury core.",
	}
	if c == nil {
		return readiness
	}

	readiness.BudgetPct = c.GrowthSleeveBudgetPct
	readiness.MaxAssetPct = c.GrowthSleeveMaxAssetPct
	readiness.AllowedAssets = append([]string(nil), c.GrowthSleeveAllowedAssets...)
	readiness.LiveExecution = c.GrowthSleeveLiveExecution
	readiness.WithinBudget = c.GrowthSleeveBudgetPct > 0
	readiness.HasAllowedAssets = len(c.GrowthSleeveAllowedAssets) > 0

	if !c.GrowthSleeveEnabled {
		return readiness
	}
	if !readiness.WithinBudget {
		readiness.Note = "Growth sleeve is enabled, but its budget is zero. Set GROWTH_SLEEVE_BUDGET_PCT above 0 to activate it."
		return readiness
	}
	if !readiness.HasAllowedAssets {
		readiness.Mode = "paper"
		readiness.Note = "Growth sleeve has a budget but no allowed asset universe. Add operator-approved symbols before enabling live autonomy."
		return readiness
	}

	exec := c.ExecutionReadiness()
	readiness.ExecutionReady = exec.ReadyForAutoSwap
	if !exec.ReadyForAutoSwap {
		readiness.Mode = "paper"
		readiness.Note = "Growth sleeve is policy-ready, but live execution is blocked until the market execution path is ready for automatic swaps."
		return readiness
	}

	if !c.GrowthSleeveLiveExecution {
		readiness.Mode = "paper"
		readiness.Note = "Growth sleeve is configured and execution-ready, but live autonomous execution is still disabled until explicitly opted in."
		return readiness
	}

	readiness.Mode = "live"
	readiness.ReadyForLive = true
	readiness.Note = "Growth sleeve is live: StableGuard may deploy the configured capped budget into the approved non-stable asset universe."
	return readiness
}
