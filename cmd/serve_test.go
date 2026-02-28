package cmd

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rogeecn/iflow-go/internal/config"
)

type fakeServeRunner struct {
	startFn func() error
	stopFn  func(ctx context.Context) error
}

func (f *fakeServeRunner) Start() error {
	if f.startFn != nil {
		return f.startFn()
	}
	return nil
}

func (f *fakeServeRunner) Stop(ctx context.Context) error {
	if f.stopFn != nil {
		return f.stopFn(ctx)
	}
	return nil
}

func TestRunServeStartReturns(t *testing.T) {
	origNewServeServer := newServeServer
	origSignalNotifyContext := signalNotifyContext
	origHost, origPort, origConcurrency := serveHost, servePort, serveConcurrency
	t.Cleanup(func() {
		newServeServer = origNewServeServer
		signalNotifyContext = origSignalNotifyContext
		serveHost, servePort, serveConcurrency = origHost, origPort, origConcurrency
	})

	t.Setenv("IFLOW_DATA_DIR", t.TempDir())
	t.Setenv("IFLOW_HOST", "0.0.0.0")
	t.Setenv("IFLOW_PORT", "28000")

	serveHost = "127.0.0.1"
	servePort = 19000
	serveConcurrency = 3

	var capturedCfg *config.Config
	newServeServer = func(cfg *config.Config) serveRunner {
		copied := *cfg
		capturedCfg = &copied
		return &fakeServeRunner{
			startFn: func() error { return nil },
		}
	}

	if err := runServe(nil, nil); err != nil {
		t.Fatalf("runServe error: %v", err)
	}
	if capturedCfg == nil {
		t.Fatal("newServeServer was not called")
	}
	if capturedCfg.Host != "127.0.0.1" || capturedCfg.Port != 19000 || capturedCfg.Concurrency != 3 {
		t.Fatalf("unexpected cfg overrides: %+v", *capturedCfg)
	}
}

func TestRunServeShutdownPath(t *testing.T) {
	origNewServeServer := newServeServer
	origSignalNotifyContext := signalNotifyContext
	origHost, origPort, origConcurrency := serveHost, servePort, serveConcurrency
	t.Cleanup(func() {
		newServeServer = origNewServeServer
		signalNotifyContext = origSignalNotifyContext
		serveHost, servePort, serveConcurrency = origHost, origPort, origConcurrency
	})

	t.Setenv("IFLOW_DATA_DIR", t.TempDir())

	stopCh := make(chan struct{})
	newServeServer = func(cfg *config.Config) serveRunner {
		return &fakeServeRunner{
			startFn: func() error {
				<-stopCh
				return nil
			},
			stopFn: func(ctx context.Context) error {
				close(stopCh)
				return nil
			},
		}
	}

	signalNotifyContext = func(parent context.Context, _ ...os.Signal) (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(parent)
		cancel()
		return ctx, func() {}
	}

	if err := runServe(nil, nil); err != nil {
		t.Fatalf("runServe shutdown path error: %v", err)
	}
}

func TestRunServeShutdownError(t *testing.T) {
	origNewServeServer := newServeServer
	origSignalNotifyContext := signalNotifyContext
	origHost, origPort, origConcurrency := serveHost, servePort, serveConcurrency
	t.Cleanup(func() {
		newServeServer = origNewServeServer
		signalNotifyContext = origSignalNotifyContext
		serveHost, servePort, serveConcurrency = origHost, origPort, origConcurrency
	})

	t.Setenv("IFLOW_DATA_DIR", t.TempDir())

	stopErr := fmt.Errorf("stop failed")
	newServeServer = func(cfg *config.Config) serveRunner {
		return &fakeServeRunner{
			startFn: func() error {
				time.Sleep(20 * time.Millisecond)
				return nil
			},
			stopFn: func(ctx context.Context) error {
				return stopErr
			},
		}
	}

	signalNotifyContext = func(parent context.Context, _ ...os.Signal) (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(parent)
		cancel()
		return ctx, func() {}
	}

	err := runServe(nil, nil)
	if err == nil {
		t.Fatal("expected shutdown error, got nil")
	}
	if err.Error() != stopErr.Error() {
		t.Fatalf("unexpected error: %v", err)
	}
}
