package account

import (
	"testing"
	"time"
)

func TestManagerLifecycle(t *testing.T) {
	manager := NewManager(t.TempDir())

	created, err := manager.Create("sk-test", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.BaseURL != defaultBaseURL {
		t.Fatalf("BaseURL = %q, want %q", created.BaseURL, defaultBaseURL)
	}

	got, err := manager.Get(created.UUID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.UUID != created.UUID {
		t.Fatalf("UUID = %q, want %q", got.UUID, created.UUID)
	}

	if err := manager.UpdateUsage(created.UUID); err != nil {
		t.Fatalf("UpdateUsage() error = %v", err)
	}
	used, err := manager.Get(created.UUID)
	if err != nil {
		t.Fatalf("Get() after UpdateUsage error = %v", err)
	}
	if used.RequestCount != 1 {
		t.Fatalf("RequestCount = %d, want 1", used.RequestCount)
	}
	if used.LastUsedAt.IsZero() {
		t.Fatal("LastUsedAt is zero, want non-zero")
	}

	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	if err := manager.UpdateToken(created.UUID, "access-token", "refresh-token", expiresAt); err != nil {
		t.Fatalf("UpdateToken() error = %v", err)
	}
	tokenUpdated, err := manager.Get(created.UUID)
	if err != nil {
		t.Fatalf("Get() after UpdateToken error = %v", err)
	}
	if tokenUpdated.OAuthAccessToken != "access-token" {
		t.Fatalf("OAuthAccessToken = %q, want %q", tokenUpdated.OAuthAccessToken, "access-token")
	}
	if tokenUpdated.OAuthRefreshToken != "refresh-token" {
		t.Fatalf("OAuthRefreshToken = %q, want %q", tokenUpdated.OAuthRefreshToken, "refresh-token")
	}
	if !tokenUpdated.OAuthExpiresAt.Equal(expiresAt) {
		t.Fatalf("OAuthExpiresAt = %s, want %s", tokenUpdated.OAuthExpiresAt, expiresAt)
	}

	accounts, err := manager.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("len(List()) = %d, want 1", len(accounts))
	}

	if err := manager.Delete(created.UUID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}
