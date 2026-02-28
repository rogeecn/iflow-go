package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/rogeecn/iflow-go/internal/account"
	"github.com/rogeecn/iflow-go/internal/config"
	"github.com/rogeecn/iflow-go/internal/proxy"
	"github.com/rogeecn/iflow-go/pkg/types"
	"github.com/rs/zerolog/log"
)

type proxyClient interface {
	ChatCompletions(ctx context.Context, req *types.ChatCompletionRequest) (*types.ChatCompletionResponse, error)
	ChatCompletionsStream(ctx context.Context, req *types.ChatCompletionRequest) (<-chan []byte, error)
	Models() []proxy.ModelConfig
}

type Server struct {
	config     *config.Config
	accountMgr *account.Manager
	httpServer *http.Server

	newProxy   func(acct *account.Account) proxyClient
	serveFn    func() error
	shutdownFn func(ctx context.Context) error
}

func New(cfg *config.Config) *Server {
	if cfg == nil {
		cfg = &config.Config{
			Host:    "0.0.0.0",
			Port:    28000,
			DataDir: "./data",
		}
	}

	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}
	if cfg.Port == 0 {
		cfg.Port = 28000
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "./data"
	}

	s := &Server{
		config:     cfg,
		accountMgr: account.NewManager(cfg.DataDir),
		newProxy: func(acct *account.Account) proxyClient {
			return proxy.NewProxy(acct)
		},
	}

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler: s.setupRoutes(),
	}
	s.serveFn = s.httpServer.ListenAndServe
	s.shutdownFn = s.httpServer.Shutdown

	return s
}

func (s *Server) Start() error {
	log.Info().
		Str("addr", s.httpServer.Addr).
		Msg("http server starting")

	if err := s.serveFn(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("start server: %w", err)
	}
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if err := s.shutdownFn(ctx); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("stop server: %w", err)
	}
	return nil
}
