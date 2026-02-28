package oauth

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rogeecn/iflow-go/internal/account"
	"github.com/rs/zerolog/log"
)

const (
	defaultCheckInterval = 6 * time.Hour
	defaultRefreshBuffer = 24 * time.Hour
)

type Refresher struct {
	manager       *account.Manager
	client        *Client
	checkInterval time.Duration
	refreshBuffer time.Duration
	stopChan      chan struct{}
	doneChan      chan struct{}

	mu      sync.Mutex
	running bool
}

func NewRefresher(manager *account.Manager) *Refresher {
	if manager == nil {
		dataDir := strings.TrimSpace(os.Getenv("IFLOW_DATA_DIR"))
		if dataDir == "" {
			dataDir = defaultDataDir
		}
		manager = account.NewManager(dataDir)
	}

	return &Refresher{
		manager:       manager,
		client:        NewClientWithManager(manager),
		checkInterval: defaultCheckInterval,
		refreshBuffer: defaultRefreshBuffer,
		stopChan:      make(chan struct{}),
		doneChan:      make(chan struct{}),
	}
}

func (r *Refresher) Start() {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		log.Debug().Msg("oauth refresher: start ignored because it is already running")
		return
	}
	r.running = true
	r.stopChan = make(chan struct{})
	r.doneChan = make(chan struct{})
	r.mu.Unlock()

	log.Info().
		Dur("check_interval", r.checkInterval).
		Dur("refresh_buffer", r.refreshBuffer).
		Msg("oauth refresher: started")
	go r.loop()
}

func (r *Refresher) Stop() {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		log.Debug().Msg("oauth refresher: stop ignored because it is not running")
		return
	}
	r.running = false
	stopChan := r.stopChan
	doneChan := r.doneChan
	r.mu.Unlock()

	close(stopChan)
	<-doneChan
	log.Info().Msg("oauth refresher: stopped")
}

func (r *Refresher) shouldRefresh(acct *account.Account) bool {
	if acct == nil {
		return false
	}
	if strings.TrimSpace(acct.OAuthRefreshToken) == "" {
		return false
	}
	if acct.OAuthExpiresAt.IsZero() {
		return false
	}
	return time.Until(acct.OAuthExpiresAt) <= r.refreshBuffer
}

func (r *Refresher) loop() {
	defer close(r.doneChan)

	r.refreshOnce()

	ticker := time.NewTicker(r.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.refreshOnce()
		case <-r.stopChan:
			return
		}
	}
}

func (r *Refresher) refreshOnce() {
	accounts, err := r.manager.List()
	if err != nil {
		log.Warn().Err(err).Msg("oauth refresher: list accounts failed")
		return
	}

	candidates := 0
	refreshed := 0

	for _, acct := range accounts {
		if !r.shouldRefresh(acct) {
			continue
		}
		candidates++

		token, err := r.client.Refresh(context.Background(), acct.OAuthRefreshToken)
		if err != nil {
			log.Warn().
				Err(err).
				Str("uuid", acct.UUID).
				Msg("oauth refresher: refresh token failed")
			continue
		}

		expiresAt := token.ExpiresAt
		if expiresAt.IsZero() {
			expiresAt = acct.OAuthExpiresAt
		}

		if err := r.manager.UpdateToken(acct.UUID, token.AccessToken, token.RefreshToken, expiresAt); err != nil {
			log.Warn().
				Err(err).
				Str("uuid", acct.UUID).
				Msg("oauth refresher: update account token failed")
			continue
		}

		log.Info().
			Str("uuid", acct.UUID).
			Msg("oauth refresher: token refreshed")
		refreshed++
	}

	log.Debug().
		Int("accounts", len(accounts)).
		Int("candidates", candidates).
		Int("refreshed", refreshed).
		Msg("oauth refresher: cycle completed")
}
