package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/shubhang93/tplagent/internal/agent"
	"io"
	"log/slog"
	"os"
	"time"
)

var configPath string

func init() {
	flag.StringVar(&configPath, "config", "/etc/tplagent/config.json", "-config=/path/to/config.json")
}

var configForGenerate = agent.Config{
	Agent: agent.AgentConfig{
		LogLevel: slog.LevelInfo,
		LogFmt:   "text",
	},
	TemplateSpecs: map[string]*agent.TemplateConfig{
		"myapp-config": {
			Actions:     []agent.ActionsConfig{},
			Source:      "/path/to/template-file",
			Destination: "/path/to/outfile",
			StaticData: map[string]string{
				"key": "value",
			},
			RefreshInterval: agent.Duration(1 * time.Second),
			RenderOnce:      false,
			MissingKey:      "error",
			Exec: &agent.ExecConfig{
				Cmd:        "echo",
				CmdArgs:    []string{"hello"},
				CmdTimeout: agent.Duration(30 * time.Second),
			},
		},
	},
}

func createCommandFuncs(stdout io.Writer, stderr io.Writer) map[string]func(ctx context.Context) {
	return map[string]func(context.Context){
		"start": func(ctx context.Context) {
			flag.Parse()
			startAgent(ctx, configPath)
		},
		"generate": func(_ context.Context) {
			jd := json.NewEncoder(stdout)
			jd.SetIndent("", " ")
			if err := jd.Encode(configForGenerate); err != nil {
				_, _ = fmt.Fprint(stderr, "error generating config")
				os.Exit(1)
			}
		},
	}
}

var usage = `usage: tplagent <start|generate> -config=/path/to/config`

func cli(ctx context.Context, stdout, stderr io.Writer, args ...string) {
	if len(args) < 1 {
		_, _ = fmt.Fprintln(stderr, usage)
		os.Exit(1)
	}
	cmd := args[0]
	cfs := createCommandFuncs(stdout, stderr)
	f, ok := cfs[cmd]
	if !ok {
		_, _ = fmt.Fprintln(stderr, usage)
		os.Exit(1)
	}
	f(ctx)
}
