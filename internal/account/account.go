package account

import "time"

type Account struct {
	UUID              string    `json:"uuid"`
	APIKey            string    `json:"api_key"`
	BaseURL           string    `json:"base_url"`
	AuthType          string    `json:"auth_type"`
	OAuthAccessToken  string    `json:"oauth_access_token,omitempty"`
	OAuthRefreshToken string    `json:"oauth_refresh_token,omitempty"`
	OAuthExpiresAt    time.Time `json:"oauth_expires_at,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	LastUsedAt        time.Time `json:"last_used_at,omitempty"`
	RequestCount      int       `json:"request_count"`
}
