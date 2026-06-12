package storage

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

const r2Region = "auto"

type Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
}

func GetConfig() (Config, error) {
	cfg := Config{
		AccountID:       strings.TrimSpace(viper.GetString("storage.account_id")),
		AccessKeyID:     strings.TrimSpace(viper.GetString("storage.access_key_id")),
		SecretAccessKey: strings.TrimSpace(viper.GetString("storage.secret_access_key")),
		Bucket:          strings.TrimSpace(viper.GetString("storage.bucket")),
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

	return nil
}

func (cfg Config) EndpointURL() string {
	return "https://" + strings.TrimSpace(cfg.AccountID) + ".r2.cloudflarestorage.com"
}
