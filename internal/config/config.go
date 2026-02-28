package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
)

// Config defines all environment-driven runtime options.
type Config struct {
	Host        string `env:"IFLOW_HOST" envDefault:"0.0.0.0"`
	Port        int    `env:"IFLOW_PORT" envDefault:"28000"`
	Concurrency int    `env:"IFLOW_CONCURRENCY" envDefault:"1"`
	DataDir     string `env:"IFLOW_DATA_DIR" envDefault:"./data"`
	LogLevel    string `env:"IFLOW_LOG_LEVEL" envDefault:"info"`
	Proxy       string `env:"IFLOW_UPSTREAM_PROXY"`
}

// Load reads .env (if present) and parses environment variables into Config.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) {
			return nil, fmt.Errorf("load .env: %w", err)
		}
	}

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse env config: %w", err)
	}

	return cfg, nil
}
