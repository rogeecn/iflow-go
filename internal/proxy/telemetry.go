package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	mmstatGMBase         = "https://gm.mmstat.com"
	mmstatVGIFURL        = "https://log.mmstat.com/v.gif"
	iflowCLIVersion      = "0.5.13"
	nodeVersionEmulated  = "v22.22.0"
	platformTypePC       = "pc"
	telemetryUserAgent   = "node"
	telemetryLanguage    = "zh_CN.UTF-8"
	telemetryContentType = "application/json"
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

func (t *Telemetry) EmitRunStarted(ctx context.Context, model, traceID string) string {
	if t == nil || t.client == nil {
		return ""
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

	if err := t.postGM(ctx, "//aitrack.lifecycle.run_started", gokey); err != nil {
		log.Debug().Err(err).Msg("mmstat gm event failed (//aitrack.lifecycle.run_started)")
	}
	if err := t.postVGIF(ctx); err != nil {
		log.Debug().Err(err).Msg("mmstat v.gif failed")
	}

	return observationID
}

func (t *Telemetry) EmitRunError(ctx context.Context, model, traceID, parentObservationID, errMsg string) {
	if t == nil || t.client == nil {
		return
	}

	observationID := randomHex(8)
	systemName := runtimeSystemName()
	gokey := fmt.Sprintf(
		"pid=iflow&sam=iflow.cli.%s.%s&trace_id=%s&observation_id=%s&parent_observation_id=%s&session_id=%s&conversation_id=%s&user_id=%s&error_msg=%s&model=%s&tool=&toolName=&toolArgs=&cliVer=%s&platform=%s&arch=%s&nodeVersion=%s&osVersion=%s",
		url.QueryEscape(t.conversationID),
		url.QueryEscape(traceID),
		url.QueryEscape(traceID),
		url.QueryEscape(observationID),
		url.QueryEscape(parentObservationID),
		url.QueryEscape(t.sessionID),
		url.QueryEscape(t.conversationID),
		url.QueryEscape(t.userID),
		url.QueryEscape(errMsg),
		url.QueryEscape(model),
		url.QueryEscape(iflowCLIVersion),
		url.QueryEscape(strings.ToLower(systemName)),
		url.QueryEscape(runtime.GOARCH),
		url.QueryEscape(nodeVersionEmulated),
		url.QueryEscape(runtimePlatformVersion()),
	)

	if err := t.postGM(ctx, "//aitrack.lifecycle.run_error", gokey); err != nil {
		log.Debug().Err(err).Msg("mmstat gm event failed (//aitrack.lifecycle.run_error)")
	}
}

func (t *Telemetry) postGM(ctx context.Context, path, gokey string) error {
	payload := fmt.Sprintf(`{"gmkey":"AI","gokey":"%s"}`, gokey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mmstatGMBase+path, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("telemetry request: %w", err)
	}

	req.Header.Set("Content-Type", telemetryContentType)
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "*")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("user-agent", telemetryUserAgent)
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

func (t *Telemetry) postVGIF(ctx context.Context) error {
	systemName := runtimeSystemName()
	o := strings.ToLower(systemName)
	if o == "windows" {
		o = "win"
	}

	form := url.Values{}
	form.Set("logtype", "1")
	form.Set("title", "iFlow-CLI")
	form.Set("pre", "-")
	form.Set("platformType", platformTypePC)
	form.Set("device_model", systemName)
	form.Set("os", systemName)
	form.Set("o", o)
	form.Set("node_version", nodeVersionEmulated)
	form.Set("language", telemetryLanguage)
	form.Set("interactive", "0")
	form.Set("iFlowEnv", "")
	form.Set("_g_encode", "utf-8")
	form.Set("pid", "iflow")
	form.Set("_user_id", t.userID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mmstatVGIFURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("telemetry request: %w", err)
	}

	req.Header.Set("content-type", "text/plain;charset=UTF-8")
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "*")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("user-agent", telemetryUserAgent)
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

func runtimeSystemName() string {
	switch runtime.GOOS {
	case "darwin":
		return "Darwin"
	case "linux":
		return "Linux"
	case "windows":
		return "Windows"
	default:
		return runtime.GOOS
	}
}

func runtimePlatformVersion() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}
