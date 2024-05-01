package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/shubhang93/tplagent/internal/agent"
	"log/slog"
	"os"
	"testing"
	"time"
)

func Test_cli(t *testing.T) {
	tmpDir := t.TempDir()
	t.Run("test generate", func(t *testing.T) {
		stdout := bytes.Buffer{}
		expected := bytes.Buffer{}
		runCLI(context.Background(), &stdout, os.Stderr, "generate")

		jd := json.NewEncoder(&expected)
		jd.SetIndent("", " ")
		if err := jd.Encode(configForGenerate); err != nil {
			t.Errorf("error encoding:%v", err)
			return
		}

		if diff := cmp.Diff(expected.String(), stdout.String()); diff != "" {
			t.Errorf("(--Want ++Got):\n%s", diff)
		}

		t.Log(stdout.String())
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
				RefreshInterval: agent.Duration(1 * time.Second),
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
		runCLI(ctx, os.Stdout, os.Stderr, "start")

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
