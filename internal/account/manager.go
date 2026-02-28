package account

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const defaultBaseURL = "https://apis.iflow.cn/v1"

type Manager struct {
	storage *Storage
	mu      sync.RWMutex
}

func NewManager(dataDir string) *Manager {
	return &Manager{
		storage: NewStorage(dataDir),
	}
}

func (m *Manager) Create(apiKey, baseURL string) (*Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	apiKey = strings.TrimSpace(apiKey)
	baseURL = strings.TrimSpace(baseURL)

	if apiKey == "" {
		return nil, fmt.Errorf("create account: api key is required")
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	now := time.Now().UTC()
	account := &Account{
		UUID:         GenerateUUID(),
		APIKey:       apiKey,
		BaseURL:      baseURL,
		AuthType:     "oauth-iflow",
		CreatedAt:    now,
		UpdatedAt:    now,
		RequestCount: 0,
	}

	if err := m.storage.Save(account); err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}

	return account, nil
}

func (m *Manager) Get(uuid string) (*Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	account, err := m.storage.Load(uuid)
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}
	return account, nil
}

func (m *Manager) Delete(uuid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.storage.Delete(uuid); err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	return nil
}

func (m *Manager) List() ([]*Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	accounts, err := m.storage.List()
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	return accounts, nil
}

func (m *Manager) UpdateUsage(uuid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	account, err := m.storage.Load(uuid)
	if err != nil {
		return fmt.Errorf("update usage: %w", err)
	}

	now := time.Now().UTC()
	account.LastUsedAt = now
	account.UpdatedAt = now
	account.RequestCount++

	if err := m.storage.Save(account); err != nil {
		return fmt.Errorf("update usage: %w", err)
	}

	return nil
}

func (m *Manager) UpdateToken(uuid string, accessToken, refreshToken string, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	account, err := m.storage.Load(uuid)
	if err != nil {
		return fmt.Errorf("update token: %w", err)
	}

	account.OAuthAccessToken = strings.TrimSpace(accessToken)
	account.OAuthRefreshToken = strings.TrimSpace(refreshToken)
	account.OAuthExpiresAt = expiresAt.UTC()
	account.UpdatedAt = time.Now().UTC()

	if err := m.storage.Save(account); err != nil {
		return fmt.Errorf("update token: %w", err)
	}

	return nil
}
