package main

import (
	"os"
	"testing"
)

func TestRunVersion(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"iflow-go", "version"}
	if err := run(); err != nil {
		t.Fatalf("run() error: %v", err)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"iflow-go", "unknown-command"}
	if err := run(); err == nil {
		t.Fatal("expected error for unknown command, got nil")
	}
}
