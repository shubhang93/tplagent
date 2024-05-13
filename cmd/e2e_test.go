package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/shubhang93/tplagent/internal/config"
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
	cfg := config.TPLAgent{
		Agent: config.Agent{
			LogLevel:               slog.LevelInfo,
			LogFmt:                 "json",
			MaxConsecutiveFailures: 10,
		},
		TemplateSpecs: map[string]*config.TemplateSpec{
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
				Actions: []config.Actions{{
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
				Exec: &config.ExecSpec{
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

func Test_With_HTTPLis(t *testing.T) {
	tmp := t.TempDir()
	configPath := tmp + "/config.json"

	f, err := os.Create(configPath)
	if err != nil {
		t.Error(err)
		return
	}

	cfg := config.TPLAgent{
		Agent: config.Agent{
			LogLevel:               slog.LevelInfo,
			LogFmt:                 "json",
			MaxConsecutiveFailures: 10,
			HTTPListenerAddr:       "localhost:6000",
		},
		TemplateSpecs: map[string]*config.TemplateSpec{
			"appconf": {
				Raw:         "hello {{.name}}",
				Destination: tmp + "/appconf.txt",
				StaticData:  map[string]any{"name": "foo"},
				RenderOnce:  true,
			},
		},
	}

	if err := json.NewEncoder(f).Encode(cfg); err != nil {
		t.Error(err)
		return
	}
	_ = f.Close()

	cliErr := make(chan error)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		cliErr <- startCLI(ctx, os.Stdout, []string{"start", "-config", configPath}...)

	}()

	cfg.TemplateSpecs["server-conf"] = &config.TemplateSpec{
		Raw:             "PORT: {{.Port}}",
		Destination:     tmp + "/server.conf",
		StaticData:      map[string]any{"Port": "9090"},
		RefreshInterval: duration.Duration(2 * time.Second),
	}

	reloadReq := map[string]any{
		"config":      cfg,
		"config_path": configPath,
	}

	var buff bytes.Buffer
	err = json.NewEncoder(&buff).Encode(reloadReq)
	if err != nil {
		t.Error(err)
	}

	time.Sleep(100 * time.Millisecond)
	resp, err := http.Post("http://localhost:6000/config/reload", "application/json", &buff)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Error(err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status code %d got %d", http.StatusOK, resp.StatusCode)
	}

	if err := <-cliErr; err != nil {
		t.Errorf("CLI err %v", err)
	}

	bs, err := os.ReadFile(tmp + "/server.conf")
	if err != nil {
		t.Error(err)
	}

	serverConf := string(bs)
	const expected = "PORT: 9090"
	if serverConf != expected {
		t.Errorf("expected %s got %s", expected, serverConf)
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
