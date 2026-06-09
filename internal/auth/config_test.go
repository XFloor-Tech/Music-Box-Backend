package auth

import (
	"strings"
	"testing"

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

func resetViper(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func validTestSecret() string {
	return strings.Repeat("a", minSecretBytes)
}
