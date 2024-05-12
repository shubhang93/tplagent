package main

import (
	"context"
	"github.com/shubhang93/tplagent/internal/config"
	"github.com/shubhang93/tplagent/internal/fatal"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type procStarters struct {
	listener func(ctx context.Context, conf config.TPLAgent, reload bool) error
	agent    func(ctx context.Context, conf config.TPLAgent, reload bool) error
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

	lisDone := make(chan struct{})
	go func() {
		defer close(lisDone)
		_ = starters.listener(ctx, conf, false)
	}()

	errCh := make(chan error, 1)
	var agentWG sync.WaitGroup
	agentWG.Add(1)
	go func() {
		errCh <- starters.agent(ctx, conf, false)
		agentWG.Done()
	}()

	var lastAgentErr error
	for {
		select {
		case <-sighup:
			cancel(sighupReceived)
			<-lisDone
			agentWG.Wait()

			ctx, cancel = context.WithCancelCause(root)
			agentWG.Add(1)
			go func() {
				errCh <- starters.agent(ctx, conf, true)
				agentWG.Done()
			}()
		case err := <-errCh:
			lastAgentErr = err
			if fatal.Is(err) {
				cancel(err)
				return err
			}
		case <-ctx.Done():
			<-lisDone
			agentWG.Wait()
			return lastAgentErr
		}
	}
}
