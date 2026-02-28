package config

import (
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// InitLogger initializes and returns a structured logger.
func InitLogger(level string) zerolog.Logger {
	parsedLevel, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		parsedLevel = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(parsedLevel)

	return zerolog.New(os.Stdout).With().Timestamp().Logger()
}
