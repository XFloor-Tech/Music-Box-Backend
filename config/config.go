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

	viper.RegisterAlias("auth.cookie_state_cookie_name", "AUTH_COOKIE_STATE_COOKIE_NAME")
	viper.RegisterAlias("auth.cookie_state_max_age", "AUTH_COOKIE_STATE_MAX_AGE")
	viper.RegisterAlias("auth.cookie_secret", "AUTH_COOKIE_SECRET")
	viper.RegisterAlias("auth.cookie_secure", "AUTH_COOKIE_SECURE")
	viper.RegisterAlias("auth.cookie_same_site", "AUTH_COOKIE_SAME_SITE")

	viper.RegisterAlias("auth.jwt_secret", "AUTH_JWT_SECRET")
	viper.RegisterAlias("auth.jwt_issuer", "AUTH_JWT_ISSUER")
	viper.RegisterAlias("auth.jwt_audience", "AUTH_JWT_AUDIENCE")
	viper.RegisterAlias("auth.jwt_ttl", "AUTH_JWT_TTL")
	viper.RegisterAlias("auth.refresh_token_cookie_name", "AUTH_REFRESH_TOKEN_COOKIE_NAME")
	viper.RegisterAlias("auth.refresh_token_ttl", "AUTH_REFRESH_TOKEN_TTL")

	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.backend_uri", "")
	viper.SetDefault("server.max_body_bytes", 1024*1024)
	viper.SetDefault("server.max_header_bytes", 1024*1024)
	viper.SetDefault("database.addr", "")
	viper.SetDefault("database.max_connections", 10)
	viper.SetDefault("auth.mount_path", "/auth")
	viper.SetDefault("auth.root_url", "")
	viper.SetDefault("auth.session_cookie_name", "music_box_session")
	viper.SetDefault("auth.cookie_state_cookie_name", "music_box_auth")
	viper.SetDefault("auth.cookie_state_max_age", "720h")
	viper.SetDefault("auth.cookie_secret", "")
	viper.SetDefault("auth.cookie_secure", true)
	viper.SetDefault("auth.cookie_same_site", "lax")
	viper.SetDefault("auth.jwt_secret", "")
	viper.SetDefault("auth.jwt_issuer", "music-box-backend")
	viper.SetDefault("auth.jwt_audience", "music-box")
	viper.SetDefault("auth.jwt_ttl", "15m")
	viper.SetDefault("auth.refresh_token_cookie_name", "music_box_refresh")
	viper.SetDefault("auth.refresh_token_ttl", "720h")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	return nil
}
