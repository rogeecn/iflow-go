package proxy

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rogeecn/iflow-go/internal/account"
)

func TestNewHeaderBuilder(t *testing.T) {
	builder := NewHeaderBuilder(&account.Account{APIKey: "sk-test"})

	if !strings.HasPrefix(builder.sessionID, "session-") {
		t.Fatalf("sessionID = %q, want session- prefix", builder.sessionID)
	}
	if !account.IsValidUUID(strings.TrimPrefix(builder.sessionID, "session-")) {
		t.Fatalf("sessionID suffix is not a uuid: %q", builder.sessionID)
	}
	if !account.IsValidUUID(builder.conversationID) {
		t.Fatalf("conversationID is not a uuid: %q", builder.conversationID)
	}
}

func TestHeaderBuilderBuild(t *testing.T) {
	builder := NewHeaderBuilder(&account.Account{APIKey: "sk-test-key"})
	builder.sessionID = "session-123"
	builder.conversationID = "conversation-123"
	builder.now = func() time.Time { return time.UnixMilli(1700000000000) }

	headers := builder.Build(false, "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01")

	if headers["Content-Type"] != "application/json" {
		t.Fatalf("Content-Type = %q", headers["Content-Type"])
	}
	if headers["Authorization"] != "Bearer sk-test-key" {
		t.Fatalf("Authorization = %q", headers["Authorization"])
	}
	if headers["user-agent"] != IFLOWCLIUserAgent {
		t.Fatalf("user-agent = %q", headers["user-agent"])
	}
	if headers["session-id"] != "session-123" {
		t.Fatalf("session-id = %q", headers["session-id"])
	}
	if headers["conversation-id"] != "conversation-123" {
		t.Fatalf("conversation-id = %q", headers["conversation-id"])
	}
	if headers["traceparent"] != "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01" {
		t.Fatalf("traceparent = %q", headers["traceparent"])
	}
	if headers["x-iflow-timestamp"] != "1700000000000" {
		t.Fatalf("x-iflow-timestamp = %q", headers["x-iflow-timestamp"])
	}

	wantSig := "58c53cca7a2e841bed7cd0e9d89d519b04ac2b16414480e86430ceddde6d4547"
	if headers["x-iflow-signature"] != wantSig {
		t.Fatalf("x-iflow-signature = %q, want %q", headers["x-iflow-signature"], wantSig)
	}
}

func TestHeaderBuilderBuildWithoutAPIKey(t *testing.T) {
	builder := NewHeaderBuilder(&account.Account{APIKey: "   "})
	builder.now = func() time.Time { return time.UnixMilli(1700000000000) }

	headers := builder.Build(true, "")

	if _, ok := headers["Authorization"]; ok {
		t.Fatalf("Authorization should be omitted, got %q", headers["Authorization"])
	}
	if _, ok := headers["x-iflow-signature"]; ok {
		t.Fatalf("x-iflow-signature should be omitted, got %q", headers["x-iflow-signature"])
	}
	if _, ok := headers["x-iflow-timestamp"]; ok {
		t.Fatalf("x-iflow-timestamp should be omitted, got %q", headers["x-iflow-timestamp"])
	}
	if _, ok := headers["traceparent"]; ok {
		t.Fatalf("traceparent should be omitted, got %q", headers["traceparent"])
	}
}

func TestHeaderBuilderBuildAoneHeaders(t *testing.T) {
	builder := NewHeaderBuilder(&account.Account{
		APIKey:  "sk-test-key",
		BaseURL: "https://ducky.code.alibaba-inc.com/v1",
	})
	builder.now = func() time.Time { return time.UnixMilli(1700000000000) }

	headers := builder.Build(false, "")

	if headers["X-Client-Type"] != "iflow-cli" {
		t.Fatalf("X-Client-Type = %q", headers["X-Client-Type"])
	}
	if headers["X-Client-Version"] != IFLOWCLIVersion {
		t.Fatalf("X-Client-Version = %q", headers["X-Client-Version"])
	}
}

func TestGenerateTraceparent(t *testing.T) {
	got := generateTraceparent()
	pattern := regexp.MustCompile(`^00-[0-9a-f]{32}-[0-9a-f]{16}-01$`)
	if !pattern.MatchString(got) {
		t.Fatalf("generateTraceparent() = %q, invalid format", got)
	}
}
