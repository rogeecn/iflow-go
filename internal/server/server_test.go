package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rogeecn/iflow-go/internal/account"
	"github.com/rogeecn/iflow-go/internal/config"
	"github.com/rogeecn/iflow-go/internal/proxy"
	"github.com/rogeecn/iflow-go/pkg/types"
)

type fakeProxy struct {
	models    []proxy.ModelConfig
	chatResp  *types.ChatCompletionResponse
	chatErr   error
	stream    <-chan []byte
	streamErr error
}

func (f *fakeProxy) ChatCompletions(_ context.Context, _ *types.ChatCompletionRequest) (*types.ChatCompletionResponse, error) {
	if f.chatErr != nil {
		return nil, f.chatErr
	}
	return f.chatResp, nil
}

func (f *fakeProxy) ChatCompletionsStream(_ context.Context, _ *types.ChatCompletionRequest) (<-chan []byte, error) {
	if f.streamErr != nil {
		return nil, f.streamErr
	}
	return f.stream, nil
}

func (f *fakeProxy) Models() []proxy.ModelConfig {
	return f.models
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	cfg := &config.Config{
		Host:    "127.0.0.1",
		Port:    28000,
		DataDir: t.TempDir(),
	}
	return New(cfg)
}

func createTestAccount(t *testing.T, s *Server) *account.Account {
	t.Helper()

	acct, err := s.accountMgr.Create("sk-test", "")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	return acct
}

func TestServerStartStop(t *testing.T) {
	s := newTestServer(t)

	startCalled := false
	s.serveFn = func() error {
		startCalled = true
		return http.ErrServerClosed
	}

	if err := s.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if !startCalled {
		t.Fatal("serveFn should be called")
	}

	stopCalled := false
	s.shutdownFn = func(_ context.Context) error {
		stopCalled = true
		return nil
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	if !stopCalled {
		t.Fatal("shutdownFn should be called")
	}

	s.shutdownFn = func(_ context.Context) error {
		return http.ErrServerClosed
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() should ignore ErrServerClosed, got: %v", err)
	}
}

func TestHandleHealth(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestHandleModelsRequiresAuth(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleModels(t *testing.T) {
	s := newTestServer(t)
	acct := createTestAccount(t, s)

	s.newProxy = func(*account.Account) proxyClient {
		return &fakeProxy{
			models: []proxy.ModelConfig{
				{ID: "glm-5", Name: "GLM-5", Description: "desc", SupportsVision: true},
			},
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+acct.UUID)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"object":"list"`) {
		t.Fatalf("body missing list object: %s", body)
	}
	if !strings.Contains(body, `"id":"glm-5"`) {
		t.Fatalf("body missing glm-5: %s", body)
	}
}

func TestHandleChatCompletions(t *testing.T) {
	s := newTestServer(t)
	acct := createTestAccount(t, s)

	finish := "stop"
	s.newProxy = func(*account.Account) proxyClient {
		return &fakeProxy{
			chatResp: &types.ChatCompletionResponse{
				ID:      "chat-1",
				Object:  "chat.completion",
				Created: 1700000000,
				Model:   "glm-5",
				Choices: []types.Choice{
					{
						Index: 0,
						Message: &types.Message{
							Role:    "assistant",
							Content: "ok",
						},
						FinishReason: &finish,
					},
				},
				Usage: types.Usage{},
			},
		}
	}

	body := `{"model":"glm-5","messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+acct.UUID)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"content":"ok"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}

	updated, err := s.accountMgr.Get(acct.UUID)
	if err != nil {
		t.Fatalf("reload account: %v", err)
	}
	if updated.RequestCount != 1 {
		t.Fatalf("request_count = %d, want 1", updated.RequestCount)
	}
}

func TestHandleChatCompletionsStream(t *testing.T) {
	s := newTestServer(t)
	acct := createTestAccount(t, s)

	ch := make(chan []byte, 3)
	ch <- []byte("data: {\"id\":\"chunk-1\",\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
	ch <- []byte("data: [DONE]\n\n")
	close(ch)

	s.newProxy = func(*account.Account) proxyClient {
		return &fakeProxy{stream: ch}
	}

	body := `{"model":"glm-5","messages":[{"role":"user","content":"hello"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+acct.UUID)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("unexpected Content-Type: %s", rec.Header().Get("Content-Type"))
	}
	respBody := rec.Body.String()
	if !strings.Contains(respBody, `data: {"id":"chunk-1","choices":[{"delta":{"content":"hello"}}]}`) {
		t.Fatalf("unexpected stream body: %s", respBody)
	}
	if !strings.Contains(respBody, "data: [DONE]") {
		t.Fatalf("stream missing done marker: %s", respBody)
	}
}

func TestHandleChatCompletionsBodyLimit(t *testing.T) {
	s := newTestServer(t)
	acct := createTestAccount(t, s)

	oversized := strings.Repeat("a", (4<<20)+1024)
	body := `{"model":"glm-5","messages":[{"role":"user","content":"` + oversized + `"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+acct.UUID)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}
