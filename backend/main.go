package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stableguard-backend/ai"
	"stableguard-backend/alerts"
	"stableguard-backend/api"
	"stableguard-backend/config"
	"stableguard-backend/hub"
	"stableguard-backend/llm"
	"stableguard-backend/onchain"
	"stableguard-backend/pipeline"
	"stableguard-backend/pyth"
	"stableguard-backend/risk"
	solanaexec "stableguard-backend/solana"
	"stableguard-backend/store"
	"stableguard-backend/yield"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	cfg := config.Load()

	// ── Persistent store ──────────────────────────────────────────────────
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open store: %v", err)
	}
	defer db.Close()

	// ── Services ──────────────────────────────────────────────────────────
	pythMonitor := pyth.New(cfg.PythHermesURL)
	llmClient := llm.New(cfg.AnthropicAPIKey)

	executor, err := solanaexec.New(cfg.SolanaRPCURL, cfg.WalletKeyPath, cfg.ProgramID)
	if err != nil {
		log.Fatalf("Failed to initialize Solana executor: %v", err)
	}

	// ── Alerts ────────────────────────────────────────────────────────────
	alerter := alerts.New(
		cfg.TelegramBotToken,
		cfg.TelegramChatID,
		cfg.DiscordWebhookURL,
		time.Duration(cfg.AlertCooldownSec)*time.Second,
	).WithTelegramSubscriptionStore(db)
	if persisted, err := db.OperatorSettings(); err == nil {
		cfg.StrategyMode = persisted.StrategyMode
		cfg.AutoExecute = persisted.AutoExecute
		cfg.YieldEnabled = persisted.YieldEnabled
		cfg.YieldEntryRisk = persisted.YieldEntryRisk
		cfg.YieldExitRisk = persisted.YieldExitRisk
		cfg.CircuitBreakerPausePct = persisted.CircuitBreakerPausePct
		cfg.AlertRiskThreshold = persisted.AlertRiskThreshold
		if persisted.ExecutionApprovalMode != "" {
			cfg.ExecutionApprovalMode = persisted.ExecutionApprovalMode
		}
		if persisted.AIAgentModel != "" {
			cfg.AIAgentModel = persisted.AIAgentModel
		}
		if persisted.AIDecisionProfile != "" {
			cfg.AIDecisionProfile = persisted.AIDecisionProfile
		}
		cfg.GrowthSleeveEnabled = persisted.GrowthSleeveEnabled
		cfg.GrowthSleeveBudgetPct = persisted.GrowthSleeveBudgetPct
		cfg.GrowthSleeveMaxAssetPct = persisted.GrowthSleeveMaxAssetPct
		cfg.GrowthSleeveAllowedAssets = config.ParseCSVString(persisted.GrowthSleeveAllowedAssets)
		cfg.GrowthSleeveLiveExecution = persisted.GrowthSleeveLiveExecution
		// Only override env-configured credentials if DB has non-empty values
		if persisted.TelegramBotToken != "" {
			alerter.UpdateTelegram(persisted.TelegramBotToken, persisted.TelegramChatID)
		}
		if persisted.DiscordWebhookURL != "" {
			alerter.UpdateDiscord(persisted.DiscordWebhookURL)
		}
		log.Printf("  Runtime     : restored persisted operator settings from SQLite (%s)", persisted.UpdatedAt.Format(time.RFC3339))
	} else if err != nil && err != sql.ErrNoRows {
		log.Printf("WARNING: failed to restore persisted operator settings: %v", err)
	}

	// ── SSE Hub ───────────────────────────────────────────────────────────
	feedHub := hub.New()

	// ── Real-time pipeline ────────────────────────────────────────────────
	streamer := pyth.NewStreamer(
		cfg.PythHermesURL,
		time.Duration(cfg.StreamPollFallbackSec)*time.Second,
	)
	scorer := risk.NewWindowedScorer(20)
	agents := ai.New(cfg.AnthropicAPIKey, cfg.AIAgentModel, cfg.AIDecisionProfile)

	// ── Yield Aggregator ──────────────────────────────────────────────────
	yieldAgg := yield.NewAggregator()
	yieldReadiness := cfg.YieldExecutionReadiness()
	executionReadiness := cfg.ExecutionReadiness()
	growthReadiness := cfg.GrowthReadiness()

	// ── On-chain Whale Aggregator ─────────────────────────────────────────
	whaleAgg := onchain.NewAggregator()

	// ── Slippage Analyzer ─────────────────────────────────────────────────
	slippageAnal := onchain.NewSlippageAnalyzer()

	// NOTE: auto-repair removed. decision_count is now managed correctly on-chain
	// via record_decision (which properly increments vault.decision_count after the mut fix).

	// ── Optional: auto-delegate agent pubkey from config ──────────────────
	if cfg.AgentPubkey != "" {
		agentPK, err := solanaexec.ParsePublicKey(cfg.AgentPubkey)
		if err != nil {
			log.Printf("WARNING: invalid AGENT_PUBKEY=%s: %v", cfg.AgentPubkey, err)
		} else {
			log.Printf("  Agent PK    : %s (will delegate on startup if needed)", agentPK)
		}
	}

	pipe := pipeline.New(streamer, scorer, agents, executor, cfg).
		WithStore(db).
		WithAlerter(alerter).
		WithHub(feedHub).
		WithYield(yieldAgg).
		WithWhales(whaleAgg)

	// Graceful shutdown context
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start pipeline in background
	go pipe.Run(ctx)
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				alerter.SyncTelegramUpdates()
			}
		}
	}()

	// ── Logging banner ────────────────────────────────────────────────────
	strategyNames := map[uint8]string{0: "SAFE", 1: "BALANCED", 2: "YIELD"}
	log.Printf("╔══════════════════════════════════════════════════╗")
	log.Printf("║         StableGuard Backend v3                  ║")
	log.Printf("╚══════════════════════════════════════════════════╝")
	log.Printf("  Program ID  : %s", cfg.ProgramID)
	log.Printf("  Wallet      : %s", executor.WalletAddress())
	log.Printf("  RPC         : %s", cfg.SolanaRPCURL)
	log.Printf("  Strategy    : %s (mode=%d)", strategyNames[cfg.StrategyMode], cfg.StrategyMode)
	log.Printf("  AutoExecute : %v", cfg.AutoExecute)
	log.Printf("  AI interval : %ds", cfg.AIIntervalSec)
	log.Printf("  AI model    : %s | profile=%s", cfg.AIAgentModel, cfg.AIDecisionProfile)
	log.Printf("  Alerts      : Telegram=%v Discord=%v",
		cfg.TelegramBotToken != "", cfg.DiscordWebhookURL != "")
	log.Printf("  Circuit Br. : enabled=%v pause=%.1f%% emergency=%.1f%%",
		cfg.CircuitBreakerEnabled, cfg.CircuitBreakerPausePct, cfg.CircuitBreakerEmergencyPct)
	log.Printf("  Pipeline    : running (SSE → risk v2 → agents → alerts)")

	// ── Fiber API ─────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName: "StableGuard API v3",
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, PATCH, DELETE, OPTIONS",
	}))

	log.Printf("  Yield Opt.  : enabled=%v minAPY=%.1f%% entryRisk=%.0f exitRisk=%.0f",
		cfg.YieldEnabled, cfg.YieldMinAPY, cfg.YieldEntryRisk, cfg.YieldExitRisk)
	log.Printf("  Yield Live  : mode=%s ready=%v mainnet=%v",
		yieldReadiness.Mode, yieldReadiness.ReadyForLive, yieldReadiness.MainnetRPC)
	if len(yieldReadiness.MissingStrategyATAs) > 0 {
		log.Printf("  Yield ATAs  : missing=%v", yieldReadiness.MissingStrategyATAs)
	}
	if len(yieldReadiness.MissingKaminoVaults) > 0 {
		log.Printf("  Yield Vaults: missing=%v", yieldReadiness.MissingKaminoVaults)
	}
	log.Printf("  Yield Note  : %s", yieldReadiness.Note)
	log.Printf("  Exec Custody: mode=%s ready=%v missing=%v",
		executionReadiness.Mode, executionReadiness.ReadyForStaging, executionReadiness.MissingCustodyAccounts)
	log.Printf("  Exec Note   : %s", executionReadiness.Note)
	log.Printf("  Growth Mode : mode=%s budget=%.1f%% assets=%v",
		growthReadiness.Mode, growthReadiness.BudgetPct, growthReadiness.AllowedAssets)
	log.Printf("  Growth Note : %s", growthReadiness.Note)

	handler := api.New(pythMonitor, llmClient, executor).
		WithConfig(cfg).
		WithPipeline(pipe).
		WithStore(db).
		WithAlerter(alerter).
		WithHub(feedHub).
		WithYield(yieldAgg).
		WithWhales(whaleAgg).
		WithSlippage(slippageAnal)
	handler.Register(app)

	// Shutdown Fiber when context is cancelled
	go func() {
		<-ctx.Done()
		log.Printf("Shutting down server…")
		_ = app.Shutdown()
	}()

	log.Printf("  Listening on :%s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
