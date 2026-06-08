package auth

import (
	"encoding/base64"
	"net/http"
	"testing"
	"time"
)

func TestRefreshTokenDataFromCookieValueMatchesOpaqueToken(t *testing.T) {
	hash, token, err := newRefreshToken()
	if err != nil {
		t.Fatalf("newRefreshToken() error = %v", err)
	}

	data, err := refreshTokenDataFromCookieValue(token)
	if err != nil {
		t.Fatalf("refreshTokenDataFromCookieValue() error = %v", err)
	}

	if data.Hash != hash {
		t.Fatalf("Hash = %q, want %q", data.Hash, hash)
	}
}

func TestRefreshTokenDataFromCookieValueRejectsInvalidToken(t *testing.T) {
	if _, err := refreshTokenDataFromCookieValue("not-a-refresh-token"); err == nil {
		t.Fatal("refreshTokenDataFromCookieValue() error = nil, want error")
	}
}

func TestRefreshTokenDataFromCookieValueRejectsShortToken(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString([]byte("short"))
	if _, err := refreshTokenDataFromCookieValue(token); err == nil {
		t.Fatal("refreshTokenDataFromCookieValue() error = nil, want length error")
	}
}

func TestRefreshTokenCookieUsesConfiguredSecurityAttributes(t *testing.T) {
	module := &Module{
		config: Config{
			RefreshTokenCookieName: "music_box_refresh",
			CookieSecure:           true,
			CookieSameSite:         http.SameSiteStrictMode,
		},
	}

	cookie := module.refreshTokenCookie("token", int(defaultRefreshTokenTTL.Seconds()))
	if cookie.Name != "music_box_refresh" {
		t.Fatalf("Name = %q, want %q", cookie.Name, "music_box_refresh")
	}
	if cookie.Value != "token" {
		t.Fatalf("Value = %q, want token", cookie.Value)
	}
	if cookie.Path != "/" {
		t.Fatalf("Path = %q, want /", cookie.Path)
	}
	if cookie.MaxAge != int((30 * 24 * time.Hour).Seconds()) {
		t.Fatalf("MaxAge = %d, want %d", cookie.MaxAge, int((30 * 24 * time.Hour).Seconds()))
	}
	if !cookie.HttpOnly {
		t.Fatal("HttpOnly = false, want true")
	}
	if !cookie.Secure {
		t.Fatal("Secure = false, want true")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("SameSite = %v, want %v", cookie.SameSite, http.SameSiteStrictMode)
	}
	if time.Until(cookie.Expires) <= 0 {
		t.Fatal("Expires is not in the future")
	}
}

func TestRefreshTokenCookieClearsCookie(t *testing.T) {
	module := &Module{
		config: Config{
			RefreshTokenCookieName: "music_box_refresh",
		},
	}

	cookie := module.refreshTokenCookie("", -1)
	if cookie.Name != "music_box_refresh" {
		t.Fatalf("Name = %q, want %q", cookie.Name, "music_box_refresh")
	}
	if cookie.MaxAge != -1 {
		t.Fatalf("MaxAge = %d, want -1", cookie.MaxAge)
	}
	if !cookie.Expires.Before(time.Now()) {
		t.Fatal("Expires is not in the past")
	}
}
