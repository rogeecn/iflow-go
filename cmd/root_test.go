package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func executeForTest(args ...string) (string, error) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	rootCmd.SetArgs(nil)
	return buf.String(), err
}

func TestRootHelp(t *testing.T) {
	out, err := executeForTest("--help")
	if err != nil {
		t.Fatalf("help command error: %v", err)
	}
	if !strings.Contains(out, "serve") {
		t.Fatalf("help output missing serve command: %s", out)
	}
	if !strings.Contains(out, "token") {
		t.Fatalf("help output missing token command: %s", out)
	}
	if !strings.Contains(out, "version") {
		t.Fatalf("help output missing version command: %s", out)
	}
}

func TestVersionCommand(t *testing.T) {
	out, err := executeForTest("version")
	if err != nil {
		t.Fatalf("version command error: %v", err)
	}
	if !strings.Contains(out, "iflow-go") {
		t.Fatalf("version output missing binary name: %s", out)
	}
	if !strings.Contains(out, "commit:") {
		t.Fatalf("version output missing commit field: %s", out)
	}
}

func TestTokenDeleteInvalidUUID(t *testing.T) {
	_, err := executeForTest("token", "delete", "not-a-uuid")
	if err == nil {
		t.Fatal("expected invalid uuid error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid uuid") {
		t.Fatalf("unexpected error: %v", err)
	}
}
