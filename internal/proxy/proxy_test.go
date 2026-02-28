package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rogeecn/iflow-go/internal/account"
	"github.com/rogeecn/iflow-go/pkg/types"
)

type proxyRoundTripFunc func(req *http.Request) (*http.Response, error)

func (f proxyRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newProxyResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestChatCompletions(t *testing.T) {
	acct := &account.Account{
		APIKey:  "sk-test",
		BaseURL: "https://apis.iflow.cn/v1",
	}
	p := NewProxy(acct)
	p.headerBuilder.sessionID = "session-fixed"
	p.headerBuilder.conversationID = "conversation-fixed"
	p.headerBuilder.now = func() time.Time { return time.UnixMilli(1700000000000) }
	p.headerBuilder.traceparentGenerator = func() string {
		return "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01"
	}
	p.telemetry = nil

	p.client = &http.Client{
		Transport: proxyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://apis.iflow.cn/v1/chat/completions" {
				t.Fatalf("unexpected url: %s", req.URL.String())
			}
			if req.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", req.Method)
			}
			if req.Header.Get("Authorization") != "Bearer sk-test" {
				t.Fatalf("Authorization = %q", req.Header.Get("Authorization"))
			}
			if req.Header.Get("x-iflow-signature") == "" {
				t.Fatal("x-iflow-signature should not be empty")
			}

			bodyRaw, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			body := string(bodyRaw)
			if !strings.Contains(body, `"enable_thinking":true`) {
				t.Fatalf("request body missing glm-5 params: %s", body)
			}

			return newProxyResponse(http.StatusOK, `{
			  "id":"chatcmpl-1",
			  "object":"chat.completion",
			  "created":1700000000,
			  "model":"glm-5",
			  "choices":[{"index":0,"message":{"role":"assistant","content":null,"reasoning_content":"thinking-text"},"finish_reason":"stop"}]
			}`), nil
		}),
	}

	resp, err := p.ChatCompletions(context.Background(), &types.ChatCompletionRequest{
		Model: "glm-5",
		Messages: []types.Message{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletions error: %v", err)
	}

	if len(resp.Choices) != 1 {
		t.Fatalf("choices len = %d, want 1", len(resp.Choices))
	}
	if resp.Choices[0].Message == nil || resp.Choices[0].Message.Content != "thinking-text" {
		t.Fatalf("message content = %#v, want thinking-text", resp.Choices[0].Message)
	}
	if resp.Choices[0].Message.ReasoningContent != "" {
		t.Fatalf("reasoning_content = %q, want empty", resp.Choices[0].Message.ReasoningContent)
	}
	if resp.Usage.TotalTokens != 0 {
		t.Fatalf("usage.total_tokens = %d, want 0", resp.Usage.TotalTokens)
	}
}

func TestChatCompletionsStream(t *testing.T) {
	acct := &account.Account{
		APIKey:  "sk-test",
		BaseURL: "https://apis.iflow.cn/v1",
	}
	p := NewProxy(acct)
	p.headerBuilder.sessionID = "session-fixed"
	p.headerBuilder.conversationID = "conversation-fixed"
	p.telemetry = nil

	p.client = &http.Client{
		Transport: proxyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := "data: {\"id\":\"chunk-1\",\"object\":\"chat.completion.chunk\",\"created\":1700000000,\"model\":\"glm-5\",\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"thinking\"},\"finish_reason\":null}]}\n\n" +
				"data: [DONE]\n\n"
			return newProxyResponse(http.StatusOK, body), nil
		}),
	}

	stream, err := p.ChatCompletionsStream(context.Background(), &types.ChatCompletionRequest{
		Model: "glm-5",
		Messages: []types.Message{
			{Role: "user", Content: "hello"},
		},
		Stream: true,
	})
	if err != nil {
		t.Fatalf("ChatCompletionsStream error: %v", err)
	}

	var got strings.Builder
	for chunk := range stream {
		got.Write(chunk)
	}

	streamData := got.String()
	if !strings.Contains(streamData, `"content":"thinking"`) {
		t.Fatalf("stream output missing normalized content: %s", streamData)
	}
	if strings.Contains(streamData, `"reasoning_content":"thinking"`) {
		t.Fatalf("stream output should remove reasoning_content: %s", streamData)
	}
	if !strings.Contains(streamData, "data: [DONE]") {
		t.Fatalf("stream output missing [DONE]: %s", streamData)
	}
}

