package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"stableguard-backend/auth"
	"stableguard-backend/config"

	"github.com/gofiber/fiber/v2"
)

func TestOperatorRequiredAllowsConfiguredWallet(t *testing.T) {
	app := fiber.New()
	cfg := &config.Config{OperatorWallets: []string{"operator-wallet-1"}}
	app.Get("/protected", operatorRequired(cfg, "server-wallet"), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	token, err := auth.GenerateWalletToken("operator-wallet-1")
	if err != nil {
		t.Fatalf("generate wallet token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}

	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestOperatorRequiredAllowsServerWalletByDefault(t *testing.T) {
	app := fiber.New()
	app.Get("/protected", operatorRequired(&config.Config{}, "server-wallet"), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	token, err := auth.GenerateWalletToken("server-wallet")
	if err != nil {
		t.Fatalf("generate wallet token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}

	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestOperatorRequiredRejectsAuthenticatedButUnauthorizedWallet(t *testing.T) {
	app := fiber.New()
	cfg := &config.Config{OperatorWallets: []string{"operator-wallet-1"}}
	app.Get("/protected", operatorRequired(cfg, "server-wallet"), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	token, err := auth.GenerateWalletToken("random-wallet")
	if err != nil {
		t.Fatalf("generate wallet token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}

	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestOperatorRequiredAllowsConfiguredEmail(t *testing.T) {
	app := fiber.New()
	cfg := &config.Config{OperatorEmails: []string{"ops@example.com"}}
	app.Get("/protected", operatorRequired(cfg, "server-wallet"), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	token, err := auth.GenerateToken(1, "ops@example.com")
	if err != nil {
		t.Fatalf("generate email token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}

	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}
