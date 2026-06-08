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

func TestGetConfigRejectsWeakJWTSecret(t *testing.T) {
	resetViper(t)

	viper.Set("auth.cookie_secret", validTestSecret())
	viper.Set("auth.jwt_secret", "short")

	if _, err := GetConfig(); err == nil {
		t.Fatal("GetConfig() error = nil, want weak JWT secret error")
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

func resetViper(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func validTestSecret() string {
	return strings.Repeat("a", minSecretBytes)
}
