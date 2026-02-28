package config

import (
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// InitLogger initializes and returns a structured logger.
func InitLogger(level string) zerolog.Logger {
	logger := zerolog.New(os.Stdout).With().Timestamp().Str("service", "iflow-go").Logger()

	normalizedLevel := strings.ToLower(strings.TrimSpace(level))
	if normalizedLevel == "" {
		normalizedLevel = "info"
	}

	parsedLevel, err := zerolog.ParseLevel(normalizedLevel)
	if err != nil {
		parsedLevel = zerolog.InfoLevel
		logger.Warn().
			Str("configured_level", level).
			Str("fallback_level", parsedLevel.String()).
			Msg("invalid log level, fallback to default")
	}

	zerolog.SetGlobalLevel(parsedLevel)
	return logger.Level(parsedLevel)
}
