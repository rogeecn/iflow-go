package proxy

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rogeecn/iflow-go/internal/account"
	"github.com/rogeecn/iflow-go/pkg/types"
)

const (
	defaultBaseURL = "https://apis.iflow.cn/v1"
)

type IFlowProxy struct {
	account       *account.Account
	client        *http.Client
	baseURL       string
	headerBuilder *HeaderBuilder
	telemetry     *Telemetry
}

func NewProxy(acct *account.Account) *IFlowProxy {
	if acct == nil {
		acct = &account.Account{}
	}

	baseURL := strings.TrimSpace(acct.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	builder := NewHeaderBuilder(acct)
	userID := uuid.NewSHA1(uuid.NameSpaceDNS, []byte(strings.TrimSpace(acct.APIKey)+builder.sessionID)).String()

	return &IFlowProxy{
		account:       acct,
		client:        &http.Client{Timeout: 300 * time.Second},
		baseURL:       baseURL,
		headerBuilder: builder,
		telemetry:     NewTelemetry(userID, builder.sessionID, builder.conversationID),
	}
}

func (p *IFlowProxy) ChatCompletions(ctx context.Context, req *types.ChatCompletionRequest) (*types.ChatCompletionResponse, error) {
	requestBody, err := requestToBodyMap(req)
	if err != nil {
		return nil, err
	}

	model := ""
	if req != nil {
		model = req.Model
	}
	requestBody = ConfigureModelParams(requestBody, model)

	traceparent := p.headerBuilder.traceparentGenerator()
	traceID := extractTraceID(traceparent)
	if p.telemetry != nil {
		_ = p.telemetry.EmitRunStarted(ctx, model, traceID)
	}

	headers := p.headerBuilder.Build(false)
	headers["traceparent"] = traceparent

	responseBody, statusCode, err := p.doChatRequest(ctx, headers, requestBody)
	if err != nil {
		if p.telemetry != nil {
			_ = p.telemetry.EmitRunError(ctx, model, traceID, err.Error())
		}
		return nil, err
	}
	if statusCode >= http.StatusBadRequest {
		err := fmt.Errorf("chat completions: status=%d body=%s", statusCode, strings.TrimSpace(string(responseBody)))
		if p.telemetry != nil {
			_ = p.telemetry.EmitRunError(ctx, model, traceID, err.Error())
		}
		return nil, err
	}

	var normalized map[string]interface{}
	if err := json.Unmarshal(responseBody, &normalized); err != nil {
		if p.telemetry != nil {
			_ = p.telemetry.EmitRunError(ctx, model, traceID, err.Error())
		}
		return nil, fmt.Errorf("chat completions: decode response: %w", err)
	}
	normalized = NormalizeResponse(normalized, false)
	ensureUsage(normalized)

	normalizedBytes, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("chat completions: encode normalized response: %w", err)
	}

	var parsed types.ChatCompletionResponse
	if err := json.Unmarshal(normalizedBytes, &parsed); err != nil {
		return nil, fmt.Errorf("chat completions: parse response type: %w", err)
	}

	return &parsed, nil
}

