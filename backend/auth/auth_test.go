package auth

import (
	"testing"
	"time"
)

func TestVerifyWalletMessageFreshnessAcceptsRecentTimestamp(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	msg := "StableGuard login: 1700000000"
	if err := verifyWalletMessageFreshness(msg, now); err != nil {
		t.Fatalf("expected recent login message to be accepted, got %v", err)
	}
}

func TestVerifyWalletMessageFreshnessRejectsExpiredTimestamp(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	msg := "StableGuard login: 1699999600"
	if err := verifyWalletMessageFreshness(msg, now); err == nil {
		t.Fatal("expected expired login message to be rejected")
	}
}

func TestVerifyWalletMessageFreshnessRejectsMalformedTimestamp(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	msg := "StableGuard login: not-a-timestamp"
	if err := verifyWalletMessageFreshness(msg, now); err == nil {
		t.Fatal("expected malformed login message to be rejected")
	}
}
