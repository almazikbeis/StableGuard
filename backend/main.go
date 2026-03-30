package main

import (
	"log"
	"stableguard-backend/api"
	"stableguard-backend/config"
	"stableguard-backend/llm"
	"stableguard-backend/pyth"
	solanaexec "stableguard-backend/solana"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	cfg := config.Load()

	// Initialize services
	pythMonitor := pyth.New(cfg.PythHermesURL)

	llmClient := llm.New(cfg.AnthropicAPIKey)

	executor, err := solanaexec.New(cfg.SolanaRPCURL, cfg.WalletKeyPath, cfg.ProgramID)
	if err != nil {
		log.Fatalf("Failed to initialize Solana executor: %v", err)
	}

	log.Printf("StableGuard backend starting")
	log.Printf("  Program ID  : %s", cfg.ProgramID)
	log.Printf("  Wallet      : %s", executor.WalletAddress())
	log.Printf("  RPC         : %s", cfg.SolanaRPCURL)

	// Set up Fiber app
	app := fiber.New(fiber.Config{
		AppName: "StableGuard API v1",
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	// Register routes
	handler := api.New(pythMonitor, llmClient, executor)
	handler.Register(app)

	log.Printf("  Listening on :%s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
