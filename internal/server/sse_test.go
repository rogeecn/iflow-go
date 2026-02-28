package server

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSSEWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, err := NewSSEWriter(rec)
	if err != nil {
		t.Fatalf("NewSSEWriter error: %v", err)
	}

	if err := sse.WriteEvent(`{"ok":true}`); err != nil {
		t.Fatalf("WriteEvent error: %v", err)
	}
	if err := sse.WriteDone(); err != nil {
		t.Fatalf("WriteDone error: %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "data: {\"ok\":true}") {
		t.Fatalf("unexpected event body: %s", body)
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Fatalf("missing done marker: %s", body)
	}
}
