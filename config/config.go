package config

import (
	"errors"
	"strings"

	"github.com/spf13/viper"
)

func Load() error {
	viper.SetDefault("server.port", "8080")

	viper.SetConfigFile(".env.development")
	viper.SetConfigType("env")

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return err
		}
	}

	viper.RegisterAlias("server.port", "SERVER_PORT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	return nil
}
