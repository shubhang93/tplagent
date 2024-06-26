package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/shubhang93/tplagent/internal/agent"
	"github.com/shubhang93/tplagent/internal/config"
	"github.com/shubhang93/tplagent/internal/httplis"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var sighupReceived = errors.New("context canceled: SIGHUP")

const pidDir = "/tmp/tplagent"
const pidFilename = "agent.pid"

func startAgent(ctx context.Context, configFilePath string) error {
	writePID(filepath.Join(pidDir, pidFilename))
	defer func() {
		_ = os.Remove(fmt.Sprintf("%s/%s", pidDir, pidFilename))
	}()

	return spawnAndReload(ctx, configFilePath)

}

func spawnAndReload(rootCtx context.Context, configPath string) error {
	starters := procStarters{
		listener: func(ctx context.Context, conf config.TPLAgent, reload bool) error {
			if conf.Agent.HTTPListenerAddr != "" {
				logFmt := conf.Agent.LogFmt
				level := conf.Agent.LogLevel
				s := httplis.Proc{
					Logger:   newLogger(logFmt, level).WithGroup("http-lis"),
					Reloaded: reload,
				}
				s.Start(ctx, conf.Agent.HTTPListenerAddr)
			}
			return nil
		},
		agent: func(ctx context.Context, conf config.TPLAgent, reload bool) error {
			logFmt := conf.Agent.LogFmt
			level := conf.Agent.LogLevel
			proc := agent.Proc{
				Logger:   newLogger(logFmt, level).WithGroup("agent"),
				TickFunc: agent.RenderAndExec,
				Reloaded: reload,
			}
			return proc.Start(ctx, conf)
		},
	}
	return reloadProcs(rootCtx, configPath, starters)

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

func writePID(path string) {
	pid := os.Getpid()
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return
	}
	bs := []byte(strconv.Itoa(pid))
	_ = os.WriteFile(path, bs, 0755)
}
