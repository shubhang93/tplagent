package agent

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"github.com/shubhang93/tplagent/internal/actionable"
	"github.com/shubhang93/tplagent/internal/cmdexec"
	"github.com/shubhang93/tplagent/internal/render"
	"log/slog"
	"os"
	"sync"
	"time"
)

const defaultExecTimeout = 60 * time.Second

type sinkExecConfig struct {
	sinkConfig
	execConfig
}

type tickFunc func(ctx context.Context, config sinkExecConfig, sink render.Sink) error

type sinkConfig struct {
	parsed          *actionable.Template
	refreshInterval time.Duration
	html            bool
	templateDelims  []string
	actions         []ActionConfig
	dest            string
	staticData      any
	name            string
	renderOnce      bool
	raw             string
	readFrom        string
}

type execConfig struct {
	cmd        string
	cmdTimeout time.Duration
}

func Run(ctx context.Context, config Config) error {
	logger := createLogger(config.Agent.LogFmt, config.Agent.LogLevel)
	logger.Info("starting agent")
	templConfig := config.TemplateSpecs
	scs := makeSinkExecConfigs(templConfig)
	return renderAndRefresh(ctx, scs, renderAndExec, logger)
}

func makeSinkExecConfigs(templConfig map[string]*TemplateConfig) []sinkExecConfig {
	var i int
	var scs = make([]sinkExecConfig, len(templConfig))
	for name := range templConfig {
		spec := templConfig[name]
		scs[i] = sinkExecConfig{
			sinkConfig: sinkConfig{
				refreshInterval: time.Duration(spec.RefreshInterval),
				html:            spec.HTML,
				templateDelims:  spec.TemplateDelimiters,
				actions:         spec.Actions,
				readFrom:        os.ExpandEnv(spec.Source),
				dest:            os.ExpandEnv(spec.Destination),
				staticData:      spec.StaticData,
				name:            name,
				renderOnce:      spec.RenderOnce,
				raw:             spec.Raw,
			},
			execConfig: execConfig{
				cmd:        spec.ExecCMD,
				cmdTimeout: cmp.Or(time.Duration(spec.ExecTimeout), defaultExecTimeout),
			},
		}
		i++
	}
	return scs
}

type errMap map[string]error

func (e errMap) Error() string {
	var errs []error
	for k, err := range e {
		errs = append(errs, fmt.Errorf("%s:%w", k, err))
	}
	return errors.Join(errs...).Error()
}

func renderAndRefresh(ctx context.Context, scs []sinkExecConfig, tf tickFunc, l *slog.Logger) error {
	var wg sync.WaitGroup

	initErrors := errMap{}
	for i := range scs {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sc := scs[idx]
			if err := initTemplate(&sc); err != nil {
				initErrors[sc.name] = fmt.Errorf("init template error for %s:%w", sc.name, err)
				l.Error("init template error", slog.String("error", err.Error()), slog.String("name", sc.name))
				return
			}
			startRenderLoop(ctx, sc, tf, l)
		}(i)
	}

	wg.Wait()
	if len(initErrors) < 1 {
		return nil
	}
	return initErrors
}

func initTemplate(sc *sinkExecConfig) error {
	at := actionable.NewTemplate(sc.name, sc.html)
	setTemplateDelims(at, sc.templateDelims)
	if err := attachActions(at, sc.actions); err != nil {
		return err
	}
	sc.parsed = at
	return parseTemplate(sc.raw, sc.readFrom, sc.parsed)
}

func startRenderLoop(ctx context.Context, cfg sinkExecConfig, onTick func(context.Context, sinkExecConfig, render.Sink) error, logger *slog.Logger) {
	ticker := time.NewTicker(cfg.refreshInterval)
	tick := ticker.C

	defer ticker.Stop()

	sink := render.Sink{
		Templ:   cfg.parsed,
		WriteTo: cfg.dest,
	}

	if cfg.renderOnce {
		if err := onTick(ctx, cfg, sink); err != nil {
			logger.Error("renderAndExec error", slog.String("error", err.Error()), slog.String("loop", cfg.name), slog.Bool("once", true))
		}
		return
	}

	failureCount := 0
	for {
		select {
		case <-ctx.Done():
			logger.Info("stopping render sink", slog.String("sink", cfg.name), slog.String("cause", ctx.Err().Error()))
			return
		case <-tick:
			err := onTick(ctx, cfg, sink)
			if err != nil {
				failureCount++
				logger.Error(
					"renderAndExec error",
					slog.String("error", err.Error()),
					slog.String("loop", cfg.name),
					slog.Int("consecutive-fail-count", failureCount),
				)
				continue
			}
			// reset failure count
			failureCount = 0
		}
	}

}

func renderAndExec(ctx context.Context, cfg sinkExecConfig, sink render.Sink) error {
	err := sink.Render(cfg.staticData)
	if err != nil {
		return err
	}

	if cfg.cmd == "" {
		return nil
	}

	cmdCtx, cancel := context.WithTimeout(ctx, cfg.cmdTimeout)
	defer cancel()
	if err := cmdexec.Do(cmdCtx, cfg.cmd); err != nil {
		return err
	}

	return nil

}

func createLogger(fmt string, level slog.Level) *slog.Logger {
	if fmt == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:       level,
			ReplaceAttr: replacer,
		}))
	}

	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: replacer,
	}))
}

var replacer = func(groups []string, a slog.Attr) slog.Attr {
	if a.Key == "time" {
		return slog.String(a.Key, a.Value.Time().Format(time.DateTime))
	}
	return a
}
