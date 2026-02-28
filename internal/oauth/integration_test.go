package oauth

import (
	"net/http"
	"testing"
	"time"

	"github.com/rogeecn/iflow-go/internal/account"
)

func TestIntegrationTokenRefreshFlow(t *testing.T) {
	manager := account.NewManager(t.TempDir())
	acct, err := manager.Create("sk-integration", "")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := manager.UpdateToken(acct.UUID, "old-access", "old-refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("seed token: %v", err)
	}

	refresher := NewRefresher(manager)
	refresher.refreshBuffer = 2 * time.Hour
	refresher.client.tokenURL = "https://example.com/oauth/token"
	refresher.client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return newJSONResponse(http.StatusOK, `{"access_token":"integration-access","refresh_token":"integration-refresh","expires_in":3600}`), nil
		}),
	}

	refresher.refreshOnce()

	updated, err := manager.Get(acct.UUID)
	if err != nil {
		t.Fatalf("reload account: %v", err)
	}
	if updated.OAuthAccessToken != "integration-access" {
		t.Fatalf("access token = %q, want integration-access", updated.OAuthAccessToken)
	}
	if updated.OAuthRefreshToken != "integration-refresh" {
		t.Fatalf("refresh token = %q, want integration-refresh", updated.OAuthRefreshToken)
	}
}
