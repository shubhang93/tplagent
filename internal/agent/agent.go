package agent

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"github.com/shubhang93/tplagent/internal/actionable"
	"github.com/shubhang93/tplagent/internal/cmdexec"
	"github.com/shubhang93/tplagent/internal/fatal"
	"github.com/shubhang93/tplagent/internal/render"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

const defaultExecTimeout = 60 * time.Second

type sinkExecConfig struct {
	sinkConfig
	*execConfig
}

type tickFunc func(ctx context.Context, config sinkExecConfig, sink render.Sink) error

type sinkConfig struct {
	parsed          *actionable.Template
	refreshInterval time.Duration
	html            bool
	templateDelims  []string
	actions         []ActionsConfig
	dest            string
	staticData      any
	name            string
	renderOnce      bool
	raw             string
	readFrom        string
	missingKey      string
}

type execConfig struct {
	cmd        string
	cmdTimeout time.Duration
	args       []string
}

type Process struct {
	Logger  *slog.Logger
	configs []sinkExecConfig
}

func (p *Process) Start(ctx context.Context, config Config) error {
	templConfig := config.TemplateSpecs
	scs := makeSinkExecConfigs(templConfig)
	p.configs = scs
	return p.startTickLoops(ctx, renderAndExec)
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
				renderOnce:      cmp.Or(spec.RenderOnce || spec.RefreshInterval == 0),
				raw:             spec.Raw,
				missingKey:      strings.TrimSpace(spec.MissingKey),
			},
		}

		specExec := spec.Exec
		if specExec != nil {
			var ec execConfig
			ec.args = expandEnvs(specExec.CmdArgs)
			ec.cmd = specExec.Cmd
			ec.cmdTimeout = cmp.Or(time.Duration(specExec.CmdTimeout), defaultExecTimeout)
			scs[i].execConfig = &ec
		}
		i++
	}
	return scs
}

type templInitErr struct {
	name string
	err  error
}

func (t templInitErr) Error() string {
	return fmt.Sprintf("template init error for %s:%s", t.name, t.err.Error())
}

func (p *Process) startTickLoops(ctx context.Context, tf tickFunc) error {
	var wg sync.WaitGroup

	errsChan := make(chan error)

	for i := range p.configs {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sc := p.configs[idx]
			if err := initTemplate(&sc); err != nil {
				initErr := templInitErr{
					name: sc.name,
					err:  err,
				}
				errsChan <- initErr
				p.Logger.Error("init template error", slog.String("error", err.Error()), slog.String("name", sc.name))
				return
			}
			startRenderLoop(ctx, sc, tf, p.Logger)
		}(i)
	}

	go func() {
		wg.Wait()
		close(errsChan)
	}()

	var initErrs []error
	for err := range errsChan {
		initErrs = append(initErrs, err)
	}

	if len(initErrs) < 1 {
		return nil
	}

	if len(initErrs) == len(p.configs) {
		return fatal.NewError(errors.Join(initErrs...))
	}

	return errors.Join(initErrs...)
}

func initTemplate(sc *sinkExecConfig) error {
	at := actionable.NewTemplate(sc.name, sc.html)
	at.SetMissingKeyBehaviour(sc.missingKey)
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

	defer cfg.parsed.CloseActions()

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

	if cfg.execConfig == nil {
		return nil
	}

	cmdCtx, cancel := context.WithTimeout(ctx, cfg.cmdTimeout)
	defer cancel()
	if err := cmdexec.Do(cmdCtx, cfg.cmd, cfg.execConfig.args...); err != nil {
		return err
	}

	return nil

}

func expandEnvs(args []string) []string {
	for i := range args {
		args[i] = os.ExpandEnv(args[i])
	}
	return args
}
