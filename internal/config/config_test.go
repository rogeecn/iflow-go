package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("IFLOW_HOST", "127.0.0.1")
	t.Setenv("IFLOW_PORT", "18080")
	t.Setenv("IFLOW_CONCURRENCY", "3")
	t.Setenv("IFLOW_DATA_DIR", "./tmp-data")
	t.Setenv("IFLOW_LOG_LEVEL", "debug")
	t.Setenv("IFLOW_UPSTREAM_PROXY", "http://127.0.0.1:8080")
	t.Setenv("IFLOW_PRESERVE_REASONING_CONTENT", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Host != "127.0.0.1" {
		t.Fatalf("Host = %q, want %q", cfg.Host, "127.0.0.1")
	}
	if cfg.Port != 18080 {
		t.Fatalf("Port = %d, want %d", cfg.Port, 18080)
	}
	if cfg.Concurrency != 3 {
		t.Fatalf("Concurrency = %d, want %d", cfg.Concurrency, 3)
	}
	if cfg.DataDir != "./tmp-data" {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, "./tmp-data")
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.Proxy != "http://127.0.0.1:8080" {
		t.Fatalf("Proxy = %q, want %q", cfg.Proxy, "http://127.0.0.1:8080")
	}
	if cfg.PreserveReasoningContent {
		t.Fatalf("PreserveReasoningContent = %v, want false", cfg.PreserveReasoningContent)
	}
}

func TestLoadInvalidPort(t *testing.T) {
	t.Setenv("IFLOW_PORT", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
}

func TestLoadPreserveReasoningDefault(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.PreserveReasoningContent {
		t.Fatalf("PreserveReasoningContent = %v, want true", cfg.PreserveReasoningContent)
	}
}
