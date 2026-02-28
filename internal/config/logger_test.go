package config

import (
	"testing"

	"github.com/rs/zerolog"
)

func TestInitLogger(t *testing.T) {
	InitLogger("debug")
	if got := zerolog.GlobalLevel(); got != zerolog.DebugLevel {
		t.Fatalf("GlobalLevel = %s, want %s", got, zerolog.DebugLevel)
	}

	InitLogger("invalid-level")
	if got := zerolog.GlobalLevel(); got != zerolog.InfoLevel {
		t.Fatalf("GlobalLevel = %s, want %s", got, zerolog.InfoLevel)
	}
}
