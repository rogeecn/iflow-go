package account

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Storage struct {
	dataDir string
}

func NewStorage(dataDir string) *Storage {
	return &Storage{dataDir: dataDir}
}

func (s *Storage) Save(account *Account) error {
	if account == nil {
		return fmt.Errorf("save account: nil account")
	}
	if !IsValidUUID(account.UUID) {
		return fmt.Errorf("save account: invalid uuid %q", account.UUID)
	}
	if err := s.ensureAccountsDir(); err != nil {
		return fmt.Errorf("save account: ensure accounts dir: %w", err)
	}

	payload, err := json.MarshalIndent(account, "", "  ")
	if err != nil {
		return fmt.Errorf("save account: marshal json: %w", err)
	}

	path := s.accountPath(account.UUID)
	tmpPath := path + ".tmp"

	if err := os.WriteFile(tmpPath, payload, 0o600); err != nil {
		return fmt.Errorf("save account: write temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("save account: rename temp file: %w", err)
	}

	return nil
}

func (s *Storage) Load(uuid string) (*Account, error) {
	if !IsValidUUID(uuid) {
		return nil, fmt.Errorf("load account: invalid uuid %q", uuid)
	}

	content, err := os.ReadFile(s.accountPath(uuid))
	if err != nil {
		return nil, fmt.Errorf("load account: read file: %w", err)
	}

	var account Account
	if err := json.Unmarshal(content, &account); err != nil {
		return nil, fmt.Errorf("load account: unmarshal json: %w", err)
	}

	return &account, nil
}

func (s *Storage) Delete(uuid string) error {
	if !IsValidUUID(uuid) {
		return fmt.Errorf("delete account: invalid uuid %q", uuid)
	}

	if err := os.Remove(s.accountPath(uuid)); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("delete account: remove file: %w", err)
	}

	return nil
}

func (s *Storage) List() ([]*Account, error) {
	accountsDir := s.accountsDir()
	if err := s.ensureAccountsDir(); err != nil {
		return nil, fmt.Errorf("list accounts: ensure accounts dir: %w", err)
	}

	entries, err := os.ReadDir(accountsDir)
	if err != nil {
		return nil, fmt.Errorf("list accounts: read dir: %w", err)
	}

	accounts := make([]*Account, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(accountsDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("list accounts: read %s: %w", entry.Name(), err)
		}

		var account Account
		if err := json.Unmarshal(content, &account); err != nil {
			return nil, fmt.Errorf("list accounts: parse %s: %w", entry.Name(), err)
		}

		accounts = append(accounts, &account)
	}

	sort.Slice(accounts, func(i, j int) bool {
		if accounts[i].CreatedAt.Equal(accounts[j].CreatedAt) {
			return accounts[i].UUID < accounts[j].UUID
		}
		return accounts[i].CreatedAt.Before(accounts[j].CreatedAt)
	})

	return accounts, nil
}

func (s *Storage) Exists(uuid string) bool {
	if !IsValidUUID(uuid) {
		return false
	}

	_, err := os.Stat(s.accountPath(uuid))
	return err == nil
}

func (s *Storage) ensureAccountsDir() error {
	return os.MkdirAll(s.accountsDir(), 0o755)
}

func (s *Storage) accountsDir() string {
	return filepath.Join(s.dataDir, "accounts")
}

func (s *Storage) accountPath(uuid string) string {
	return filepath.Join(s.accountsDir(), uuid+".json")
}
