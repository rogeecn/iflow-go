package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rogeecn/iflow-go/internal/account"
	"github.com/rogeecn/iflow-go/pkg/types"
)

func TestIntegrationRequestFlow(t *testing.T) {
	s := newTestServer(t)
	acct := createTestAccount(t, s)

	finish := "stop"
	s.newProxy = func(*account.Account) proxyClient {
		return &fakeProxy{
			chatResp: &types.ChatCompletionResponse{
				ID:      "chat-integration",
				Object:  "chat.completion",
				Created: 1700000000,
				Model:   "glm-5",
				Choices: []types.Choice{
					{
						Index: 0,
						Message: &types.Message{
							Role:    "assistant",
							Content: "integration-ok",
						},
						FinishReason: &finish,
					},
				},
				Usage: types.Usage{},
			},
		}
	}

	body := `{"model":"glm-5","messages":[{"role":"user","content":"integration"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+acct.UUID)
	rec := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(rec, req.WithContext(context.Background()))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "integration-ok") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}
