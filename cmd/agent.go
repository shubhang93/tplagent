package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"github.com/shubhang93/tplagent/internal/agent"
	"github.com/shubhang93/tplagent/internal/config"
	"github.com/shubhang93/tplagent/internal/fatal"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

var sighupReceived = errors.New("context canceled: SIGHUP")

const pidDir = "/tmp/tplagent"
const pidFilename = "agent.pid"

type agentProcess interface {
	Start(context.Context, config.TPLAgent) error
}

func startAgent(ctx context.Context, configFilePath string) error {
	pid := os.Getpid()
	writePID(pid)
	defer func() {
		_ = os.Remove(fmt.Sprintf("%s/%s", pidDir, pidFilename))
	}()

	processMaker := func(l *slog.Logger) agentProcess {
		return &agent.Process{
			Logger:   l,
			TickFunc: agent.RenderAndExec,
		}
	}
	return spawnAndReload(ctx, processMaker, configFilePath)

}

func spawnAndReload(rootCtx context.Context, processMaker func(logger *slog.Logger) agentProcess, configPath string) error {

	conf, err := config.ReadFromFile(configPath)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancelCause(rootCtx)
	defer cancel(nil)

	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)

	spawnErrChan := make(chan error, 1)

	go func() {
		err := spawn(ctx, processMaker, conf, false)
		spawnErrChan <- err
	}()

	for {
		select {
		case <-sighup:
			cancel(sighupReceived)
			err := <-spawnErrChan
			if fatal.Is(err) {
				return err
			}

			cfg, err := config.ReadFromFile(configPath)
			if err != nil {
				return err
			}

			ctx, cancel = context.WithCancelCause(rootCtx)
			go func() {
				err := spawn(ctx, processMaker, cfg, true)
				spawnErrChan <- err
			}()
		case err := <-spawnErrChan:
			if fatal.Is(err) {
				cancel(err)
				return err
			}
		case <-ctx.Done():
			err := <-spawnErrChan
			if err != nil {
				return err
			}
			return nil
		}
	}
}

func spawn(ctx context.Context, processMaker func(logger *slog.Logger) agentProcess, conf config.TPLAgent, isReload bool) error {

	logFmt := cmp.Or(conf.Agent.LogFmt, "text")
	logger := newLogger(logFmt, conf.Agent.LogLevel)

	if isReload {
		logger.Info("reloading agent")
	} else {
		logger.Info("starting agent")
	}

	proc := processMaker(logger)
	err := proc.Start(ctx, conf)
	if err != nil {
		logger.Error("agent exited with error", slog.String("error", err.Error()))
		return err
	}
	logger.Info("agent exited without errors")
	return nil
}

var replacer = func(groups []string, a slog.Attr) slog.Attr {
	if a.Key == "time" {
		return slog.String(a.Key, a.Value.Time().Format(time.DateTime))
	}
	return a
}

func newLogger(fmt string, level slog.Level) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: replacer,
	}
	if fmt == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))

}

func writePID(pid int) {
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return
	}
	fullPath := filepath.Join(pidDir, pidFilename)
	bs := []byte(strconv.Itoa(pid))
	_ = os.WriteFile(fullPath, bs, 0755)
}
