package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"github.com/shubhang93/tplagent/internal/agent"
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

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	err := startCLI(ctx, os.Stdout, os.Args[1:]...)
	if err != nil && notCtxErr(err) {
		_, _ = fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func startAgent(ctx context.Context, configFilePath string) error {
	pid := os.Getpid()
	writePID(pid)
	defer func() {
		_ = os.Remove(fmt.Sprintf("%s/%s", pidDir, pidFilename))
	}()

	processMaker := func(l *slog.Logger) agentProcess {
		return &agent.Process{Logger: l}
	}
	return spawnAndReload(ctx, processMaker, configFilePath)

}

type agentProcess interface {
	Start(context.Context, agent.Config) error
}

func spawnAndReload(rootCtx context.Context, processMaker func(logger *slog.Logger) agentProcess, configPath string) error {
	ctx, cancel := context.WithCancelCause(rootCtx)
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)

	spawnErrChan := make(chan error, 1)

	go func() {
		err := spawn(ctx, processMaker, configPath, false)
		spawnErrChan <- err
	}()

	var spawnErr error
	run := true
	for run {
		select {
		case <-sighup:
			cancel(sighupReceived)
			err := <-spawnErrChan
			if fatal.Is(err) {
				spawnErr = err
				run = false
				break
			}

			ctx, cancel = context.WithCancelCause(rootCtx)
			go func() {
				err := spawn(ctx, processMaker, configPath, true)
				spawnErrChan <- err
			}()
		case err := <-spawnErrChan:
			if fatal.Is(err) {
				spawnErr = err
				run = false
				cancel(err)
			}
		case <-ctx.Done():
			err := <-spawnErrChan
			if err != nil {
				spawnErr = err
			}
			cancel(ctx.Err())
			// acquiring sem ensures that
			// the last spawned process
			// has completed its execution
			run = false
		}
	}
	return spawnErr
}

func spawn(ctx context.Context, processMaker func(logger *slog.Logger) agentProcess, confPath string, isReload bool) error {

	config, err := agent.ReadConfigFromFile(confPath)
	if err != nil {
		return err
	}

	logFmt := cmp.Or(config.Agent.LogFmt, "text")
	logger := newLogger(logFmt, config.Agent.LogLevel)

	if isReload {
		logger.Info("agent reloading")
	}

	logger.Info("starting agent")

	proc := processMaker(logger)
	err = proc.Start(ctx, config)
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

func notCtxErr(err error) bool {
	return !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, context.Canceled)
}