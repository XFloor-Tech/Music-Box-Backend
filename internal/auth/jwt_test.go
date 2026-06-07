package auth

import (
	"testing"
	"time"
)

func TestTokenServiceIssueAndVerify(t *testing.T) {
	service := NewTokenService(TokenConfig{
		Secret:   []byte("test-secret"),
		Issuer:   "music-box-backend",
		Audience: "music-box",
		TTL:      time.Hour,
	})

	issued, err := service.Issue("user-id-1", "user@example.com")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if issued.Token == "" {
		t.Fatal("Issue() token is empty")
	}
	if issued.TokenID == "" {
		t.Fatal("Issue() token id is empty")
	}
	if time.Until(issued.ExpiresAt) <= 0 {
		t.Fatal("Issue() expiresAt is not in the future")
	}

	verified, err := service.Verify(issued.Token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if verified.UserID != "user-id-1" {
		t.Fatalf("Verify() user id = %q, want %q", verified.UserID, "user-id-1")
	}
	if verified.PID != "user@example.com" {
		t.Fatalf("Verify() pid = %q, want %q", verified.PID, "user@example.com")
	}
	if verified.TokenID != issued.TokenID {
		t.Fatalf("Verify() token id = %q, want %q", verified.TokenID, issued.TokenID)
	}
}

func TestTokenServiceVerifyRejectsWrongAudience(t *testing.T) {
	issuer := NewTokenService(TokenConfig{
		Secret:   []byte("test-secret"),
		Issuer:   "music-box-backend",
		Audience: "music-box",
		TTL:      time.Hour,
	})
	verifier := NewTokenService(TokenConfig{
		Secret:   []byte("test-secret"),
		Issuer:   "music-box-backend",
		Audience: "other-client",
		TTL:      time.Hour,
	})

	issued, err := issuer.Issue("user-id-1", "user@example.com")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	if _, err := verifier.Verify(issued.Token); err == nil {
		t.Fatal("Verify() error = nil, want audience validation error")
	}
}
