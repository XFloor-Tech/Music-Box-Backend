package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestGetConfigRejectsWeakCookieSecret(t *testing.T) {
	resetViper(t)

	viper.Set("auth.cookie_secret", "short")

	if _, err := GetConfig(); err == nil {
		t.Fatal("GetConfig() error = nil, want weak cookie secret error")
	}
}

func TestGetConfigRejectsSameSiteNoneWithoutSecureCookie(t *testing.T) {
	resetViper(t)

	viper.Set("auth.cookie_secret", validTestSecret())
	viper.Set("auth.cookie_same_site", "none")
	viper.Set("auth.cookie_secure", false)

	if _, err := GetConfig(); err == nil {
		t.Fatal("GetConfig() error = nil, want SameSite=None secure cookie error")
	}
}

func TestGetConfigIncludesRootURLAsTrustedOrigin(t *testing.T) {
	resetViper(t)

	viper.Set("auth.cookie_secret", validTestSecret())
	viper.Set("auth.root_url", "https://api.example.com/auth")

	cfg, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if len(cfg.TrustedOrigins) != 1 || cfg.TrustedOrigins[0] != "https://api.example.com" {
		t.Fatalf("TrustedOrigins = %v, want [https://api.example.com]", cfg.TrustedOrigins)
	}
}

func TestGetConfigDefaultsSessionUpdateAge(t *testing.T) {
	resetViper(t)

	viper.Set("auth.cookie_secret", validTestSecret())

	cfg, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if cfg.SessionUpdateAge != defaultSessionUpdateAge {
		t.Fatalf("SessionUpdateAge = %v, want %v", cfg.SessionUpdateAge, defaultSessionUpdateAge)
	}
}

func TestGetConfigDefaultsSessionCleanupInterval(t *testing.T) {
	resetViper(t)

	viper.Set("auth.cookie_secret", validTestSecret())

	cfg, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if cfg.SessionCleanupInterval != defaultSessionCleanupInterval {
		t.Fatalf("SessionCleanupInterval = %v, want %v", cfg.SessionCleanupInterval, defaultSessionCleanupInterval)
	}
}

func TestGetConfigReadsSessionCleanupInterval(t *testing.T) {
	resetViper(t)

	viper.Set("auth.cookie_secret", validTestSecret())
	viper.Set("auth.session_cleanup_interval", "2h")

	cfg, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if cfg.SessionCleanupInterval != 2*time.Hour {
		t.Fatalf("SessionCleanupInterval = %v, want 2h", cfg.SessionCleanupInterval)
	}
}

func TestGetConfigRejectsSessionUpdateAgeAtLeastSessionTTL(t *testing.T) {
	resetViper(t)

	viper.Set("auth.cookie_secret", validTestSecret())
	viper.Set("auth.session_ttl", "24h")
	viper.Set("auth.session_update_age", "24h")

	if _, err := GetConfig(); err == nil {
		t.Fatal("GetConfig() error = nil, want invalid session update age error")
	}
}

func resetViper(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func validTestSecret() string {
	return strings.Repeat("a", minSecretBytes)
}
