package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rogeecn/iflow-go/internal/account"
	"github.com/rogeecn/iflow-go/internal/oauth"
)

type fakeOAuthClient struct {
	loginFn   func(ctx context.Context) (*account.Account, error)
	refreshFn func(ctx context.Context, refreshToken string) (*oauth.Token, error)
}

func (f *fakeOAuthClient) Login(ctx context.Context) (*account.Account, error) {
	if f.loginFn != nil {
		return f.loginFn(ctx)
	}
	return nil, fmt.Errorf("login not configured")
}

func (f *fakeOAuthClient) Refresh(ctx context.Context, refreshToken string) (*oauth.Token, error) {
	if f.refreshFn != nil {
		return f.refreshFn(ctx, refreshToken)
	}
	return nil, fmt.Errorf("refresh not configured")
}

func TestTokenListNoAccounts(t *testing.T) {
	t.Setenv("IFLOW_DATA_DIR", t.TempDir())

	out, err := executeForTest("token", "list")
	if err != nil {
		t.Fatalf("token list error: %v", err)
	}
	if !strings.Contains(out, "No accounts found.") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestTokenListWithAccounts(t *testing.T) {
	dataDir := t.TempDir()
	manager := account.NewManager(dataDir)
	acct, err := manager.Create("sk-test", "")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	t.Setenv("IFLOW_DATA_DIR", dataDir)
	out, err := executeForTest("token", "list")
	if err != nil {
		t.Fatalf("token list error: %v", err)
	}
	if !strings.Contains(out, acct.UUID) {
		t.Fatalf("output missing uuid: %s", out)
	}
}

func TestTokenDelete(t *testing.T) {
	dataDir := t.TempDir()
	manager := account.NewManager(dataDir)
	acct, err := manager.Create("sk-test", "")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	t.Setenv("IFLOW_DATA_DIR", dataDir)
	out, err := executeForTest("token", "delete", acct.UUID)
	if err != nil {
		t.Fatalf("token delete error: %v", err)
	}
	if !strings.Contains(out, "Account deleted") {
		t.Fatalf("unexpected output: %s", out)
	}
	if _, err := manager.Get(acct.UUID); err == nil {
		t.Fatalf("account %s should be deleted", acct.UUID)
	}
}

func TestTokenRefreshWithoutRefreshToken(t *testing.T) {
	dataDir := t.TempDir()
	manager := account.NewManager(dataDir)
	acct, err := manager.Create("sk-test", "")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	t.Setenv("IFLOW_DATA_DIR", dataDir)
	_, err = executeForTest("token", "refresh", acct.UUID)
	if err == nil {
		t.Fatal("expected missing refresh token error, got nil")
	}
	if !strings.Contains(err.Error(), "has no refresh token") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTokenImportWithInjectedOAuthClient(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("IFLOW_DATA_DIR", dataDir)

	origNewOAuthClient := newOAuthClient
	t.Cleanup(func() { newOAuthClient = origNewOAuthClient })

	newOAuthClient = func(manager *account.Manager) oauthClient {
		return &fakeOAuthClient{
			loginFn: func(ctx context.Context) (*account.Account, error) {
				acct, err := manager.Create("sk-imported", "")
				if err != nil {
					return nil, err
				}
				return manager.Get(acct.UUID)
			},
		}
	}

	out, err := executeForTest("token", "import")
	if err != nil {
		t.Fatalf("token import error: %v", err)
	}
	if !strings.Contains(out, "Account imported successfully.") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestTokenImportOAuthError(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("IFLOW_DATA_DIR", dataDir)

	origNewOAuthClient := newOAuthClient
	t.Cleanup(func() { newOAuthClient = origNewOAuthClient })

	newOAuthClient = func(manager *account.Manager) oauthClient {
		return &fakeOAuthClient{
			loginFn: func(ctx context.Context) (*account.Account, error) {
				return nil, fmt.Errorf("oauth failed")
			},
		}
	}

	_, err := executeForTest("token", "import")
	if err == nil {
		t.Fatal("expected token import error, got nil")
	}
	if !strings.Contains(err.Error(), "oauth import") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTokenRefreshWithInjectedOAuthClient(t *testing.T) {
	dataDir := t.TempDir()
	manager := account.NewManager(dataDir)
	acct, err := manager.Create("sk-test", "")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := manager.UpdateToken(acct.UUID, "old-access", "old-refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("seed token: %v", err)
	}

	t.Setenv("IFLOW_DATA_DIR", dataDir)

	origNewOAuthClient := newOAuthClient
	t.Cleanup(func() { newOAuthClient = origNewOAuthClient })

	newOAuthClient = func(manager *account.Manager) oauthClient {
		return &fakeOAuthClient{
			refreshFn: func(ctx context.Context, refreshToken string) (*oauth.Token, error) {
				if refreshToken != "old-refresh" {
					t.Fatalf("unexpected refresh token: %s", refreshToken)
				}
				return &oauth.Token{
					AccessToken:  "new-access",
					RefreshToken: "new-refresh",
					ExpiresAt:    time.Now().Add(2 * time.Hour),
				}, nil
			},
		}
	}

	out, err := executeForTest("token", "refresh", acct.UUID)
	if err != nil {
		t.Fatalf("token refresh error: %v", err)
	}
	if !strings.Contains(out, "Token refreshed") {
		t.Fatalf("unexpected output: %s", out)
	}

	updated, err := manager.Get(acct.UUID)
	if err != nil {
		t.Fatalf("reload account: %v", err)
	}
	if updated.OAuthAccessToken != "new-access" || updated.OAuthRefreshToken != "new-refresh" {
		t.Fatalf("tokens not updated: %+v", updated)
	}
}

func TestTokenImportFromSettingsFile(t *testing.T) {
	dataDir := t.TempDir()
	sourceDir := t.TempDir()

	settingsPath := filepath.Join(sourceDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{
  "apiKey": "sk-file",
  "baseUrl": "https://apis.iflow.cn/v1"
}`), 0o600); err != nil {
		t.Fatalf("write settings file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "oauth_creds.json"), []byte(`{
  "access_token": "access-file",
  "refresh_token": "refresh-file",
  "expiry_date": 1772347112327
}`), 0o600); err != nil {
		t.Fatalf("write oauth creds file: %v", err)
	}

	t.Setenv("IFLOW_DATA_DIR", dataDir)

	out, err := executeForTest("token", "import", settingsPath)
	if err != nil {
		t.Fatalf("token import from file error: %v", err)
	}
	if !strings.Contains(out, "Account imported successfully.") {
		t.Fatalf("unexpected output: %s", out)
	}

	manager := account.NewManager(dataDir)
	accounts, err := manager.List()
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("accounts len = %d, want 1", len(accounts))
	}
	if accounts[0].APIKey != "sk-file" {
		t.Fatalf("api key = %q, want sk-file", accounts[0].APIKey)
	}
	if accounts[0].OAuthAccessToken != "access-file" || accounts[0].OAuthRefreshToken != "refresh-file" {
		t.Fatalf("oauth tokens not imported: %+v", accounts[0])
	}
	if accounts[0].OAuthExpiresAt.IsZero() {
		t.Fatalf("oauth expiry should be set")
	}
}
