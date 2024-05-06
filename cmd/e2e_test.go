package main

import (
	"context"
	"encoding/json"
	"github.com/shubhang93/tplagent/internal/agent"
	"github.com/shubhang93/tplagent/internal/duration"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestE2E(t *testing.T) {
	tmp := t.TempDir()

	serverConfTemplate := tmp + "/server-conf.tmpl"
	err := os.WriteFile(serverConfTemplate, []byte(`{{with httpjson_GET_Map "/server-conf" -}}
{
  "port":{{.Port}},
  "log_level":"{{.LogLevel}}"
}
{{end}}`), 0755)
	if err != nil {
		t.Error(err)
		return
	}

	serverConfDest := tmp + "/server-conf.json"
	appConfDest := tmp + "/app-conf.json"
	cfg := agent.Config{
		Agent: agent.AgentConfig{
			LogLevel:               slog.LevelInfo,
			LogFmt:                 "json",
			MaxConsecutiveFailures: 10,
		},
		TemplateSpecs: map[string]*agent.TemplateConfig{
			"app-conf": {
				Raw:         `{"id":"{{.ID}}"}`,
				Destination: appConfDest,
				StaticData: map[string]any{
					"ID": "foo-bar",
				},
				RefreshInterval: duration.Duration(1 * time.Second),
				MissingKey:      "error",
			},
			"server-conf": {
				Actions: []agent.ActionsConfig{{
					Name:   "httpjson",
					Config: json.RawMessage(`{"base_url":"http://localhost:6000"}`),
				}},
				Source:      serverConfTemplate,
				Destination: serverConfDest,
				StaticData: map[string]any{
					"Port":     9090,
					"LogLevel": "ERROR",
				},
				RefreshInterval: duration.Duration(1 * time.Second),
				Exec: &agent.ExecConfig{
					Cmd: "bash",
					CmdArgs: []string{
						"-c",
						`cat "$CONF" > "$OUTFILE"`,
					},
					CmdTimeout: duration.Duration(30 * time.Second),
					Env: map[string]string{
						"CONF":    serverConfDest,
						"OUTFILE": tmp + "/out.json",
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

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		startGracefulHTTPServer(ctx)
		close(done)
	}()

	err = startCLI(ctx, os.Stdout, []string{"start", "-config", configFileLoc}...)
	if err != nil && !isCtxErr(err) {
		t.Error(err)
		return
	}

	<-done

	expectedServerConf := `{
  "port":5005,
  "log_level":"ERROR"
}
`
	gotServerConf, err := os.ReadFile(tmp + "/out.json")
	if err != nil {
		t.Error(err)
		return
	}
	if expectedServerConf != string(gotServerConf) {
		t.Errorf("-(%q) +(%q)", expectedServerConf, string(gotServerConf))
		return
	}

	expectedAppConf := `{"id":"foo-bar"}`
	gotAppConf, err := os.ReadFile(appConfDest)
	if err != nil {
		t.Errorf("-(%q) +(%q)", expectedAppConf, string(gotAppConf))
	}
}

func startGracefulHTTPServer(ctx context.Context) {
	mux := http.NewServeMux()

	portMaker := func() http.HandlerFunc {
		count := 0
		data := map[string]any{}
		port := 5000
		return func(writer http.ResponseWriter, request *http.Request) {
			if count == 5 {
				_ = json.NewEncoder(writer).Encode(data)
				return
			}
			count++
			data = map[string]any{
				"Port":     port + count,
				"LogLevel": "ERROR",
			}
			_ = json.NewEncoder(writer).Encode(data)
			return
		}
	}

	mux.HandleFunc("/server-conf", portMaker())
	s := http.Server{
		Addr:    "localhost:6000",
		Handler: mux,
	}

	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_ = s.Shutdown(shutdownCtx)
		close(done)
	}()

	_ = s.ListenAndServe()
	<-done

}
