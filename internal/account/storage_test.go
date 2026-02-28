package account

import (
	"testing"
	"time"
)

func TestStorageCRUD(t *testing.T) {
	storage := NewStorage(t.TempDir())

	account := &Account{
		UUID:         GenerateUUID(),
		APIKey:       "sk-test",
		BaseURL:      "https://apis.iflow.cn/v1",
		AuthType:     "oauth-iflow",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		RequestCount: 0,
	}

	if err := storage.Save(account); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if !storage.Exists(account.UUID) {
		t.Fatalf("Exists() = false, want true")
	}

	loaded, err := storage.Load(account.UUID)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.APIKey != account.APIKey {
		t.Fatalf("APIKey = %q, want %q", loaded.APIKey, account.APIKey)
	}

	accounts, err := storage.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("len(List()) = %d, want 1", len(accounts))
	}

	if err := storage.Delete(account.UUID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if storage.Exists(account.UUID) {
		t.Fatalf("Exists() = true, want false")
	}
}
