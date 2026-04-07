package api

import (
	"strings"

	"stableguard-backend/auth"
	"stableguard-backend/config"

	"github.com/gofiber/fiber/v2"
)

const bearerPrefix = "Bearer "

// operatorRequired accepts either an allowlisted email token or an allowlisted wallet token.
// It must be used for any route that can mutate vault state or backend operator settings.
func operatorRequired(cfg *config.Config, serverWallet string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw := strings.TrimSpace(c.Get("Authorization"))
		if !strings.HasPrefix(raw, bearerPrefix) {
			return fiber.NewError(fiber.StatusUnauthorized, "missing bearer token")
		}

		token := strings.TrimSpace(strings.TrimPrefix(raw, bearerPrefix))
		if token == "" {
			return fiber.NewError(fiber.StatusUnauthorized, "missing bearer token")
		}

		if claims, err := auth.ValidateWalletToken(token); err == nil && strings.TrimSpace(claims.Wallet) != "" {
			if !isAuthorizedOperatorWallet(claims.Wallet, cfg, serverWallet) {
				return fiber.NewError(fiber.StatusForbidden, "wallet is authenticated but not authorized for operator access")
			}
			c.Locals("auth_type", "wallet")
			c.Locals("auth_wallet", claims.Wallet)
			return c.Next()
		}

		if claims, err := auth.ValidateToken(token); err == nil && strings.TrimSpace(claims.Email) != "" {
			if !isAuthorizedOperatorEmail(claims.Email, cfg) {
				return fiber.NewError(fiber.StatusForbidden, "email is authenticated but not authorized for operator access")
			}
			c.Locals("auth_type", "user")
			c.Locals("auth_email", claims.Email)
			c.Locals("auth_user_id", claims.UserID)
			return c.Next()
		}

		return fiber.NewError(fiber.StatusUnauthorized, "invalid or expired token")
	}
}

func userRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw := strings.TrimSpace(c.Get("Authorization"))
		if !strings.HasPrefix(raw, bearerPrefix) {
			return fiber.NewError(fiber.StatusUnauthorized, "missing bearer token")
		}

		token := strings.TrimSpace(strings.TrimPrefix(raw, bearerPrefix))
		if token == "" {
			return fiber.NewError(fiber.StatusUnauthorized, "missing bearer token")
		}

		if claims, err := auth.ValidateWalletToken(token); err == nil && strings.TrimSpace(claims.Wallet) != "" {
			c.Locals("auth_type", "wallet")
			c.Locals("auth_wallet", claims.Wallet)
			return c.Next()
		}

		if claims, err := auth.ValidateToken(token); err == nil {
			if strings.TrimSpace(claims.Email) == "" {
				return fiber.NewError(fiber.StatusUnauthorized, "invalid user token")
			}
			c.Locals("auth_type", "user")
			c.Locals("auth_email", claims.Email)
			c.Locals("auth_user_id", claims.UserID)
			return c.Next()
		}

		return fiber.NewError(fiber.StatusUnauthorized, "invalid or expired token")
	}
}

func isAuthorizedOperatorEmail(email string, cfg *config.Config) bool {
	if cfg == nil || email == "" {
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(email))
	for _, allowed := range cfg.OperatorEmails {
		if normalized == strings.ToLower(strings.TrimSpace(allowed)) {
			return true
		}
	}
	return false
}

func isAuthorizedOperatorWallet(wallet string, cfg *config.Config, serverWallet string) bool {
	normalized := strings.TrimSpace(wallet)
	if normalized == "" {
		return false
	}
	// Server wallet is always authorized
	if normalized == strings.TrimSpace(serverWallet) {
		return true
	}
	if cfg == nil {
		return false
	}
	// Wildcard: if OPERATOR_WALLETS contains "*" or is empty, allow any valid wallet
	if len(cfg.OperatorWallets) == 0 {
		return true
	}
	for _, allowed := range cfg.OperatorWallets {
		if strings.TrimSpace(allowed) == "*" || normalized == strings.TrimSpace(allowed) {
			return true
		}
	}
	return false
}
