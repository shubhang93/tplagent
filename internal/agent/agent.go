package agent

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"github.com/shubhang93/tplagent/internal/actionable"
	"github.com/shubhang93/tplagent/internal/cmdexec"
	"github.com/shubhang93/tplagent/internal/config"
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

type CMDExecer interface {
	ExecContext(ctx context.Context) error
}

type Renderer interface {
	Render(data any) error
}

type sinkExecConfig struct {
	sinkConfig
	*execConfig
}

var errTooManyFailures = errors.New("too many failures")

type tickFunc func(ctx context.Context, sink Renderer, execer CMDExecer, staticData any) error

type sinkConfig struct {
	parsed           *actionable.Template
	refreshInterval  time.Duration
	refreshOnTrigger bool
	html             bool
	templateDelims   []string
	actions          []config.Actions
	dest             string
	staticData       any
	name             string
	renderOnce       bool
	raw              string
	readFrom         string
	missingKey       string
}

type execConfig struct {
	cmd     string
	timeout time.Duration
	args    []string
	env     map[string]string
}

type triggerFlow struct {
	trigger     chan struct{}
	triggerResp chan error
}
type Proc struct {
	Logger   *slog.Logger
	TickFunc tickFunc
	configs  []sinkExecConfig
	Reloaded bool

	triggerMU       sync.Mutex
	refreshTriggers map[string]triggerFlow

	maxConsecFailures int
}

func (p *Proc) Start(ctx context.Context, config config.TPLAgent) error {

	if p.Reloaded {
		p.Logger.Info("agent reloading")
	} else {
		p.Logger.Info("agent starting")
	}

	if p.refreshTriggers == nil {
		p.refreshTriggers = make(map[string]triggerFlow, len(config.TemplateSpecs))
	}

	templConfig := config.TemplateSpecs
	scs := sanitizeConfigs(templConfig)
	p.configs = scs
	p.maxConsecFailures = cmp.Or(config.Agent.MaxConsecutiveFailures, defaultMaxConsecFailures)
	return p.startTickLoops(ctx)
}

func (p *Proc) TriggerRefresh(templateName string) error {

	p.triggerMU.Lock()
	flow, ok := p.refreshTriggers[templateName]
	p.triggerMU.Unlock()

	if !ok {
		return fmt.Errorf("render loop not initialized for template %s", templateName)
	}

	flow.trigger <- struct{}{}
	return <-flow.triggerResp

}

