package main

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/shubhang93/tplagent/internal/agent"
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

type semToken struct{}

func main() {

	configPath := flag.String("config", "/etc/tplagent/config.json", "-config=/path/to/config.json")
	flag.Parse()

	pid := os.Getpid()
	writePID(pid)
	defer func() {
		_ = os.Remove(pidDir)
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	processMaker := func(l *slog.Logger) agentProcess {
		return &agent.Process{Logger: l}
	}

	spawnAndReload(ctx, processMaker, *configPath)

}

type agentProcess interface {
	Start(context.Context, agent.Config) error
}

func spawnAndReload(rootCtx context.Context, processMaker func(logger *slog.Logger) agentProcess, configPath string) {
	ctx, cancel := context.WithCancelCause(rootCtx)
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)

	sem := make(chan semToken, 1)
	spawnErrChan := make(chan error, 1)

	acquireSem(sem)
	go func() {
		err := spawn(ctx, processMaker, configPath, false)
		spawnErrChan <- err
		releaseSem(sem)
	}()

	run := true
	for run {
		select {
		case <-sighup:
			cancel(sighupReceived)
			// spawn a new agent to reload config
			acquireSem(sem)
			ctx, cancel = context.WithCancelCause(rootCtx)
			go func() {
				err := spawn(ctx, processMaker, configPath, true)
				spawnErrChan <- err
				releaseSem(sem)
			}()
		case err := <-spawnErrChan:
			if isFatal(err) {
				_, _ = fmt.Fprintf(os.Stderr, "spawn returned a fatal error %s", err.Error())
				run = false
				cancel(err)
			}
		case <-ctx.Done():
			cancel(ctx.Err())
			// acquiring sem ensures that
			// the last spawned process
			// has completed its execution
			acquireSem(sem)
			run = false
		}
	}
}

func spawn(ctx context.Context, processMaker func(logger *slog.Logger) agentProcess, confPath string, isReload bool) error {

	config, err := agent.ReadConfigFromFile(confPath)
	if err != nil {
		return err
	}

	logFmt := cmp.Or(config.Agent.LogFmt, "text")
	logger := newLogger(logFmt, config.Agent.LogLevel)

	if isReload {
		logger.Info("config reloaded")
	}

	proc := processMaker(logger)
	err = proc.Start(ctx, config)
	if err != nil {
		logger.Error("agent exited with error", slog.String("error", err.Error()))
		return err
	}
	logger.Info("agent exited without errors")
	return nil
}

func acquireSem(sem chan semToken) {
	sem <- semToken{}
}

func releaseSem(sem chan semToken) {
	<-sem
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

func isFatal(err error) bool {
	errFatal, ok := err.(interface{ Fatal() bool })
	return ok && errFatal.Fatal()
}
