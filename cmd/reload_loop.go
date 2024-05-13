package main

import (
	"context"
	"github.com/shubhang93/tplagent/internal/config"
	"github.com/shubhang93/tplagent/internal/fatal"
	"os"
	"os/signal"
	"syscall"
)

type launcherFunc func(ctx context.Context, conf config.TPLAgent, reload bool) error

type procStarters struct {
	listener launcherFunc
	agent    launcherFunc
}

func reloadProcs(root context.Context, configPath string, starters procStarters) error {
	ctx, cancel := context.WithCancelCause(root)
	defer cancel(nil)

	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)

	conf, err := config.ReadFromFile(configPath)
	if err != nil {
		return err
	}

	serverDone := make(chan struct{}, 1)
	go launchListener(ctx, starters.listener, conf, false, serverDone)

	agentErrCh := make(chan error, 1)
	go launchAgent(ctx, starters.agent, conf, false, agentErrCh)

	for {
		select {
		case <-sighup:
			cancel(sighupReceived)

			// wait for err from the
			// last agent goroutine
			err := <-agentErrCh
			if fatal.Is(err) {
				// agent had a fatal error
				// exit the loop
				<-serverDone
				return err
			}

			// wait for server to exit
			<-serverDone

			// reset all the sync
			// primitives

			ctx, cancel = context.WithCancelCause(root)
			conf, err := config.ReadFromFile(configPath)
			if err != nil {
				return err
			}

			go launchListener(ctx, starters.listener, conf, true, serverDone)

			agentErrCh = make(chan error, 1)
			go launchAgent(ctx, starters.agent, conf, true, agentErrCh)
		case err := <-agentErrCh:
			if fatal.Is(err) {
				cancel(err)
				// wait for server and
				// exit
				<-serverDone
				return err
			}
		// for non-fatal errors
		// server goroutine
		// is kept running
		// to allow reloads
		case <-ctx.Done():
			err := <-agentErrCh
			<-serverDone
			return err
		}
	}
}

func launchAgent(ctx context.Context, lf launcherFunc, conf config.TPLAgent, reloaded bool, errCh chan<- error) {
	err := lf(ctx, conf, reloaded)
	errCh <- err
	// channel is closed
	// so that only one of
	// select cases can read the error
	// and the second read returns
	// immediately
	close(errCh)
}

func launchListener(ctx context.Context, lf launcherFunc, conf config.TPLAgent, reloaded bool, doneCh chan<- struct{}) {
	_ = lf(ctx, conf, reloaded)
	doneCh <- struct{}{}
}
