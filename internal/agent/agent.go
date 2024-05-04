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

type tickFunc func(ctx context.Context, sink render.Sink, execer cmdExecer, staticData any) error

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
	cmd     string
	timeout time.Duration
	args    []string
	env     map[string]string
}

type Process struct {
	Logger            *slog.Logger
	TickFunc          tickFunc
	configs           []sinkExecConfig
	maxConsecFailures int
}

func (p *Process) Start(ctx context.Context, config Config) error {
	templConfig := config.TemplateSpecs
	scs := makeSinkExecConfigs(templConfig)
	p.configs = scs
	p.maxConsecFailures = cmp.Or(config.Agent.MaxConsecutiveFailures, defaultMaxConsecFailures)
	return p.startTickLoops(ctx)
}

func makeSinkExecConfigs(templConfig map[string]*TemplateConfig) []sinkExecConfig {
	var i int
	var scs = make([]sinkExecConfig, len(templConfig))
	for name := range templConfig {
		specTempl := templConfig[name]
		scs[i] = sinkExecConfig{
			sinkConfig: sinkConfig{
				refreshInterval: time.Duration(specTempl.RefreshInterval),
				html:            specTempl.HTML,
				templateDelims:  specTempl.TemplateDelimiters,
				actions:         specTempl.Actions,
				readFrom:        os.ExpandEnv(specTempl.Source),
				dest:            os.ExpandEnv(specTempl.Destination),
				staticData:      specTempl.StaticData,
				name:            name,
				renderOnce:      cmp.Or(specTempl.RenderOnce || specTempl.RefreshInterval == 0),
				raw:             specTempl.Raw,
				missingKey:      strings.TrimSpace(specTempl.MissingKey),
			},
		}

		specExec := specTempl.Exec
		if specExec != nil {
			scs[i].execConfig = &execConfig{
				cmd:     specExec.Cmd,
				timeout: cmp.Or(time.Duration(specExec.CmdTimeout), defaultExecTimeout),
				args:    expandEnvs(specExec.CmdArgs),
				env:     nil,
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

func (p *Process) startTickLoops(ctx context.Context) error {
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
			errsChan <- p.startRenderLoop(ctx, sc)
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

func (p *Process) startRenderLoop(ctx context.Context, cfg sinkExecConfig) error {
	ticker := time.NewTicker(cfg.refreshInterval)
	tick := ticker.C
	defer ticker.Stop()

	p.Logger.Info("starting refresh loop", slog.String("templ", cfg.name))
	sink := render.Sink{
		Templ:   cfg.parsed,
		WriteTo: cfg.dest,
	}

	var execer cmdExecer = nil
	ec := cfg.execConfig
	if ec != nil {
		execer = &cmdexec.Default{
			Args:    ec.args,
			Cmd:     ec.cmd,
			Env:     ec.env,
			Timeout: ec.timeout,
		}
	}

	defer cfg.parsed.CloseActions()

	if cfg.renderOnce {
		if err := p.TickFunc(ctx, sink, execer, cfg.staticData); err != nil && !errors.Is(err, render.ContentsIdentical) {
			p.Logger.Error("RenderAndExec error", slog.String("error", err.Error()), slog.String("loop", cfg.name), slog.Bool("once", true))
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
			err := p.TickFunc(ctx, sink, execer, cfg.staticData)
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

func RenderAndExec(ctx context.Context, sink render.Sink, execer cmdExecer, staticData any) error {

	select {
	case <-ctx.Done():
		return nil
	default:
	}

	err := sink.Render(staticData)
	if err != nil {
		return err
	}

	if execer == nil {
		return nil
	}

	errCh := make(chan error, 1)
	go func() {
		// use a new background context
		// we want the cmd
		// to exec within the timeout
		// and not cancel on the
		// main context
		if err := execer.ExecContext(context.Background()); err != nil {
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
