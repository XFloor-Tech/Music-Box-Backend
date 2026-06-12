package auth

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAuthResponseUsesCamelCaseFields(t *testing.T) {
	expiresAt := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	body, err := json.Marshal(AuthResponse{
		Success: true,
		Status:  "success",
		Data: AuthResponseData{
			Session: AuthSessionResponse{
				ExpiresAt: expiresAt,
			},
			User: AuthUserResponse{
				ID:            "usr_123",
				Email:         "user@example.com",
				Name:          "user",
				EmailVerified: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	data, ok := raw["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %T, want object", raw["data"])
	}
	session, ok := data["session"].(map[string]any)
	if !ok {
		t.Fatalf("session = %T, want object", data["session"])
	}
	if session["expiresAt"] != "2026-06-17T12:00:00Z" {
		t.Fatalf("expiresAt = %v, want timestamp", session["expiresAt"])
	}
	if _, ok := session["expires_at"]; ok {
		t.Fatalf("expires_at = %v, want absent", session["expires_at"])
	}
	user, ok := data["user"].(map[string]any)
	if !ok {
		t.Fatalf("user = %T, want object", data["user"])
	}
	if user["emailVerified"] != true {
		t.Fatalf("emailVerified = %v, want true", user["emailVerified"])
	}
	if _, ok := user["email_verified"]; ok {
		t.Fatalf("email_verified = %v, want absent", user["email_verified"])
	}
}