func sanitizeConfigs(templConfig map[string]*config.TemplateSpec) []sinkExecConfig {
	var i int
	var scs = make([]sinkExecConfig, len(templConfig))
	for name := range templConfig {
		specTempl := templConfig[name]
		scs[i] = sinkExecConfig{
			sinkConfig: sinkConfig{
				refreshInterval:  time.Duration(specTempl.RefreshInterval),
				html:             specTempl.HTML,
				templateDelims:   specTempl.TemplateDelimiters,
				actions:          specTempl.Actions,
				readFrom:         os.ExpandEnv(specTempl.Source),
				dest:             os.ExpandEnv(specTempl.Destination),
				staticData:       specTempl.StaticData,
				name:             name,
				renderOnce:       cmp.Or(specTempl.RenderOnce || specTempl.RefreshInterval == 0),
				raw:              specTempl.Raw,
				missingKey:       strings.TrimSpace(specTempl.MissingKey),
				refreshOnTrigger: specTempl.RefreshOnTrigger,
			},
		}

		specExec := specTempl.Exec
		if specExec != nil {
			scs[i].execConfig = &execConfig{
				cmd:     specExec.Cmd,
				timeout: cmp.Or(time.Duration(specExec.CmdTimeout), defaultExecTimeout),
				args:    specExec.CmdArgs,
				env:     specExec.Env,
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

func (p *Proc) startTickLoops(ctx context.Context) error {
	var wg sync.WaitGroup
	errsChan := make(chan error)

	for i := range p.configs {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sc := p.configs[idx]
			if err := p.initTemplate(&sc); err != nil {
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

func (p *Proc) initTemplate(sc *sinkExecConfig) error {
	at := actionable.NewTemplate(sc.name, sc.html)
	at.SetMissingKeyBehaviour(sc.missingKey)
	setTemplateDelims(at, sc.templateDelims)
	if err := attachActions(at, tplactions.Registry, p.Logger, sc.actions); err != nil {
		return err
	}
	sc.parsed = at
	return parseTemplate(sc.raw, sc.readFrom, sc.parsed)
}

func (p *Proc) startRenderLoop(ctx context.Context, cfg sinkExecConfig) error {

	p.Logger.Info("starting refresh loop", slog.String("templ", cfg.name))
	sink := render.Sink{
		Templ:   cfg.parsed,
		WriteTo: cfg.dest,
	}

	var execer CMDExecer = nil
	ec := cfg.execConfig
	if ec != nil {
		execer = &cmdexec.Default{
			Args:    ec.args,
			Cmd:     ec.cmd,
			Env:     ec.env,
			Timeout: ec.timeout,
		}
	}

	var ticker *time.Ticker
	var tick <-chan time.Time
	if cfg.renderOnce {
		if err := p.TickFunc(ctx, &sink, execer, cfg.staticData); err != nil && !errors.Is(err, render.ContentsIdentical) {
			p.Logger.Error("RenderAndExec error", slog.String("error", err.Error()), slog.String("loop", cfg.name), slog.Bool("once", true))
		}
		p.Logger.Info("refresh complete", slog.Bool("once", true), slog.String("templ", cfg.name))
	} else {
		ticker = time.NewTicker(cfg.refreshInterval)
		defer ticker.Stop()
		tick = ticker.C
	}

	refreshTrigger := make(chan struct{})
	triggerResp := make(chan error)

	p.triggerMU.Lock()
	p.refreshTriggers[cfg.name] = triggerFlow{
		trigger:     refreshTrigger,
		triggerResp: triggerResp,
	}
	p.triggerMU.Unlock()

	defer func() {
		cfg.parsed.CloseActions()
		p.triggerMU.Lock()
		delete(p.refreshTriggers, cfg.name)
		p.triggerMU.Unlock()
	}()

	consecutiveFailures := 0
	for consecutiveFailures < p.maxConsecFailures {
		resetFailures := true
		select {
		case <-ctx.Done():
			p.Logger.Info("stopping render sink", slog.String("sink", cfg.name), slog.String("cause", ctx.Err().Error()))
			return ctx.Err()
		case <-refreshTrigger:
			err := p.TickFunc(ctx, &sink, execer, cfg.staticData)
			triggerResp <- err
			resetFailures = p.handleTickExecErr(err, cfg)
		case <-tick:
			err := p.TickFunc(ctx, &sink, execer, cfg.staticData)
			resetFailures = p.handleTickExecErr(err, cfg)
		}
		if resetFailures {
			consecutiveFailures = 0
			continue
		}
		consecutiveFailures++
	}

	if consecutiveFailures == p.maxConsecFailures {
		fmt.Println("failures:", consecutiveFailures, p.maxConsecFailures)
		p.Logger.Error(
			"stopping refresh loop",
			slog.String("templ", cfg.name),
			slog.String("cause", "too many render failures"),
		)
		return fatal.NewError(errTooManyFailures)
	}
	return nil
}

func (p *Proc) handleTickExecErr(err error, cfg sinkExecConfig) (reset bool) {
	execErr := &cmdexec.ExecErr{}
	switch {
	case errors.Is(err, render.ContentsIdentical):
		return true
	case errors.As(err, &execErr):
		p.Logger.Error("render succeeded, exec failed",
			slog.String("error", string(execErr.Stderr)),
			slog.Int("exit-code", execErr.Status),
			slog.String("tmpl", cfg.name))
		return false
	case err != nil:
		p.Logger.Error("render failed", slog.String("cause", err.Error()))
		return false
	default:
		p.Logger.Info("refresh complete", slog.String("tmpl", cfg.name))
		return true
	}
}

type renderExecErr struct {
	execErr bool
	err     error
}

func (r renderExecErr) Unwrap() error {
	return r.err
}

func (r renderExecErr) Error() string {
	if r.execErr {
		return fmt.Sprintf("exec err:%s", r.err.Error())
	}
	return r.err.Error()
}

func RenderAndExec(ctx context.Context, sink Renderer, execer CMDExecer, staticData any) error {
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	err := sink.Render(staticData)
	if err != nil {
		return renderExecErr{
			execErr: false,
			err:     err,
		}
	}

	if execer == nil {
		return nil
	}

	return execer.ExecContext(context.Background())

}
