package proxy

import "testing"

func TestGenerateSignature(t *testing.T) {
	got := GenerateSignature("iFlow-Cli", "session-123", 1700000000000, "sk-test-key")
	want := "58c53cca7a2e841bed7cd0e9d89d519b04ac2b16414480e86430ceddde6d4547"
	if got != want {
		t.Fatalf("GenerateSignature() = %q, want %q", got, want)
	}
}

func TestGenerateSignatureEmptyAPIKey(t *testing.T) {
	got := GenerateSignature("iFlow-Cli", "session-123", 1700000000000, "")
	if got != "" {
		t.Fatalf("GenerateSignature() with empty api key = %q, want empty string", got)
	}
}

func TestGenerateSignatureKeepsRawInput(t *testing.T) {
	got := GenerateSignature("", "session-123", 1700000000000, "sk-test-key")
	want := "447cc85c4958d3d088ca905b2fd5cc0e883c6e22d61c308bf838b46048451353"
	if got != want {
		t.Fatalf("GenerateSignature() = %q, want %q", got, want)
	}
}
