package proxy

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestTelemetryEmitRunStarted(t *testing.T) {
	telemetry := NewTelemetry("user-1", "session-1", "conversation-1")
	telemetry.client = &http.Client{
		Transport: proxyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://gm.mmstat.com//aitrack.lifecycle.run_started" {
				t.Fatalf("unexpected url: %s", req.URL.String())
			}

			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			bodyText := string(body)
			if !strings.Contains(bodyText, `"gmkey":"AI"`) {
				t.Fatalf("payload missing gmkey: %s", bodyText)
			}
			if !strings.Contains(bodyText, "model=glm-5") {
				t.Fatalf("payload missing model: %s", bodyText)
			}
			if !strings.Contains(bodyText, "trace_id=trace-id") {
				t.Fatalf("payload missing trace_id: %s", bodyText)
			}
			return newProxyResponse(http.StatusOK, `{"ok":true}`), nil
		}),
	}

	if err := telemetry.EmitRunStarted(context.Background(), "glm-5", "trace-id"); err != nil {
		t.Fatalf("EmitRunStarted error: %v", err)
	}
}

func TestTelemetryEmitRunError(t *testing.T) {
	telemetry := NewTelemetry("user-1", "session-1", "conversation-1")
	telemetry.client = &http.Client{
		Transport: proxyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://gm.mmstat.com//aitrack.lifecycle.run_error" {
				t.Fatalf("unexpected url: %s", req.URL.String())
			}

			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			bodyText := string(body)
			if !strings.Contains(bodyText, "error_msg=bad+request") {
				t.Fatalf("payload missing error_msg: %s", bodyText)
			}
			return newProxyResponse(http.StatusOK, `{"ok":true}`), nil
		}),
	}

	if err := telemetry.EmitRunError(context.Background(), "glm-5", "trace-id", "bad request"); err != nil {
		t.Fatalf("EmitRunError error: %v", err)
	}
}
