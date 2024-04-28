package agent

import (
	"cmp"
	"context"
	"fmt"
	gocmp "github.com/google/go-cmp/cmp"
	"github.com/shubhang93/tplagent/internal/actionable"
	"github.com/shubhang93/tplagent/internal/render"
	"log/slog"
	"os"
	"slices"
	"testing"
	"text/template"
	"time"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

type mockAction struct{}

func (m mockAction) FuncMap() template.FuncMap {
	return make(template.FuncMap)
}

func (m mockAction) SetConfig(bytes []byte) error {
	return nil
}

func Test_makeSinkExecConfigs(t *testing.T) {
	t.Run("paths containing env vars should get expanded", func(t *testing.T) {
		config := map[string]*TemplateConfig{
			"testconfig": {
				Source:          "$HOME/testdir",
				Destination:     "${HOME}/testdir2",
				HTML:            false,
				StaticData:      map[string]any{},
				RefreshInterval: Duration(1 * time.Second),
				RenderOnce:      true,
				ExecCMD:         "echo hello",
				ExecTimeout:     Duration(5 * time.Second),
			},
			"testconfig2": {
				Actions: []ActionConfig{{
					Name:   "httpJson",
					Config: []byte(`{"key":"value"}`),
				}},
				TemplateDelimiters: []string{"<<", ">>"},
				HTML:               true,
				StaticData:         map[string]any{},
				RenderOnce:         true,
				ExecCMD:            "echo hello",
			},
		}

		homeDir := os.Getenv("HOME")
		expectedConfigs := []sinkExecConfig{
			{
				sinkConfig: sinkConfig{
					refreshInterval: 1 * time.Second,
					dest:            homeDir + "/testdir2",
					staticData:      map[string]any{},
					name:            "testconfig",
					renderOnce:      true,
					readFrom:        homeDir + "/testdir",
				},
				execConfig: execConfig{
					cmd:        "echo hello",
					cmdTimeout: 5 * time.Second,
				},
			},
			{
				sinkConfig: sinkConfig{
					html:           true,
					templateDelims: []string{"<<", ">>"},
					actions: []ActionConfig{{
						Name:   "httpJson",
						Config: []byte(`{"key":"value"}`),
					}},
					staticData: map[string]any{},
					name:       "testconfig2",
					renderOnce: true,
				},
				execConfig: execConfig{
					cmd:        "echo hello",
					cmdTimeout: 60 * time.Second,
				},
			}}

		seConfigs := makeSinkExecConfigs(config)
		slices.SortFunc(seConfigs, func(a, b sinkExecConfig) int {
			return cmp.Compare(a.name, b.name)
		})

		if diff := gocmp.Diff(
			expectedConfigs,
			seConfigs,
			gocmp.AllowUnexported(sinkExecConfig{}, execConfig{}, sinkConfig{}),
		); diff != "" {
			t.Errorf("(--Want ++Got)%s\n", diff)
		}
	})

}

func Test_renderLoop(t *testing.T) {

	tmpDir := t.TempDir()
	renderPath := fmt.Sprintf("%s/%s", tmpDir, "test.render")
	tpl := actionable.NewTemplate("test", false)
	must(tpl.Parse("Name {{.name}}"))

	type loopTest struct {
		name      string
		wantCount int
		cfg       sinkExecConfig
	}

	ltests := []loopTest{{
		name:      "render once is false",
		wantCount: 10,
		cfg: sinkExecConfig{
			sinkConfig: sinkConfig{
				parsed:          tpl,
				refreshInterval: 500 * time.Millisecond,
				dest:            renderPath,
				staticData:      map[string]any{"name": "foo"},
				name:            "test-tmpl",
			},
			execConfig: execConfig{
				cmd:        `echo "rendered"`,
				cmdTimeout: 30 * time.Second,
			},
		},
	}, {
		name:      "render once is true",
		wantCount: 1,
		cfg: sinkExecConfig{
			sinkConfig: sinkConfig{
				renderOnce:      true,
				parsed:          tpl,
				refreshInterval: 500 * time.Millisecond,
				dest:            renderPath,
				staticData:      map[string]any{"name": "foo"},
				name:            "test-tmpl",
			},
			execConfig: execConfig{
				cmd:        `echo "rendered"`,
				cmdTimeout: 30 * time.Second,
			},
		},
	}}

	for _, ltest := range ltests {
		t.Run(ltest.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
			defer cancel()

			logger := newLogger()

			execCount := 0
			onTick := func(ctx context.Context, config sinkExecConfig, sink render.Sink) error {
				execCount++
				return nil
			}

			startRenderLoop(ctx, ltest.cfg, onTick, logger)
			if ltest.wantCount != execCount {
				t.Errorf("expected count %d got %d", ltest.wantCount, execCount)
			}
		})
	}

	t.Run("init template", func(t *testing.T) {
		err := initTemplate(&sinkExecConfig{
			sinkConfig: sinkConfig{
				readFrom: "/some-non-existent/path.tmpl",
			},
		})
		if err == nil {
			t.Errorf("expected error to be not nil")
			return
		}
		t.Log(err)
	})

	t.Run("all templates in renderAndRefresh fail to initialize", func(t *testing.T) {
		scs := []sinkExecConfig{
			{
				sinkConfig: sinkConfig{
					name: "malofmedTmpl",
					raw:  "{{.",
				},
			}, {
				sinkConfig: sinkConfig{
					name:     "nonExistentPath",
					readFrom: "/some/nonexistent-path/path.tpl",
				},
			}, {
				sinkConfig: sinkConfig{
					name: "nonExistentAction",
					actions: []ActionConfig{{
						Name:   "fooaction",
						Config: nil,
					}},
				},
			}}

		tf := tickFunc(func(ctx context.Context, config sinkExecConfig, sink render.Sink) error {
			return nil
		})
		err := renderAndRefresh(context.Background(), scs, tf, newLogger())
		if err == nil {
			t.Error("expected error but got nil")
			return
		}
		t.Log(err)
	})

	t.Run("test render and refresh for a valid config", func(t *testing.T) {

	})
}

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}
