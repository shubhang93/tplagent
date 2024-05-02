package agent

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	gocmp "github.com/google/go-cmp/cmp"
	"github.com/shubhang93/tplagent/internal/actionable"
	"github.com/shubhang93/tplagent/internal/fatal"
	"github.com/shubhang93/tplagent/internal/render"
	"log/slog"
	"os"
	"slices"
	"sync"
	"testing"
	"time"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
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
				Exec: &ExecConfig{
					Cmd:        "echo",
					CmdArgs:    []string{"hello"},
					CmdTimeout: Duration(5 * time.Second),
				},
			},
			"testconfig2": {
				Actions: []ActionsConfig{{
					Name:   "httpJson",
					Config: []byte(`{"key":"value"}`),
				}},
				TemplateDelimiters: []string{"<<", ">>"},
				HTML:               true,
				StaticData:         map[string]any{},
				RenderOnce:         true,
				Exec: &ExecConfig{
					Cmd:     "echo",
					CmdArgs: []string{"hello"},
				},
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
				execConfig: &execConfig{
					cmd:        "echo",
					args:       []string{"hello"},
					cmdTimeout: 5 * time.Second,
				},
			},
			{
				sinkConfig: sinkConfig{
					html:           true,
					templateDelims: []string{"<<", ">>"},
					actions: []ActionsConfig{{
						Name:   "httpJson",
						Config: []byte(`{"key":"value"}`),
					}},
					staticData: map[string]any{},
					name:       "testconfig2",
					renderOnce: true,
				},
				execConfig: &execConfig{
					cmd:        "echo",
					args:       []string{"hello"},
					cmdTimeout: 30 * time.Second,
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
			execConfig: &execConfig{
				cmd:        `echo`,
				args:       []string{"hello"},
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
			execConfig: &execConfig{
				cmd:        `echo`,
				args:       []string{"hello"},
				cmdTimeout: 30 * time.Second,
			},
		},
	}}

	for _, ltest := range ltests {
		t.Run(ltest.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
			defer cancel()

			execCount := 0
			onTick := func(ctx context.Context, config sinkExecConfig, sink render.Sink) error {
				execCount++
				return nil
			}

			p := Process{Logger: newLogger(), maxConsecFailures: 10}
			err := p.startRenderLoop(ctx, ltest.cfg, onTick)
			if err != nil && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("%v", err)
			}
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

	t.Run("all templates in startTickLoops fail to initialize", func(t *testing.T) {
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
					actions: []ActionsConfig{{
						Name:   "fooaction",
						Config: nil,
					}},
				},
			}}

		p := Process{
			Logger:  newLogger(),
			configs: scs,
		}

		tf := tickFunc(func(ctx context.Context, config sinkExecConfig, sink render.Sink) error {
			return nil
		})
		err := p.startTickLoops(context.Background(), tf)
		if err == nil {
			t.Error("expected error but got nil")
			return
		}
		t.Log(err)
	})

	t.Run("test render and refresh for a valid config", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
		defer cancel()

		templ1 := `Name: {{.name}}`
		templ2 := `<div>{{.name}}</div>`

		src1 := tmpDir + "/test1.tmpl"
		src2 := tmpDir + "/test2.tmpl"

		err := os.WriteFile(src1, []byte(templ1), 0755)
		if err != nil {
			t.Errorf("error writing src1:%v", err)
			return
		}
		err = os.WriteFile(src2, []byte(templ2), 0755)
		if err != nil {
			t.Errorf("error writing src2:%v", err)
			return
		}

		dest1 := tmpDir + "/test1.render"
		dest2 := tmpDir + "/dest2.render"

		configs := []sinkExecConfig{{
			sinkConfig: sinkConfig{
				refreshInterval: 1 * time.Second,
				dest:            dest1,
				staticData:      map[string]string{"name": "foo"},
				name:            "template1",
				readFrom:        src1,
			},
		}, {
			sinkConfig: sinkConfig{
				refreshInterval: 1 * time.Second,
				dest:            dest2,
				staticData:      map[string]string{"name": "foo"},
				name:            "template2",
				readFrom:        src2,
			},
			execConfig: &execConfig{
				cmd:  `echo`,
				args: []string{"hello"},
			},
		}}
		p := Process{
			Logger:            newLogger(),
			configs:           configs,
			maxConsecFailures: 10,
		}

		var mu sync.Mutex
		loopRunCounts := map[string]int{}
		tf := tickFunc(func(ctx context.Context, config sinkExecConfig, sink render.Sink) error {
			err := p.renderAndExec(ctx, config, sink)
			switch {
			case errors.Is(err, context.DeadlineExceeded):
			case errors.Is(err, render.ContentsIdentical):
			case err != nil:
				t.Errorf("renderAndExec failed for %s:%v", config.name, err)

			}

			mu.Lock()
			loopRunCounts[config.name]++
			mu.Unlock()
			return nil
		})

		if err := p.startTickLoops(ctx, tf); fatal.Is(err) {
			t.Errorf("startTickLoops failed with error:%v", err)
		}
		for name, lrc := range loopRunCounts {
			if lrc < 3 {
				t.Errorf("loop run count for %s < 3 got:%d", name, lrc)
				return
			}
		}
		bs, err := os.ReadFile(dest1)
		if err != nil {
			t.Errorf("error reading dest1:%v", err)
			return
		}
		expectedContent1 := `Name: foo`
		if string(bs) != expectedContent1 {
			t.Errorf("expectedContent1:\n%s got:\n%s", expectedContent1, string(bs))
			return
		}

		expectedContent2 := `<div>foo</div>`
		bs, err = os.ReadFile(dest2)
		if err != nil {
			t.Errorf("error reading dest2:%v", err)
			return
		}
		if string(bs) != expectedContent2 {
			t.Errorf("expectedContent2:\n%s got:\n%s", expectedContent2, string(bs))
			return
		}
	})
}

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}
