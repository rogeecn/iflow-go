package cmd

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rogeecn/iflow-go/internal/account"
	"github.com/rogeecn/iflow-go/internal/config"
)

type fakeServeRunner struct {
	startFn    func() error
	stopFn     func(ctx context.Context) error
	accountMgr *account.Manager
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

func (f *fakeServeRunner) AccountManager() *account.Manager {
	return f.accountMgr
}

type fakeServeRefresher struct {
	startCalls int
	stopCalls  int
}

func (f *fakeServeRefresher) Start() {
	f.startCalls++
}

func (f *fakeServeRefresher) Stop() {
	f.stopCalls++
}

func TestRunServeStartReturns(t *testing.T) {
	origNewServeServer := newServeServer
	origNewServeRefresher := newServeRefresher
	origSignalNotifyContext := signalNotifyContext
	origHost, origPort, origConcurrency := serveHost, servePort, serveConcurrency
	t.Cleanup(func() {
		newServeServer = origNewServeServer
		newServeRefresher = origNewServeRefresher
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
	var refresher *fakeServeRefresher
	var refresherManager *account.Manager
	expectedManager := account.NewManager(t.TempDir())
	newServeServer = func(cfg *config.Config) serveRunner {
		copied := *cfg
		capturedCfg = &copied
		return &fakeServeRunner{
			startFn:    func() error { return nil },
			accountMgr: expectedManager,
		}
	}
	newServeRefresher = func(manager *account.Manager) serveRefresher {
		refresherManager = manager
		refresher = &fakeServeRefresher{}
		return refresher
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
	if refresher == nil {
		t.Fatal("refresher was not created")
	}
	if refresher.startCalls != 1 || refresher.stopCalls != 1 {
		t.Fatalf("unexpected refresher calls: start=%d stop=%d", refresher.startCalls, refresher.stopCalls)
	}
	if refresherManager != expectedManager {
		t.Fatal("refresher should use server account manager")
	}
}

func TestRunServeShutdownPath(t *testing.T) {
	origNewServeServer := newServeServer
	origNewServeRefresher := newServeRefresher
	origSignalNotifyContext := signalNotifyContext
	origHost, origPort, origConcurrency := serveHost, servePort, serveConcurrency
	t.Cleanup(func() {
		newServeServer = origNewServeServer
		newServeRefresher = origNewServeRefresher
		signalNotifyContext = origSignalNotifyContext
		serveHost, servePort, serveConcurrency = origHost, origPort, origConcurrency
	})

	t.Setenv("IFLOW_DATA_DIR", t.TempDir())

	stopCh := make(chan struct{})
	var refresher *fakeServeRefresher
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
	newServeRefresher = func(manager *account.Manager) serveRefresher {
		refresher = &fakeServeRefresher{}
		return refresher
	}

	signalNotifyContext = func(parent context.Context, _ ...os.Signal) (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(parent)
		cancel()
		return ctx, func() {}
	}

	if err := runServe(nil, nil); err != nil {
		t.Fatalf("runServe shutdown path error: %v", err)
	}
	if refresher == nil {
		t.Fatal("refresher was not created")
	}
	if refresher.startCalls != 1 || refresher.stopCalls != 1 {
		t.Fatalf("unexpected refresher calls: start=%d stop=%d", refresher.startCalls, refresher.stopCalls)
	}
}

func TestRunServeShutdownError(t *testing.T) {
	origNewServeServer := newServeServer
	origNewServeRefresher := newServeRefresher
	origSignalNotifyContext := signalNotifyContext
	origHost, origPort, origConcurrency := serveHost, servePort, serveConcurrency
	t.Cleanup(func() {
		newServeServer = origNewServeServer
		newServeRefresher = origNewServeRefresher
		signalNotifyContext = origSignalNotifyContext
		serveHost, servePort, serveConcurrency = origHost, origPort, origConcurrency
	})

	t.Setenv("IFLOW_DATA_DIR", t.TempDir())

	stopErr := fmt.Errorf("stop failed")
	var refresher *fakeServeRefresher
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
	newServeRefresher = func(manager *account.Manager) serveRefresher {
		refresher = &fakeServeRefresher{}
		return refresher
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
	if refresher == nil {
		t.Fatal("refresher was not created")
	}
	if refresher.startCalls != 1 || refresher.stopCalls != 1 {
		t.Fatalf("unexpected refresher calls: start=%d stop=%d", refresher.startCalls, refresher.stopCalls)
	}
}
