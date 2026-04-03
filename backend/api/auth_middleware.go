package api

import (
	"strings"

	"stableguard-backend/auth"

	"github.com/gofiber/fiber/v2"
)

const bearerPrefix = "Bearer "

// authRequired accepts either an email/password JWT or a wallet JWT.
// It is used for operator/admin routes that must never be publicly callable.
func authRequired(c *fiber.Ctx) error {
	raw := strings.TrimSpace(c.Get("Authorization"))
	if !strings.HasPrefix(raw, bearerPrefix) {
		return fiber.NewError(fiber.StatusUnauthorized, "missing bearer token")
	}

	token := strings.TrimSpace(strings.TrimPrefix(raw, bearerPrefix))
	if token == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "missing bearer token")
	}

	if claims, err := auth.ValidateToken(token); err == nil {
		c.Locals("auth_type", "user")
		c.Locals("auth_email", claims.Email)
		c.Locals("auth_user_id", claims.UserID)
		return c.Next()
	}

	if claims, err := auth.ValidateWalletToken(token); err == nil {
		c.Locals("auth_type", "wallet")
		c.Locals("auth_wallet", claims.Wallet)
		return c.Next()
	}

	return fiber.NewError(fiber.StatusUnauthorized, "invalid or expired token")
}
