package config

import (
	"errors"
	"strings"

	"github.com/spf13/viper"
)

func Load() error {
	viper.SetConfigFile(".env.development")
	viper.SetConfigType("env")

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return err
		}
	}

	viper.RegisterAlias("server.port", "SERVER_PORT")
	viper.RegisterAlias("server.backend_uri", "SERVER_BACKEND_URI")
	viper.RegisterAlias("server.max_body_bytes", "SERVER_MAX_BODY_BYTES")
	viper.RegisterAlias("server.max_header_bytes", "SERVER_MAX_HEADER_BYTES")

	viper.RegisterAlias("database.addr", "DATABASE_ADDR")
	viper.RegisterAlias("database.max_connections", "DATABASE_MAX_CONNECTIONS")

	viper.RegisterAlias("auth.mount_path", "AUTH_MOUNT_PATH")
	viper.RegisterAlias("auth.root_url", "AUTH_ROOT_URL")
	viper.RegisterAlias("auth.session_cookie_name", "AUTH_SESSION_COOKIE_NAME")
	viper.RegisterAlias("auth.session_ttl", "AUTH_SESSION_TTL")
	viper.RegisterAlias("auth.session_update_age", "AUTH_SESSION_UPDATE_AGE")
	viper.RegisterAlias("auth.session_cleanup_interval", "AUTH_SESSION_CLEANUP_INTERVAL")
	viper.RegisterAlias("auth.trusted_origins", "AUTH_TRUSTED_ORIGINS")

	viper.RegisterAlias("auth.cookie_state_cookie_name", "AUTH_COOKIE_STATE_COOKIE_NAME")
	viper.RegisterAlias("auth.cookie_state_max_age", "AUTH_COOKIE_STATE_MAX_AGE")
	viper.RegisterAlias("auth.cookie_secret", "AUTH_COOKIE_SECRET")
	viper.RegisterAlias("auth.cookie_secure", "AUTH_COOKIE_SECURE")
	viper.RegisterAlias("auth.cookie_same_site", "AUTH_COOKIE_SAME_SITE")

	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.backend_uri", "")
	viper.SetDefault("server.max_body_bytes", 1024*1024)
	viper.SetDefault("server.max_header_bytes", 1024*1024)
	viper.SetDefault("database.addr", "")
	viper.SetDefault("database.max_connections", 10)

	viper.SetDefault("auth.mount_path", "/auth")
	viper.SetDefault("auth.root_url", "")
	viper.SetDefault("auth.session_cookie_name", "music_box_session")
	viper.SetDefault("auth.session_ttl", "168h")
	viper.SetDefault("auth.session_update_age", "24h")
	viper.SetDefault("auth.session_cleanup_interval", "5h")
	viper.SetDefault("auth.trusted_origins", "")
	viper.SetDefault("auth.cookie_state_cookie_name", "music_box_auth")
	viper.SetDefault("auth.cookie_state_max_age", "720h")
	viper.SetDefault("auth.cookie_secret", "")
	viper.SetDefault("auth.cookie_secure", true)
	viper.SetDefault("auth.cookie_same_site", "lax")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	return nil
}
