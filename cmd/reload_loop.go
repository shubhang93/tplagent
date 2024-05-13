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

	var serverWG sync.WaitGroup
	serverWG.Add(1)
	go func() {
		defer serverWG.Done()
		_ = starters.listener(ctx, conf, false)
	}()

	agentErrCh := make(chan error, 1)
	var agentWG sync.WaitGroup
	agentWG.Add(1)
	go func() {
		agentErrCh <- starters.agent(ctx, conf, false)
		agentWG.Done()
	}()

	var lastAgentErr error
	for {
		select {
		case <-sighup:
			cancel(sighupReceived)
			serverWG.Wait()
			agentWG.Wait()

			newConf, err := config.ReadFromFile(configPath)
			if err != nil {
				return err
			}
			ctx, cancel = context.WithCancelCause(root)

			serverWG.Add(1)
			go func() {
				_ = starters.listener(ctx, conf, true)
				serverWG.Done()
			}()

			agentWG.Add(1)
			go func() {
				agentErrCh <- starters.agent(ctx, newConf, true)
				agentWG.Done()
			}()
		case err := <-agentErrCh:
			agentWG.Wait()
			lastAgentErr = err
			if fatal.Is(err) {
				cancel(err)
				return err
			}
		case <-ctx.Done():
			serverWG.Wait()
			agentWG.Wait()
			return lastAgentErr
		}
	}
}
