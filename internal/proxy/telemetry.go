package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	mmstatGMBase              = "https://gm.mmstat.com"
	mmstatVGIFURL             = "https://log.mmstat.com/v.gif"
	defaultTelemetryNodeVer   = "v22.21.0"
	platformTypePC            = "pc"
	telemetryLanguage         = "en_US.UTF-8"
	telemetryContentType      = "application/json"
	telemetrySPMCnt           = "a2110qe.33796382.46182003.0.0"
	telemetrySIDX             = "aplusSidex"
	telemetryCKX              = "aplusCkx"
	telemetryDefaultPageTitle = "iFlow-CLI"
)

type Telemetry struct {
	userID         string
	sessionID      string
	conversationID string
	client         *http.Client
	nodeVersion    string
	cna            string
}

func NewTelemetry(userID, sessionID, conversationID string) *Telemetry {
	return &Telemetry{
		userID:         strings.TrimSpace(userID),
		sessionID:      strings.TrimSpace(sessionID),
		conversationID: strings.TrimSpace(conversationID),
		client:         &http.Client{Timeout: 10 * time.Second},
		nodeVersion:    telemetryNodeVersion(),
		cna:            telemetryCNAToken(),
	}
}

func (t *Telemetry) EmitRunStarted(ctx context.Context, model, traceID string) string {
	if t == nil || t.client == nil {
		return ""
	}

	observationID := randomHex(8)
	gokey := fmt.Sprintf(
		"pid=iflow&sam=&trace_id=%s&session_id=%s&conversation_id=%s&observation_id=%s&model=%s&tool=&user_id=%s",
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

	return observationID
}

func (t *Telemetry) EmitRunFinished(ctx context.Context, model, traceID, parentObservationID string, duration time.Duration) {
	if t == nil || t.client == nil {
		return
	}

	durationMillis := duration.Milliseconds()
	if durationMillis < 0 {
		durationMillis = 0
	}

	observationID := randomHex(8)
	gokey := fmt.Sprintf(
		"pid=iflow&sam=&trace_id=%s&session_id=%s&conversation_id=%s&observation_id=%s&parent_observation_id=%s&duration=%d&model=%s&tool=&sessionId=%s&user_id=%s",
		url.QueryEscape(traceID),
		url.QueryEscape(t.sessionID),
		url.QueryEscape(t.conversationID),
		url.QueryEscape(observationID),
		url.QueryEscape(parentObservationID),
		durationMillis,
		url.QueryEscape(model),
		url.QueryEscape(t.sessionID),
		url.QueryEscape(t.userID),
	)

	if err := t.postGM(ctx, "//aitrack.lifecycle.run_finished", gokey); err != nil {
		log.Debug().Err(err).Msg("mmstat gm event failed (//aitrack.lifecycle.run_finished)")
	}
	if err := t.postVGIF(ctx); err != nil {
		log.Debug().Err(err).Msg("mmstat v.gif failed")
	}
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
		url.QueryEscape(IFLOWCLIVersion),
		url.QueryEscape(strings.ToLower(systemName)),
		url.QueryEscape(runtime.GOARCH),
		url.QueryEscape(t.nodeVersion),
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

	body := t.vgifBody(systemName, o)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mmstatVGIFURL, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("telemetry request: %w", err)
	}

	req.Header.Set("content-type", "text/plain;charset=UTF-8")
	req.Header.Set("cache-control", "no-cache")

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

func telemetryNodeVersion() string {
	if fromEnv := strings.TrimSpace(os.Getenv("IFLOW_NODE_VERSION")); fromEnv != "" {
		return fromEnv
	}
	return defaultTelemetryNodeVer
}

func telemetryCNAToken() string {
	token := randomHex(12)
	if token == "" {
		return "iflow-go"
	}
	return token
}

type queryEntry struct {
	key      string
	value    string
	bareOnly bool
}

func (t *Telemetry) vgifBody(systemName, osToken string) string {
	cacheToken := randomHex(3)
	if cacheToken == "" {
		cacheToken = "000000"
	}

	entries := []queryEntry{
		{key: "logtype", value: "1"},
		{key: "title", value: telemetryDefaultPageTitle},
		{key: "pre", value: "-"},
		{key: "scr", value: "-"},
		{key: "cna", value: t.cna},
		{key: "spm-cnt", value: telemetrySPMCnt},
		{key: "aplus", bareOnly: true},
		{key: "pid", value: "iflow"},
		{key: "_user_id", value: t.userID},
		{key: "cache", value: cacheToken},
		{key: "sidx", value: telemetrySIDX},
		{key: "ckx", value: telemetryCKX},
		{key: "platformType", value: platformTypePC},
		{key: "device_model", value: systemName},
		{key: "os", value: systemName},
		{key: "o", value: osToken},
		{key: "node_version", value: t.nodeVersion},
		{key: "language", value: telemetryLanguage},
		{key: "interactive", value: "0"},
		{key: "iFlowEnv", value: ""},
		{key: "_g_encode", value: "utf-8"},
	}

	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		escapedKey := url.QueryEscape(entry.key)
		if entry.bareOnly {
			parts = append(parts, escapedKey)
			continue
		}
		parts = append(parts, escapedKey+"="+url.QueryEscape(entry.value))
	}

	return strings.Join(parts, "&")
}
