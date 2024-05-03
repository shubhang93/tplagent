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
	"github.com/shubhang93/tplagent/internal/tplactions"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

const defaultExecTimeout = 30 * time.Second
const defaultMaxConsecFailures = 10

type cmdExecer interface {
	ExecContext(ctx context.Context) error
}

type sinkExecConfig struct {
	sinkConfig
	*execConfig
}

var errTooManyFailures = errors.New("too many failures")

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
	cmdTimeout time.Duration
	command    cmdExecer
}

type Process struct {
	Logger            *slog.Logger
	configs           []sinkExecConfig
	maxConsecFailures int
}

func (p *Process) Start(ctx context.Context, config Config) error {
	templConfig := config.TemplateSpecs
	scs := makeSinkExecConfigs(templConfig)
	p.configs = scs
	p.maxConsecFailures = cmp.Or(config.Agent.MaxConsecutiveFailures, defaultMaxConsecFailures)
	return p.startTickLoops(ctx, p.renderAndExec)
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
			scs[i].execConfig = &execConfig{
				cmdTimeout: cmp.Or(time.Duration(specExec.CmdTimeout), defaultExecTimeout),
				command: &cmdexec.Default{
					Args: expandEnvs(specExec.CmdArgs),
					Cmd:  specExec.Cmd,
					Env:  nil,
				},
			}
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
				errsChan <- fatal.NewError(initErr)
				p.Logger.Error("init template error", slog.String("error", err.Error()), slog.String("name", sc.name))
				return
			}
			errsChan <- p.startRenderLoop(ctx, sc, tf)
		}(i)
	}

	go func() {
		wg.Wait()
		close(errsChan)
	}()

	var loopErrs []error
	var fatalCount int
	for err := range errsChan {
		if fatal.Is(err) {
			fatalCount++
		}
		loopErrs = append(loopErrs, err)
	}

	if len(loopErrs) < 1 {
		return nil
	}

	if fatalCount == len(p.configs) {
		return fatal.NewError(errors.Join(loopErrs...))
	}

	return errors.Join(loopErrs...)
}

func initTemplate(sc *sinkExecConfig) error {
	at := actionable.NewTemplate(sc.name, sc.html)
	at.SetMissingKeyBehaviour(sc.missingKey)
	setTemplateDelims(at, sc.templateDelims)
	if err := attachActions(at, tplactions.Registry, sc.actions); err != nil {
		return err
	}
	sc.parsed = at
	return parseTemplate(sc.raw, sc.readFrom, sc.parsed)
}

func (p *Process) startRenderLoop(ctx context.Context, cfg sinkExecConfig, onTick func(context.Context, sinkExecConfig, render.Sink) error) error {
	ticker := time.NewTicker(cfg.refreshInterval)
	tick := ticker.C
	defer ticker.Stop()

	p.Logger.Info("starting refresh loop", slog.String("templ", cfg.name))
	sink := render.Sink{
		Templ:   cfg.parsed,
		WriteTo: cfg.dest,
	}

	defer cfg.parsed.CloseActions()

	if cfg.renderOnce {
		if err := onTick(ctx, cfg, sink); err != nil && !errors.Is(err, render.ContentsIdentical) {
			p.Logger.Error("renderAndExec error", slog.String("error", err.Error()), slog.String("loop", cfg.name), slog.Bool("once", true))
		}
		return nil
	}

	consecutiveFailures := 0
	for consecutiveFailures < p.maxConsecFailures {
		select {
		case <-ctx.Done():
			p.Logger.Info("stopping render sink", slog.String("sink", cfg.name), slog.String("cause", ctx.Err().Error()))
			return ctx.Err()
		case <-tick:
			err := onTick(ctx, cfg, sink)
			switch {
			case errors.Is(err, render.ContentsIdentical):
				consecutiveFailures = 0
			case err != nil:
				p.Logger.Error("refresh failed", slog.String("cause", err.Error()))
				consecutiveFailures++
			default:
				p.Logger.Info("refresh complete", slog.String("tmpl", cfg.name))
				consecutiveFailures = 0
			}
		}
	}
	if consecutiveFailures == p.maxConsecFailures {
		p.Logger.Error(
			"stopping refresh loop",
			slog.String("templ", cfg.name),
			slog.String("cause", "too many render failures"),
		)
		return fatal.NewError(errTooManyFailures)
	}
	return nil
}

func (p *Process) renderAndExec(_ context.Context, cfg sinkExecConfig, sink render.Sink) error {
	err := sink.Render(cfg.staticData)

	if err != nil {
		return err
	}

	if cfg.execConfig == nil {
		return nil
	}

	commandExecutor := cfg.command
	errCh := make(chan error, 1)
	go func() {
		// use a new context
		// we want the cmd
		// to exec within the timeout
		// and not cancel on the
		// main context
		cmdCtx, cancel := context.WithTimeout(context.Background(), cfg.cmdTimeout)
		defer cancel()
		if err := commandExecutor.ExecContext(cmdCtx); err != nil {
			errCh <- err
			return
		}
		close(errCh)
		return
	}()

	return <-errCh

}

func expandEnvs(args []string) []string {
	for i := range args {
		args[i] = os.ExpandEnv(args[i])
	}
	return args
}
