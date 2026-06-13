package storage

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	r2Region                = "auto"
	defaultPresignPutExpiry = 15 * time.Minute
	defaultPresignGetExpiry = 15 * time.Minute
	maxPresignExpiry        = 7 * 24 * time.Hour
)

type Config struct {
	AccountID        string
	AccessKeyID      string
	SecretAccessKey  string
	Bucket           string
	PresignPutExpiry time.Duration
	PresignGetExpiry time.Duration
}

func GetConfig() (Config, error) {
	cfg := Config{
		AccountID:        strings.TrimSpace(viper.GetString("storage.account_id")),
		AccessKeyID:      strings.TrimSpace(viper.GetString("storage.access_key_id")),
		SecretAccessKey:  strings.TrimSpace(viper.GetString("storage.secret_access_key")),
		Bucket:           strings.TrimSpace(viper.GetString("storage.bucket")),
		PresignPutExpiry: durationFromConfig("storage.presign_put_expiry", defaultPresignPutExpiry),
		PresignGetExpiry: durationFromConfig("storage.presign_get_expiry", defaultPresignGetExpiry),
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (cfg Config) Validate() error {
	if cfg.AccountID == "" {
		return fmt.Errorf("storage.account_id is required")
	}
	if strings.ContainsAny(cfg.AccountID, "/:") {
		return fmt.Errorf("storage.account_id must be a Cloudflare account ID, not a URL")
	}
	if cfg.AccessKeyID == "" {
		return fmt.Errorf("storage.access_key_id is required")
	}
	if cfg.SecretAccessKey == "" {
		return fmt.Errorf("storage.secret_access_key is required")
	}
	if cfg.Bucket == "" {
		return fmt.Errorf("storage.bucket is required")
	}
	if cfg.PresignPutExpiry <= 0 {
		return fmt.Errorf("storage.presign_put_expiry must be greater than 0")
	}
	if cfg.PresignPutExpiry > maxPresignExpiry {
		return fmt.Errorf("storage.presign_put_expiry must be less than or equal to %s", maxPresignExpiry)
	}
	if cfg.PresignGetExpiry <= 0 {
		return fmt.Errorf("storage.presign_get_expiry must be greater than 0")
	}
	if cfg.PresignGetExpiry > maxPresignExpiry {
		return fmt.Errorf("storage.presign_get_expiry must be less than or equal to %s", maxPresignExpiry)
	}

	return nil
}

func (cfg Config) EndpointURL() string {
	return "https://" + strings.TrimSpace(cfg.AccountID) + ".r2.cloudflarestorage.com"
}

func durationFromConfig(key string, fallback time.Duration) time.Duration {
	if !viper.IsSet(key) {
		return fallback
	}

	return viper.GetDuration(key)
}
