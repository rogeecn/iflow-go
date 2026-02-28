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
	urls := make([]string, 0, 2)
	bodies := make([]string, 0, 2)
	contentTypes := make([]string, 0, 2)

	telemetry.client = &http.Client{
		Transport: proxyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			urls = append(urls, req.URL.String())
			bodies = append(bodies, string(body))
			contentTypes = append(contentTypes, req.Header.Get("content-type"))
			return newProxyResponse(http.StatusOK, `{"ok":true}`), nil
		}),
	}

	observationID := telemetry.EmitRunStarted(context.Background(), "glm-5", "trace-id")
	if observationID == "" {
		t.Fatal("EmitRunStarted observation id should not be empty")
	}
	if len(urls) != 2 {
		t.Fatalf("requests = %d, want 2", len(urls))
	}

	if urls[0] != "https://gm.mmstat.com//aitrack.lifecycle.run_started" {
		t.Fatalf("unexpected gm url: %s", urls[0])
	}
	if !strings.Contains(bodies[0], `"gmkey":"AI"`) {
		t.Fatalf("gm payload missing gmkey: %s", bodies[0])
	}
	if !strings.Contains(bodies[0], "model=glm-5") {
		t.Fatalf("gm payload missing model: %s", bodies[0])
	}
	if !strings.Contains(bodies[0], "trace_id=trace-id") {
		t.Fatalf("gm payload missing trace_id: %s", bodies[0])
	}

	if urls[1] != "https://log.mmstat.com/v.gif" {
		t.Fatalf("unexpected vgif url: %s", urls[1])
	}
	if !strings.Contains(strings.ToLower(contentTypes[1]), "text/plain") {
		t.Fatalf("vgif content-type = %q", contentTypes[1])
	}
	if !strings.Contains(bodies[1], "_user_id=user-1") {
		t.Fatalf("vgif payload missing user id: %s", bodies[1])
	}
	if !strings.Contains(bodies[1], "node_version=v22.22.0") {
		t.Fatalf("vgif payload missing node version: %s", bodies[1])
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
			if !strings.Contains(bodyText, "parent_observation_id=parent-obs-1") {
				t.Fatalf("payload missing parent_observation_id: %s", bodyText)
			}
			if !strings.Contains(bodyText, "cliVer=0.5.13") {
				t.Fatalf("payload missing cliVer: %s", bodyText)
			}
			if !strings.Contains(bodyText, "nodeVersion=v22.22.0") {
				t.Fatalf("payload missing nodeVersion: %s", bodyText)
			}

			return newProxyResponse(http.StatusOK, `{"ok":true}`), nil
		}),
	}

	telemetry.EmitRunError(context.Background(), "glm-5", "trace-id", "parent-obs-1", "bad request")
}
