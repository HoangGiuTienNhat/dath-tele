package config

import (
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	DatabaseURL       string `mapstructure:"DATABASE_URL"`
	HTTPServerAddress string `mapstructure:"HTTP_SERVER_ADDRESS"`
	MinioEndpoint     string `mapstructure:"MINIO_ENDPOINT"`
	MinioAccessKey    string `mapstructure:"MINIO_ACCESS_KEY"`
	MinioSecretKey    string `mapstructure:"MINIO_SECRET_KEY"`
	MinioBucket       string `mapstructure:"MINIO_BUCKET"`
	MinioUseSSL       string `mapstructure:"MINIO_USE_SSL"`
}

func LoadConfig(path string) (config *Config, err error) {
	viper.SetConfigName("dev")
	viper.AddConfigPath(path)
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	// Bind environment variables explicitly
	viper.BindEnv("DATABASE_URL")
	viper.BindEnv("HTTP_SERVER_ADDRESS")
	viper.BindEnv("MINIO_ENDPOINT")
	viper.BindEnv("MINIO_ACCESS_KEY")
	viper.BindEnv("MINIO_SECRET_KEY")
	viper.BindEnv("MINIO_BUCKET")
	viper.BindEnv("MINIO_USE_SSL")

	// Try to read config file, but don't fail if it doesn't exist
	// (in production on Fly.io, env vars are set directly)
	if err = viper.ReadInConfig(); err != nil {
		// Only fail if we're not in production (no FLY_APP_NAME env var)
		if os.Getenv("FLY_APP_NAME") == "" {
			return nil, err
		}
		// In production, continue without the config file
	}

	if err = viper.Unmarshal(&config); err != nil {
		return
	}

	return
}