func (p *IFlowProxy) ChatCompletionsStream(ctx context.Context, req *types.ChatCompletionRequest) (<-chan []byte, error) {
	requestBody, err := requestToBodyMap(req)
	if err != nil {
		return nil, err
	}

	model := ""
	if req != nil {
		model = req.Model
	}
	requestBody = ConfigureModelParams(requestBody, model)
	requestBody["stream"] = true

	traceparent := p.headerBuilder.traceparentGenerator()
	traceID := extractTraceID(traceparent)
	if p.telemetry != nil {
		_ = p.telemetry.EmitRunStarted(ctx, model, traceID)
	}

	headers := p.headerBuilder.Build(true)
	headers["traceparent"] = traceparent

	reqBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("chat stream: encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.chatCompletionsURL(), bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("chat stream: create request: %w", err)
	}
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		if p.telemetry != nil {
			_ = p.telemetry.EmitRunError(ctx, model, traceID, err.Error())
		}
		return nil, fmt.Errorf("chat stream: send request: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		defer resp.Body.Close()
		body, _ := readDecodedBody(resp)
		err := fmt.Errorf("chat stream: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
		if p.telemetry != nil {
			_ = p.telemetry.EmitRunError(ctx, model, traceID, err.Error())
		}
		return nil, err
	}

	streamBody, err := decodedBodyReader(resp)
	if err != nil {
		_ = resp.Body.Close()
		if p.telemetry != nil {
			_ = p.telemetry.EmitRunError(ctx, model, traceID, err.Error())
		}
		return nil, fmt.Errorf("chat stream: decode response body: %w", err)
	}

	out := make(chan []byte, 32)
	go p.forwardSSE(ctx, streamBody, out)
	return out, nil
}

func (p *IFlowProxy) Models() []ModelConfig {
	result := make([]ModelConfig, len(Models))
	copy(result, Models)
	return result
}

func (p *IFlowProxy) doChatRequest(ctx context.Context, headers map[string]string, body map[string]interface{}) ([]byte, int, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("chat completions: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.chatCompletionsURL(), bytes.NewReader(payload))
	if err != nil {
		return nil, 0, fmt.Errorf("chat completions: create request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("chat completions: send request: %w", err)
	}
	defer resp.Body.Close()

	content, err := readDecodedBody(resp)
	if err != nil {
		return nil, 0, fmt.Errorf("chat completions: read response: %w", err)
	}

	return content, resp.StatusCode, nil
}

func (p *IFlowProxy) chatCompletionsURL() string {
	return p.baseURL + "/chat/completions"
}

func requestToBodyMap(req *types.ChatCompletionRequest) (map[string]interface{}, error) {
	if req == nil {
		return nil, fmt.Errorf("chat completions: nil request")
	}

	raw, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("chat completions: encode request type: %w", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("chat completions: decode request map: %w", err)
	}

	return body, nil
}

func ensureUsage(response map[string]interface{}) {
	if _, ok := response["usage"]; ok {
		return
	}
	response["usage"] = map[string]interface{}{
		"prompt_tokens":     0,
		"completion_tokens": 0,
		"total_tokens":      0,
	}
}

func extractTraceID(traceparent string) string {
	parts := strings.Split(traceparent, "-")
	if len(parts) == 4 && len(parts[1]) == 32 {
		return parts[1]
	}
	return randomHex(16)
}

func normalizeStreamChunk(chunk map[string]interface{}) map[string]interface{} {
	choices, ok := chunk["choices"].([]interface{})
	if !ok {
		return chunk
	}

	for _, choice := range choices {
		choiceMap, ok := choice.(map[string]interface{})
		if !ok {
			continue
		}
		delta, ok := choiceMap["delta"].(map[string]interface{})
		if !ok {
			continue
		}

		_, hasContent := delta["content"]
		reasoning, hasReasoning := delta["reasoning_content"]
		reasoningText, reasoningTextOK := reasoning.(string)
		if !hasContent && hasReasoning && reasoningTextOK && reasoningText != "" {
			delta["content"] = reasoningText
			delete(delta, "reasoning_content")
			continue
		}
		if hasContent && hasReasoning {
			delete(delta, "reasoning_content")
		}
	}

	return chunk
}

func (p *IFlowProxy) forwardSSE(ctx context.Context, in io.ReadCloser, out chan<- []byte) {
	defer close(out)
	defer in.Close()

	reader := bufio.NewReader(in)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			payload := []byte(line)
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "data:") {
				dataPart := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
				if dataPart != "" && dataPart != "[DONE]" {
					var chunk map[string]interface{}
					if jsonErr := json.Unmarshal([]byte(dataPart), &chunk); jsonErr == nil {
						chunk = normalizeStreamChunk(chunk)
						if chunkRaw, marshalErr := json.Marshal(chunk); marshalErr == nil {
							payload = []byte("data: " + string(chunkRaw) + "\n\n")
						}
					}
				}
			}

			select {
			case out <- payload:
			case <-ctx.Done():
				return
			}
		}

		if err == io.EOF {
			return
		}
		if err != nil {
			return
		}
	}
}

type compositeReadCloser struct {
	io.Reader
	closers []io.Closer
}

func (c *compositeReadCloser) Close() error {
	var firstErr error
	for _, closer := range c.closers {
		if err := closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func readDecodedBody(resp *http.Response) ([]byte, error) {
	reader, err := decodedBodyReader(resp)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func decodedBodyReader(resp *http.Response) (io.ReadCloser, error) {
	encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	switch encoding {
	case "":
		return resp.Body, nil
	case "gzip":
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		return &compositeReadCloser{
			Reader:  gr,
			closers: []io.Closer{gr, resp.Body},
		}, nil
	case "deflate":
		zr, err := zlib.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("deflate reader: %w", err)
		}
		return &compositeReadCloser{
			Reader:  zr,
			closers: []io.Closer{zr, resp.Body},
		}, nil
	default:
		// Unknown encoding, fall back to raw body for compatibility.
		return resp.Body, nil
	}
}
