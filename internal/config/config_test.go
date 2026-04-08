package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	originalEnv := make(map[string]string)
	keys := []string{"APP_PORT", "APP_ENV", "LOG_LEVEL", "AUTH_SERVICE_URL", "UPSTREAM_TIMEOUT_SECONDS", "RATE_LIMIT_RPS", "RATE_LIMIT_BURST"}
	for _, k := range keys {
		originalEnv[k] = os.Getenv(k)
		err := os.Unsetenv(k)
		if err != nil {
			return
		}
	}
	defer func() {
		for k, v := range originalEnv {
			if v != "" {
				err := os.Setenv(k, v)
				if err != nil {
					return
				}
			} else {
				err := os.Unsetenv(k)
				if err != nil {
					return
				}
			}
		}
	}()

	t.Run("default values", func(t *testing.T) {
		cfg, err := Load()
		assert.NoError(t, err)
		assert.Equal(t, 8080, cfg.Port)
		assert.Equal(t, "local", cfg.AppEnv)
		assert.Equal(t, "info", cfg.LogLevel)
		assert.Equal(t, "http://localhost:8081", cfg.AuthServiceURL)
		assert.Equal(t, 5*time.Second, cfg.UpstreamTimeout)
		assert.Equal(t, 10.0, cfg.RateLimitRPS)
		assert.Equal(t, 20, cfg.RateLimitBurst)
	})

	t.Run("custom values from env", func(t *testing.T) {
		assert.NoError(t, os.Setenv("APP_PORT", "9090"))
		assert.NoError(t, os.Setenv("APP_ENV", "prod"))
		assert.NoError(t, os.Setenv("LOG_LEVEL", "debug"))
		assert.NoError(t, os.Setenv("AUTH_SERVICE_URL", "http://auth:8081"))
		assert.NoError(t, os.Setenv("UPSTREAM_TIMEOUT_SECONDS", "10"))
		assert.NoError(t, os.Setenv("RATE_LIMIT_RPS", "50"))
		assert.NoError(t, os.Setenv("RATE_LIMIT_BURST", "100"))

		cfg, err := Load()
		assert.NoError(t, err)
		assert.Equal(t, 9090, cfg.Port)
		assert.Equal(t, "prod", cfg.AppEnv)
		assert.Equal(t, "debug", cfg.LogLevel)
		assert.Equal(t, "http://auth:8081", cfg.AuthServiceURL)
		assert.Equal(t, 10*time.Second, cfg.UpstreamTimeout)
		assert.Equal(t, 50.0, cfg.RateLimitRPS)
		assert.Equal(t, 100, cfg.RateLimitBurst)
	})

	t.Run("invalid port", func(t *testing.T) {
		assert.NoError(t, os.Setenv("APP_PORT", "invalid"))
		_, err := Load()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "APP_PORT must be an integer")
	})

	t.Run("invalid rate limit rps", func(t *testing.T) {
		assert.NoError(t, os.Unsetenv("APP_PORT"))
		assert.NoError(t, os.Setenv("RATE_LIMIT_RPS", "invalid"))
		_, err := Load()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "RATE_LIMIT_RPS must be a float")
	})
}
