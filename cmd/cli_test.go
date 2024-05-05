package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/shubhang93/tplagent/internal/agent"
	"github.com/shubhang93/tplagent/internal/duration"
	"github.com/shubhang93/tplagent/internal/fatal"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func Test_cli(t *testing.T) {
	tmpDir := t.TempDir()
	t.Run("test generate", func(t *testing.T) {
		stdout := bytes.Buffer{}
		expected := bytes.Buffer{}
		cliArgs := []string{"genconf", "-n", "2", "-indent", "4"}
		err := startCLI(context.Background(), &stdout, cliArgs...)
		if err != nil {
			t.Error(err)
			return
		}

		expectedIndent := 4
		jd := json.NewEncoder(&expected)
		jd.SetIndent("", strings.Repeat(" ", expectedIndent))

		starter := agent.Config{
			Agent: agent.AgentConfig{
				LogLevel:               slog.LevelInfo,
				LogFmt:                 "text",
				MaxConsecutiveFailures: 10,
			},
			TemplateSpecs: map[string]*agent.TemplateConfig{
				"myapp-config1": {
					Actions:     []agent.ActionsConfig{},
					Source:      "/path/to/template-file1",
					Destination: "/path/to/outfile1",
					StaticData: map[string]string{
						"key": "value",
					},
					RefreshInterval: duration.Duration(1 * time.Second),
					RenderOnce:      false,
					MissingKey:      "error",
					Exec: &agent.ExecConfig{
						Cmd:        "echo",
						CmdArgs:    []string{"hello"},
						CmdTimeout: duration.Duration(30 * time.Second),
					},
				},
				"myapp-config2": {
					Actions:     []agent.ActionsConfig{},
					Source:      "/path/to/template-file2",
					Destination: "/path/to/outfile2",
					StaticData: map[string]string{
						"key": "value",
					},
					RefreshInterval: duration.Duration(1 * time.Second),
					RenderOnce:      false,
					MissingKey:      "error",
					Exec: &agent.ExecConfig{
						Cmd:        "echo",
						CmdArgs:    []string{"hello"},
						CmdTimeout: duration.Duration(30 * time.Second),
					},
				},
			}}

		if err := jd.Encode(starter); err != nil {
			t.Errorf("error encoding:%v", err)
			return
		}

		if diff := cmp.Diff(expected.String(), stdout.String()); diff != "" {
			t.Errorf("(--Want ++Got):\n%s", diff)
		}

	})

	t.Run("test generate when num block is less than 1 and indent is less than 1", func(t *testing.T) {
		stdout := bytes.Buffer{}
		expected := bytes.Buffer{}
		cliArgs := []string{"genconf", "-n", "0", "-indent", "0"}
		err := startCLI(context.Background(), &stdout, cliArgs...)
		if err != nil {
			t.Error(err)
			return
		}

		defaultIndent := 2
		jd := json.NewEncoder(&expected)
		jd.SetIndent("", strings.Repeat(" ", defaultIndent))

		starter := agent.Config{
			Agent: agent.AgentConfig{
				LogLevel:               slog.LevelInfo,
				LogFmt:                 "text",
				MaxConsecutiveFailures: 10,
			},
			TemplateSpecs: map[string]*agent.TemplateConfig{
				"myapp-config1": {
					Actions:     []agent.ActionsConfig{},
					Source:      "/path/to/template-file1",
					Destination: "/path/to/outfile1",
					StaticData: map[string]string{
						"key": "value",
					},
					RefreshInterval: duration.Duration(1 * time.Second),
					RenderOnce:      false,
					MissingKey:      "error",
					Exec: &agent.ExecConfig{
						Cmd:        "echo",
						CmdArgs:    []string{"hello"},
						CmdTimeout: duration.Duration(30 * time.Second),
					},
				},
			}}

		if err := jd.Encode(starter); err != nil {
			t.Errorf("error encoding:%v", err)
			return
		}

		if diff := cmp.Diff(expected.String(), stdout.String()); diff != "" {
			t.Errorf("(--Want ++Got):\n%s", diff)
		}

	})

	t.Run("start agent test", func(t *testing.T) {
		configFilePath := tmpDir + "/config.json"
		dest := tmpDir + "/config.render"
		ac := agent.Config{Agent: agent.AgentConfig{
			LogLevel: slog.LevelInfo,
			LogFmt:   "text",
		}, TemplateSpecs: map[string]*agent.TemplateConfig{
			"test-config": {
				Actions: []agent.ActionsConfig{{
					Name:   "sample",
					Config: json.RawMessage(`{"greet_message":"Hello"}`),
				}},
				Raw: `Sample Render:
Sample Action:{{ sample_greet .name -}}`,
				Destination: dest,
				HTML:        false,
				StaticData: map[string]string{
					"name": "Foo",
				},
				Exec: &agent.ExecConfig{
					Cmd: "bash",
					CmdArgs: []string{
						"-c",
						fmt.Sprintf(`echo "%swritten from exec"  >> %s`, "\n", dest),
					},
				},
				RefreshInterval: duration.Duration(1 * time.Second),
				RenderOnce:      false,
				MissingKey:      "error",
			},
		}}

		cf, err := os.OpenFile(configFilePath, os.O_CREATE|os.O_RDWR, 0755)
		defer cf.Close()
		if err != nil {
			t.Errorf("error creating file:%v", err)
			return
		}
		jd := json.NewEncoder(cf)
		jd.SetIndent("", " ")
		err = jd.Encode(ac)
		if err != nil {
			t.Errorf("%v", err)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
		defer cancel()

		_ = flag.Set("config", configFilePath)
		err = startCLI(ctx, os.Stdout, "start", "-config", configFilePath)
		if fatal.Is(err) {
			t.Errorf("CLI error:%v", err)
			return
		}

		d, err := os.ReadFile(dest)
		if err != nil {
			t.Error(err)
			return
		}

		expectedFileContents := `Sample Render:
Sample Action:Hello Foo
written from exec
`
		if diff := cmp.Diff(expectedFileContents, string(d)); diff != "" {
			t.Error(diff)
		}

	})
}