func TestChatCompletionsPreserveReasoning(t *testing.T) {
	acct := &account.Account{
		APIKey:  "sk-test",
		BaseURL: "https://apis.iflow.cn/v1",
	}
	p := NewProxyWithReasoning(acct, true)
	p.telemetry = nil

	p.client = &http.Client{
		Transport: proxyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return newProxyResponse(http.StatusOK, `{
			  "id":"chatcmpl-1",
			  "object":"chat.completion",
			  "created":1700000000,
			  "model":"glm-5",
			  "choices":[{"index":0,"message":{"role":"assistant","content":"final","reasoning_content":"thinking-text"},"finish_reason":"stop"}]
			}`), nil
		}),
	}

	resp, err := p.ChatCompletions(context.Background(), &types.ChatCompletionRequest{
		Model: "glm-5",
		Messages: []types.Message{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletions error: %v", err)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message == nil {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Choices[0].Message.Content != "final" {
		t.Fatalf("content = %v, want final", resp.Choices[0].Message.Content)
	}
	if resp.Choices[0].Message.ReasoningContent != "thinking-text" {
		t.Fatalf("reasoning_content = %q, want thinking-text", resp.Choices[0].Message.ReasoningContent)
	}
}

func TestChatCompletionsPreserveReasoningWithoutMirroringToContent(t *testing.T) {
	acct := &account.Account{
		APIKey:  "sk-test",
		BaseURL: "https://apis.iflow.cn/v1",
	}
	p := NewProxyWithReasoning(acct, true)
	p.telemetry = nil

	p.client = &http.Client{
		Transport: proxyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return newProxyResponse(http.StatusOK, `{
			  "id":"chatcmpl-1",
			  "object":"chat.completion",
			  "created":1700000000,
			  "model":"glm-5",
			  "choices":[{"index":0,"message":{"role":"assistant","content":null,"reasoning_content":"thinking-text"},"finish_reason":"stop"}]
			}`), nil
		}),
	}

	resp, err := p.ChatCompletions(context.Background(), &types.ChatCompletionRequest{
		Model: "glm-5",
		Messages: []types.Message{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletions error: %v", err)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message == nil {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Choices[0].Message.Content != nil {
		t.Fatalf("content = %v, want nil", resp.Choices[0].Message.Content)
	}
	if resp.Choices[0].Message.ReasoningContent != "thinking-text" {
		t.Fatalf("reasoning_content = %q, want thinking-text", resp.Choices[0].Message.ReasoningContent)
	}
}

func TestChatCompletionsStreamPreserveReasoning(t *testing.T) {
	acct := &account.Account{
		APIKey:  "sk-test",
		BaseURL: "https://apis.iflow.cn/v1",
	}
	p := NewProxyWithReasoning(acct, true)
	p.telemetry = nil

	p.client = &http.Client{
		Transport: proxyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := "data: {\"id\":\"chunk-1\",\"object\":\"chat.completion.chunk\",\"created\":1700000000,\"model\":\"glm-5\",\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"thinking\"},\"finish_reason\":null}]}\n\n" +
				"data: [DONE]\n\n"
			return newProxyResponse(http.StatusOK, body), nil
		}),
	}

	stream, err := p.ChatCompletionsStream(context.Background(), &types.ChatCompletionRequest{
		Model: "glm-5",
		Messages: []types.Message{
			{Role: "user", Content: "hello"},
		},
		Stream: true,
	})
	if err != nil {
		t.Fatalf("ChatCompletionsStream error: %v", err)
	}

	var got strings.Builder
	for chunk := range stream {
		got.Write(chunk)
	}

	streamData := got.String()
	if !strings.Contains(streamData, `"reasoning_content":"thinking"`) {
		t.Fatalf("stream output missing reasoning_content: %s", streamData)
	}
	if strings.Contains(streamData, `"content":"thinking"`) {
		t.Fatalf("stream output should not mirror reasoning into content: %s", streamData)
	}
}

func TestModelsReturnsCopy(t *testing.T) {
	p := NewProxy(&account.Account{APIKey: "sk-test"})
	models := p.Models()
	if len(models) == 0 {
		t.Fatal("models should not be empty")
	}

	models[0].ID = "mutated"
	if Models[0].ID == "mutated" {
		t.Fatal("Models() should return a copy, but global model list was mutated")
	}
}

func TestNewProxyTelemetryUserIDPrefersAPIKey(t *testing.T) {
	p := NewProxy(&account.Account{APIKey: "sk-test"})
	if p.telemetry == nil {
		t.Fatal("telemetry should not be nil")
	}

	want := uuid.NewSHA1(uuid.NameSpaceDNS, []byte("sk-test")).String()
	if p.telemetry.userID != want {
		t.Fatalf("telemetry user id = %q, want %q", p.telemetry.userID, want)
	}
}

func TestNewProxyTelemetryUserIDFallbackSessionID(t *testing.T) {
	p := NewProxy(&account.Account{})
	if p.telemetry == nil {
		t.Fatal("telemetry should not be nil")
	}

	want := uuid.NewSHA1(uuid.NameSpaceDNS, []byte(p.headerBuilder.sessionID)).String()
	if p.telemetry.userID != want {
		t.Fatalf("telemetry user id = %q, want %q", p.telemetry.userID, want)
	}
}

func TestChatCompletionsGzipResponse(t *testing.T) {
	acct := &account.Account{
		APIKey:  "sk-test",
		BaseURL: "https://apis.iflow.cn/v1",
	}
	p := NewProxy(acct)
	p.telemetry = nil

	p.client = &http.Client{
		Transport: proxyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			var compressed bytes.Buffer
			zw := gzip.NewWriter(&compressed)
			_, _ = zw.Write([]byte(`{
			  "id":"chatcmpl-1",
			  "object":"chat.completion",
			  "created":1700000000,
			  "model":"glm-5",
			  "choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]
			}`))
			_ = zw.Close()

			resp := newProxyResponse(http.StatusOK, compressed.String())
			resp.Header.Set("Content-Encoding", "gzip")
			return resp, nil
		}),
	}

	resp, err := p.ChatCompletions(context.Background(), &types.ChatCompletionRequest{
		Model: "glm-5",
		Messages: []types.Message{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletions gzip response error: %v", err)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message == nil || resp.Choices[0].Message.Content != "ok" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
