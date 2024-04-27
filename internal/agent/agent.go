package agent

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"github.com/shubhang93/tplagent/internal/cmdexec"
	"github.com/shubhang93/tplagent/internal/render"
	"github.com/shubhang93/tplagent/internal/tplactions"
	"io"
	"log/slog"
	"os"
	"sync"
	"text/template"
	"time"
)

const stagingBuffSize = 4096
const defaultExecTimeout = 60 * time.Second

type sinkExecConfig struct {
	t               *template.Template
	refreshInterval time.Duration
	dest            string
	staticData      any
	name            string
	cmd             string
	cmdTimeout      time.Duration
	renderOnce      bool
}

func Run(ctx context.Context, config Config) error {
	logger := createLogger(config.Agent.LogFmt, config.Agent.LogLevel)
	logger.Info("starting agent")
	templConfig := config.TemplateSpecs
	scs, err := initTemplates(templConfig)
	if err != nil {
		return err
	}

	renderAndRefresh(ctx, scs, logger)
	return nil
}

func initTemplates(templConfig map[string]*TemplateConfig) ([]sinkExecConfig, error) {
	var i int
	var scs = make([]sinkExecConfig, len(templConfig))
	for name := range templConfig {
		spec := templConfig[name]
		stagingBuff := make([]byte, stagingBuffSize)
		text, err := templateText(spec.Raw, spec.Source, stagingBuff)
		if err != nil {
			return nil, fmt.Errorf("error reading template text for %s:%w", name, err)
		}
		templ, err := template.New(name).Parse(text)
		if err != nil {
			return nil, fmt.Errorf("templ parse error for %s:%w", name, err)
		}

		setTemplateDelims(templ, spec.TemplateDelimiters)
		if err := attachActions(templ, spec.Actions); err != nil {
			return nil, fmt.Errorf("error attaching actions for %s:%w", name, err)
		}
		scs[i] = sinkExecConfig{
			t:               templ,
			refreshInterval: time.Duration(spec.RefreshInterval),
			dest:            spec.Destination,
			staticData:      spec.StaticData,
			name:            name,
			cmd:             spec.ExecCMD,
			renderOnce:      spec.RenderOnce,
			cmdTimeout:      cmp.Or(time.Duration(spec.ExecTimeout), defaultExecTimeout),
		}
		i++
		clear(stagingBuff)
	}
	return scs, nil
}

func renderAndRefresh(ctx context.Context, scs []sinkExecConfig, l *slog.Logger) {
	var wg sync.WaitGroup

	for i := range scs {
		go func(idx int) {
			defer wg.Done()
			sc := scs[idx]
			startRenderLoop(ctx, sc, renderAndExec, l)
		}(i)
	}
	wg.Wait()
}

func startRenderLoop(ctx context.Context, cfg sinkExecConfig, onTick func(context.Context, sinkExecConfig, render.Sink) error, logger *slog.Logger) {
	ticker := time.NewTicker(cfg.refreshInterval)
	tick := ticker.C

	defer ticker.Stop()

	sink := render.Sink{
		Templ:   cfg.t,
		WriteTo: os.ExpandEnv(cfg.dest),
	}

	if cfg.renderOnce {
		if err := onTick(ctx, cfg, sink); err != nil {
			logger.Error("renderAndExec error", slog.String("error", err.Error()), slog.String("loop", cfg.name), slog.Bool("once", true))
		}
		return
	}

	for {
		select {
		case <-ctx.Done():
			logger.Info("stopping render sink", slog.String("sink", cfg.name), slog.String("cause", ctx.Err().Error()))
			return
		case <-tick:
			err := onTick(ctx, cfg, sink)
			if err != nil {
				logger.Error("renderAndExec error", slog.String("error", err.Error()), slog.String("loop", cfg.name))
			}
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

func templateText(raw string, path string, readBuff []byte) (string, error) {
	expandedPath := os.ExpandEnv(path)
	if raw != "" {
		return raw, nil
	}
	file, err := os.Open(expandedPath)
	if err != nil {
		return "", err
	}
	var buff bytes.Buffer
	_, err = io.CopyBuffer(&buff, file, readBuff)
	if err != nil {
		return "", err
	}
	return buff.String(), nil
}

func attachActions(t *template.Template, templActions []ActionConfig) error {
	namesSpacedFuncMap := make(template.FuncMap)
	for _, ta := range templActions {
		actionMaker, ok := tplactions.Registry[ta.Name]
		if !ok {
			return fmt.Errorf("invalid action name:%s", ta.Name)
		}
		action := actionMaker()
		if err := action.SetConfig(ta.Config); err != nil {
			return fmt.Errorf("error setting config for %s", ta.Name)
		}
		fm := action.FuncMap()
		for name, f := range fm {
			funcNameWithNS := []byte(ta.Name)
			funcNameWithNS = append(funcNameWithNS, '_')
			funcNameWithNS = append(funcNameWithNS, name...)
			namesSpacedFuncMap[string(funcNameWithNS)] = f
		}

	}
	// template.Funcs validates
	// function names in the FuncMap
	// we cannot use special chars
	// except underscores
	// we prefix the action name
	// to each of the function names
	// ex: api_getJSON
	t.Funcs(namesSpacedFuncMap)
	return nil
}

func setTemplateDelims(t *template.Template, delims []string) {
	if len(delims) < 2 {
		return
	}
	left, right := delims[0], delims[1]
	t.Delims(left, right)
}
