package oauth

import (
	"net/http"
	"testing"
	"time"

	"github.com/rogeecn/iflow-go/internal/account"
)

func TestShouldRefresh(t *testing.T) {
	refresher := &Refresher{
		refreshBuffer: 24 * time.Hour,
	}

	if refresher.shouldRefresh(nil) {
		t.Fatal("shouldRefresh(nil) = true, want false")
	}

	if refresher.shouldRefresh(&account.Account{
		OAuthRefreshToken: "",
		OAuthExpiresAt:    time.Now().Add(time.Hour),
	}) {
		t.Fatal("shouldRefresh(no refresh token) = true, want false")
	}

	if refresher.shouldRefresh(&account.Account{
		OAuthRefreshToken: "refresh",
		OAuthExpiresAt:    time.Time{},
	}) {
		t.Fatal("shouldRefresh(zero expires) = true, want false")
	}

	if !refresher.shouldRefresh(&account.Account{
		OAuthRefreshToken: "refresh",
		OAuthExpiresAt:    time.Now().Add(2 * time.Hour),
	}) {
		t.Fatal("shouldRefresh(expiring soon) = false, want true")
	}

	if refresher.shouldRefresh(&account.Account{
		OAuthRefreshToken: "refresh",
		OAuthExpiresAt:    time.Now().Add(48 * time.Hour),
	}) {
		t.Fatal("shouldRefresh(expiring late) = true, want false")
	}
}

func TestRefreshOnceUpdatesToken(t *testing.T) {
	manager := account.NewManager(t.TempDir())
	acct, err := manager.Create("sk-old", "")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := manager.UpdateToken(acct.UUID, "old-access", "old-refresh", time.Now().Add(2*time.Hour)); err != nil {
		t.Fatalf("seed token: %v", err)
	}

	refresher := NewRefresher(manager)
	refresher.refreshBuffer = 24 * time.Hour
	refresher.client.tokenURL = "https://example.com/oauth/token"
	refresher.client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm error: %v", err)
			}
			if r.Form.Get("grant_type") != "refresh_token" {
				t.Fatalf("grant_type = %q", r.Form.Get("grant_type"))
			}
			if r.Form.Get("refresh_token") != "old-refresh" {
				t.Fatalf("refresh_token = %q", r.Form.Get("refresh_token"))
			}
			return newJSONResponse(http.StatusOK, `{"access_token":"new-access","refresh_token":"new-refresh","expires_in":7200}`), nil
		}),
	}

	refresher.refreshOnce()

	updated, err := manager.Get(acct.UUID)
	if err != nil {
		t.Fatalf("load updated account: %v", err)
	}
	if updated.OAuthAccessToken != "new-access" {
		t.Fatalf("access token = %q", updated.OAuthAccessToken)
	}
	if updated.OAuthRefreshToken != "new-refresh" {
		t.Fatalf("refresh token = %q", updated.OAuthRefreshToken)
	}
	if time.Until(updated.OAuthExpiresAt) <= time.Hour {
		t.Fatalf("expires_at not updated, got %s", updated.OAuthExpiresAt)
	}
}

func TestRefresherStartStop(t *testing.T) {
	manager := account.NewManager(t.TempDir())
	refresher := NewRefresher(manager)
	refresher.checkInterval = 24 * time.Hour
	refresher.refreshBuffer = time.Hour

	refresher.Start()
	refresher.Start() // idempotent
	refresher.Stop()
	refresher.Stop() // idempotent
}
