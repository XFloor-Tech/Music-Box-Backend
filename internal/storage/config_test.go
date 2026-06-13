package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestGetConfigLoadsStorageSettings(t *testing.T) {
	resetViper(t)
	viper.Set("storage.account_id", "account123")
	viper.Set("storage.access_key_id", "access123")
	viper.Set("storage.secret_access_key", "secret123")
	viper.Set("storage.bucket", "music-box")
	viper.Set("storage.presign_put_expiry", "10m")
	viper.Set("storage.presign_get_expiry", "30m")

	cfg, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if cfg.EndpointURL() != "https://account123.r2.cloudflarestorage.com" {
		t.Fatalf("EndpointURL() = %q, want r2 endpoint", cfg.EndpointURL())
	}
	if cfg.Bucket != "music-box" {
		t.Fatalf("Bucket = %q, want music-box", cfg.Bucket)
	}
	if cfg.PresignPutExpiry != 10*time.Minute {
		t.Fatalf("PresignPutExpiry = %s, want 10m", cfg.PresignPutExpiry)
	}
	if cfg.PresignGetExpiry != 30*time.Minute {
		t.Fatalf("PresignGetExpiry = %s, want 30m", cfg.PresignGetExpiry)
	}
}

func TestConfigRejectsMissingSecretAccessKey(t *testing.T) {
	resetViper(t)
	viper.Set("storage.account_id", "account123")
	viper.Set("storage.access_key_id", "access123")
	viper.Set("storage.bucket", "music-box")

	_, err := GetConfig()
	if err == nil {
		t.Fatal("GetConfig() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "storage.secret_access_key is required") {
		t.Fatalf("error = %q, want missing secret access key", err.Error())
	}
}

func TestGetConfigUsesDefaultPresignExpiry(t *testing.T) {
	resetViper(t)
	viper.Set("storage.account_id", "account123")
	viper.Set("storage.access_key_id", "access123")
	viper.Set("storage.secret_access_key", "secret123")
	viper.Set("storage.bucket", "music-box")

	cfg, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if cfg.PresignPutExpiry != defaultPresignPutExpiry {
		t.Fatalf("PresignPutExpiry = %s, want %s", cfg.PresignPutExpiry, defaultPresignPutExpiry)
	}
	if cfg.PresignGetExpiry != defaultPresignGetExpiry {
		t.Fatalf("PresignGetExpiry = %s, want %s", cfg.PresignGetExpiry, defaultPresignGetExpiry)
	}
}

func TestConfigRejectsInvalidPresignExpiry(t *testing.T) {
	cfg := Config{
		AccountID:        "account123",
		AccessKeyID:      "access123",
		SecretAccessKey:  "secret123",
		Bucket:           "music-box",
		PresignPutExpiry: 8 * 24 * time.Hour,
		PresignGetExpiry: time.Minute,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "storage.presign_put_expiry") {
		t.Fatalf("error = %q, want presign put expiry error", err.Error())
	}
}

func TestConfigRejectsEndpointAsAccountID(t *testing.T) {
	cfg := Config{
		AccountID:       "https://account123.r2.cloudflarestorage.com",
		AccessKeyID:     "access123",
		SecretAccessKey: "secret123",
		Bucket:          "music-box",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "not a URL") {
		t.Fatalf("error = %q, want URL error", err.Error())
	}
}

func resetViper(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}
