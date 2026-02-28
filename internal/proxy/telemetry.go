package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	mmstatGMBase = "https://gm.mmstat.com"
)

type Telemetry struct {
	userID         string
	sessionID      string
	conversationID string
	client         *http.Client
}

func NewTelemetry(userID, sessionID, conversationID string) *Telemetry {
	return &Telemetry{
		userID:         strings.TrimSpace(userID),
		sessionID:      strings.TrimSpace(sessionID),
		conversationID: strings.TrimSpace(conversationID),
		client:         &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *Telemetry) EmitRunStarted(ctx context.Context, model, traceID string) error {
	if t == nil || t.client == nil {
		return nil
	}

	observationID := randomHex(8)
	gokey := fmt.Sprintf(
		"pid=iflow&sam=iflow.cli.%s.%s&trace_id=%s&session_id=%s&conversation_id=%s&observation_id=%s&model=%s&tool=&user_id=%s",
		url.QueryEscape(t.conversationID),
		url.QueryEscape(traceID),
		url.QueryEscape(traceID),
		url.QueryEscape(t.sessionID),
		url.QueryEscape(t.conversationID),
		url.QueryEscape(observationID),
		url.QueryEscape(model),
		url.QueryEscape(t.userID),
	)

	return t.postGM(ctx, "//aitrack.lifecycle.run_started", gokey)
}

func (t *Telemetry) EmitRunError(ctx context.Context, model, traceID, errMsg string) error {
	if t == nil || t.client == nil {
		return nil
	}

	observationID := randomHex(8)
	gokey := fmt.Sprintf(
		"pid=iflow&sam=iflow.cli.%s.%s&trace_id=%s&session_id=%s&conversation_id=%s&observation_id=%s&model=%s&tool=&user_id=%s&error_msg=%s",
		url.QueryEscape(t.conversationID),
		url.QueryEscape(traceID),
		url.QueryEscape(traceID),
		url.QueryEscape(t.sessionID),
		url.QueryEscape(t.conversationID),
		url.QueryEscape(observationID),
		url.QueryEscape(model),
		url.QueryEscape(t.userID),
		url.QueryEscape(errMsg),
	)

	return t.postGM(ctx, "//aitrack.lifecycle.run_error", gokey)
}

func (t *Telemetry) postGM(ctx context.Context, path, gokey string) error {
	payload := fmt.Sprintf(`{"gmkey":"AI","gokey":"%s"}`, gokey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mmstatGMBase+path, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("telemetry request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "*")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("user-agent", "node")
	req.Header.Set("accept-encoding", "br, gzip, deflate")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("telemetry send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telemetry status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}
