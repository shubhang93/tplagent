package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/shubhang93/tplagent/internal/agent"
	"github.com/shubhang93/tplagent/internal/duration"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestE2E(t *testing.T) {
	tmp := t.TempDir()

	serverConfTemplate := tmp + "/server-conf.tmpl"
	err := os.WriteFile(serverConfTemplate, []byte(`{"port":"9090","log_level":"INFO"}`), 0755)
	if err != nil {
		t.Error(err)
		return
	}

	cfg := agent.Config{
		Agent: agent.AgentConfig{
			LogLevel:               slog.LevelInfo,
			LogFmt:                 "json",
			MaxConsecutiveFailures: 10,
		},
		TemplateSpecs: map[string]*agent.TemplateConfig{
			"app-conf": {
				Actions: []agent.ActionsConfig{{
					Name:   "httpjson",
					Config: json.RawMessage(`{"base_url":"http://localhost:5000"}`),
				}},
				Raw:             `{"id":"{{.ID}}"}`,
				Destination:     tmp + "/app-conf.json",
				RefreshInterval: duration.Duration(1 * time.Second),
				MissingKey:      "error",
			},
			"server-conf": {
				Actions: []agent.ActionsConfig{{
					Name:   "httpjson",
					Config: json.RawMessage(`{"base_url":"http://localhost:5000"}`),
				}},
				Source:          serverConfTemplate,
				Destination:     tmp + "/server-conf.json",
				RefreshInterval: duration.Duration(1 * time.Second),
				Exec: &agent.ExecConfig{
					Cmd: "bash",
					CmdArgs: []string{
						"-c",
						fmt.Sprintf(`%s "%s" > %s`, "echo", "OUTFILE", "out.txt"),
					},
					CmdTimeout: duration.Duration(30 * time.Second),
					Env: map[string]string{
						"CONF":    tmp + "/server-conf.json",
						"OUTFILE": "out.txt",
					},
				},
			},
		},
	}
	configFileLoc := tmp + "/agent-config.json"
	cfgFile, err := os.OpenFile(configFileLoc, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		t.Error(err)
		return
	}

	err = json.NewEncoder(cfgFile).Encode(&cfg)
	_ = cfgFile.Close()
	if err != nil {
		t.Error(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = startCLI(ctx, os.Stdout, []string{"start", "-config", configFileLoc}...)
	if err != nil {
		t.Error(err)
		return
	}

}
