package auth

import (
	"encoding/base64"
	"net/http"
	"testing"
	"time"
)

func TestSessionTokenHashMatchesOpaqueToken(t *testing.T) {
	hash, token, err := newSessionToken()
	if err != nil {
		t.Fatalf("newSessionToken() error = %v", err)
	}

	got, err := sessionTokenHash(token)
	if err != nil {
		t.Fatalf("sessionTokenHash() error = %v", err)
	}

	if got != hash {
		t.Fatalf("hash = %q, want %q", got, hash)
	}
}

func TestSessionTokenHashRejectsShortToken(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString([]byte("short"))
	if _, err := sessionTokenHash(token); err == nil {
		t.Fatal("sessionTokenHash() error = nil, want length error")
	}
}

func TestDBSessionCookieUsesConfiguredSecurityAttributes(t *testing.T) {
	state := NewDBSessionStateReadWriter(nil, SessionCookieConfig{
		Name:     "music_box_session",
		Path:     "/",
		HTTPOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		TTL:      defaultSessionTTL,
	})

	cookie := state.cookie("token", int(defaultSessionTTL.Seconds()))
	if cookie.Name != "music_box_session" {
		t.Fatalf("Name = %q, want music_box_session", cookie.Name)
	}
	if cookie.Value != "token" {
		t.Fatalf("Value = %q, want token", cookie.Value)
	}
	if cookie.Path != "/" {
		t.Fatalf("Path = %q, want /", cookie.Path)
	}
	if cookie.MaxAge != int((7 * 24 * time.Hour).Seconds()) {
		t.Fatalf("MaxAge = %d, want %d", cookie.MaxAge, int((7 * 24 * time.Hour).Seconds()))
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
