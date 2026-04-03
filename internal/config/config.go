package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv          string
	Port            int
	LogLevel        string
	AuthServiceURL  string
	UpstreamTimeout time.Duration
}

func MustLoad() Config {
	cfg, err := Load()
	if err != nil {
		panic(err)
	}
	return cfg
}

func Load() (Config, error) {
	port, err := getEnvAsInt("APP_PORT", 8080)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}

	timeoutSeconds, err := getEnvAsInt("UPSTREAM_TIMEOUT_SECONDS", 5)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}

	cfg := Config{
		AppEnv:          getEnv("APP_ENV", "local"),
		Port:            port,
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		AuthServiceURL:  getEnv("AUTH_SERVICE_URL", "http://localhost:8081"),
		UpstreamTimeout: time.Duration(timeoutSeconds) * time.Second,
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) (int, error) {
	raw := getEnv(key, strconv.Itoa(fallback))
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", key, raw)
	}
	return value, nil
}
