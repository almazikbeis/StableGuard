package main

import (
	"context"
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
	)

	// ── SSE Hub ───────────────────────────────────────────────────────────
	feedHub := hub.New()

	// ── Real-time pipeline ────────────────────────────────────────────────
	streamer := pyth.NewStreamer(
		cfg.PythHermesURL,
		time.Duration(cfg.StreamPollFallbackSec)*time.Second,
	)
	scorer := risk.NewWindowedScorer(20)
	agents := ai.New(cfg.AnthropicAPIKey)

	// ── Yield Aggregator ──────────────────────────────────────────────────
	yieldAgg := yield.NewAggregator()
	yieldReadiness := cfg.YieldExecutionReadiness()

	// ── On-chain Whale Aggregator ─────────────────────────────────────────
	whaleAgg := onchain.NewAggregator()

	// ── Slippage Analyzer ─────────────────────────────────────────────────
	slippageAnal := onchain.NewSlippageAnalyzer()

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
		WithYield(yieldAgg)

	// Graceful shutdown context
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start pipeline in background
	go pipe.Run(ctx)

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
		AllowHeaders: "Origin, Content-Type, Accept",
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
