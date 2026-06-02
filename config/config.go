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

	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.backend_uri", "")
	viper.SetDefault("server.max_body_bytes", 1024*1024)
	viper.SetDefault("server.max_header_bytes", 1024*1024)
	viper.SetDefault("database.addr", "")
	viper.SetDefault("database.max_connections", 10)

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	return nil
}
