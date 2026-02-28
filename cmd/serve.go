package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rogeecn/iflow-go/internal/config"
	"github.com/rogeecn/iflow-go/internal/server"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

type serveRunner interface {
	Start() error
	Stop(ctx context.Context) error
}

var (
	serveHost        string
	servePort        int
	serveConcurrency int
)

var (
	newServeServer = func(cfg *config.Config) serveRunner {
		return server.New(cfg)
	}
	signalNotifyContext = signal.NotifyContext
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "启动代理服务",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVar(&serveHost, "host", "", "监听地址 (默认: 从 IFLOW_HOST 读取)")
	serveCmd.Flags().IntVar(&servePort, "port", 0, "监听端口 (默认: 从 IFLOW_PORT 读取)")
	serveCmd.Flags().IntVar(&serveConcurrency, "concurrency", 0, "并发数 (默认: 从 IFLOW_CONCURRENCY 读取)")
}

func runServe(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if serveHost != "" {
		cfg.Host = serveHost
	}
	if servePort > 0 {
		cfg.Port = servePort
	}
	if serveConcurrency > 0 {
		cfg.Concurrency = serveConcurrency
	}

	log.Logger = config.InitLogger(cfg.LogLevel)

	srv := newServeServer(cfg)
	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- srv.Start()
	}()

	ctx, stop := signalNotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-startErrCh:
		return err
	case <-ctx.Done():
		log.Info().Msg("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Stop(shutdownCtx); err != nil && err != http.ErrServerClosed {
			return err
		}

		select {
		case err := <-startErrCh:
			return err
		case <-time.After(10 * time.Second):
			return fmt.Errorf("shutdown timeout")
		}
	}
}
