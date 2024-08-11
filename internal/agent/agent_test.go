package agent

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	gocmp "github.com/google/go-cmp/cmp"
	"github.com/shubhang93/tplagent/internal/actionable"
	config2 "github.com/shubhang93/tplagent/internal/config"
	"github.com/shubhang93/tplagent/internal/duration"
	"github.com/shubhang93/tplagent/internal/fatal"
	"github.com/shubhang93/tplagent/internal/render"
	"log/slog"
	"os"
	"slices"
	"sync"
	"sync/atomic"
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
		config := map[string]*config2.TemplateSpec{
			"testconfig": {
				Source:          "$HOME/testdir",
				Destination:     "${HOME}/testdir2",
				HTML:            false,
				StaticData:      map[string]any{},
				RefreshInterval: duration.Duration(1 * time.Second),
				RenderOnce:      true,
				Exec: &config2.ExecSpec{
					Cmd:        "echo",
					CmdArgs:    []string{"hello"},
					CmdTimeout: duration.Duration(5 * time.Second),
				},
			},
			"testconfig2": {
				Actions: []config2.Actions{{
					Name:   "httpJson",
					Config: []byte(`{"key":"value"}`),
				}},
				TemplateDelimiters: []string{"<<", ">>"},
				HTML:               true,
				StaticData:         map[string]any{},
				RenderOnce:         true,
				Exec: &config2.ExecSpec{
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
					timeout: 5 * time.Second,
					args:    []string{"hello"},
					cmd:     "echo",
					env:     nil,
				},
			},
			{
				sinkConfig: sinkConfig{
					html:           true,
					templateDelims: []string{"<<", ">>"},
					actions: []config2.Actions{{
						Name:   "httpJson",
						Config: []byte(`{"key":"value"}`),
					}},
					staticData: map[string]any{},
					name:       "testconfig2",
					renderOnce: true,
				},
				execConfig: &execConfig{
					args:    []string{"hello"},
					cmd:     "echo",
					timeout: 30 * time.Second,
				},
			}}

		seConfigs := sanitizeConfigs(config)
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

	const timeoutMS = 5000
	tmpDir := t.TempDir()
	renderPath := fmt.Sprintf("%s/%s", tmpDir, "test.render")
	tpl := actionable.NewTemplate("test", false)
	must(tpl.Parse("Name {{.name}}"))

	type loopTest struct {
		name             string
		wantAtleastCount int
		cfg              sinkExecConfig
	}

	const refersIntervalMS = 500
	ltests := []loopTest{{
		name:             "render once is false",
		wantAtleastCount: 8,
		cfg: sinkExecConfig{
			sinkConfig: sinkConfig{
				parsed:          tpl,
				refreshInterval: refersIntervalMS * time.Millisecond,
				dest:            renderPath,
				staticData:      map[string]any{"name": "foo"},
				name:            "test-tmpl",
			},
			execConfig: &execConfig{
				args:    []string{"hello"},
				cmd:     "echo",
				timeout: 30 * time.Second,
			},
		},
	}, {
		name:             "render once is true",
		wantAtleastCount: 1,
		cfg: sinkExecConfig{
			sinkConfig: sinkConfig{
				renderOnce:      true,
				parsed:          tpl,
				refreshInterval: refersIntervalMS * time.Millisecond,
				dest:            renderPath,
				staticData:      map[string]any{"name": "foo"},
				name:            "test-tmpl",
			},
			execConfig: &execConfig{
				cmd:     `echo`,
				args:    []string{"hello"},
				env:     nil,
				timeout: 30 * time.Second,
			},
		},
	}}

	for _, ltest := range ltests {
		t.Run(ltest.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeoutMS*time.Millisecond)
			defer cancel()

			execCount := 0
			onTick := func(ctx context.Context, sink Renderer, execer CMDExecer, data any) error {
				execCount++
				return nil
			}

			p := Proc{
				Logger:            newLogger(),
				maxConsecFailures: 10,
				TickFunc:          onTick,
			}
			err := p.startRenderLoop(ctx, ltest.cfg)
			if err != nil && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("%v", err)
			}

			if execCount < ltest.wantAtleastCount {
				t.Errorf("expected count atleast %d got %d", ltest.wantAtleastCount, execCount)
			}
		})
	}

	t.Run("test consec failures", func(t *testing.T) {
		proc := &Proc{
			Logger: newLogger(),
			TickFunc: func(ctx context.Context, _ Renderer, _ CMDExecer, _ any) error {
				return errors.New("error occurred")
			},
			maxConsecFailures: 5,
		}
		cfg := sinkExecConfig{
			sinkConfig: sinkConfig{
				name:            "test",
				parsed:          actionable.NewTemplate("test", false),
				refreshInterval: 1 * time.Second,
			},
		}
		err := proc.startRenderLoop(context.Background(), cfg)
		if !errors.Is(err, errTooManyFailures) {
			t.Errorf("expected: %v got: %v", errTooManyFailures, err)
		}
	})

	t.Run("reset consec failures reset", func(t *testing.T) {

		tickCount := 0
		proc := Proc{
			TickFunc: func(ctx context.Context, _ Renderer, _ CMDExecer, _ any) error {
				tickCount++
				switch tickCount {
				case 1, 2, 3:
					return errors.New("error occurred")
				}
				return render.ContentsIdentical
			},
			Logger:            newLogger(),
			maxConsecFailures: 4,
		}
		cfg := sinkExecConfig{
			sinkConfig: sinkConfig{
				name:            "test",
				parsed:          actionable.NewTemplate("test", false),
				refreshInterval: 1 * time.Second,
			},
		}

		n := time.Duration(6)
		ctx, cancel := context.WithTimeout(context.Background(), n*time.Second)
		defer cancel()

		err := proc.startRenderLoop(ctx, cfg)
		if errors.Is(err, errTooManyFailures) {
			t.Error(err)
		}
		if tickCount < int(n-2) {
			t.Errorf("expected tick count to be atleast %d got %d", int(n), tickCount)
		}
	})

	t.Run("init template", func(t *testing.T) {

		p := Proc{Logger: newLogger()}

		err := p.initTemplate(&sinkExecConfig{
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
					actions: []config2.Actions{{
						Name:   "fooaction",
						Config: nil,
					}},
				},
			}}

		tf := tickFunc(func(ctx context.Context, _ Renderer, _ CMDExecer, _ any) error {
			return nil
		})
		p := Proc{
			Logger:   newLogger(),
			configs:  scs,
			TickFunc: tf,
		}

		err := p.startTickLoops(context.Background())
		if err == nil {
			t.Error("expected error but got nil")
			return
		}
		t.Log(err)
	})

	t.Run("test render and refresh for a valid config", func(t *testing.T) {

		runFor := 5 * time.Second

		ctx, cancel := context.WithTimeout(context.Background(), runFor)
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

		refreshInterval := 1 * time.Second
		configs := []sinkExecConfig{{
			sinkConfig: sinkConfig{
				refreshInterval: refreshInterval,
				dest:            dest1,
				staticData:      map[string]string{"name": "foo"},
				name:            "template1",
				readFrom:        src1,
			},
		}, {
			sinkConfig: sinkConfig{
				refreshInterval: refreshInterval,
				dest:            dest2,
				staticData:      map[string]string{"name": "foo"},
				name:            "template2",
				readFrom:        src2,
				html:            true,
			},
			execConfig: &execConfig{
				cmd:  `echo`,
				args: []string{"hello"},
			},
		}}

		var mu sync.Mutex
		loopRunCounts := map[Renderer]int{}
		tf := tickFunc(func(ctx context.Context, sink Renderer, execer CMDExecer, data any) error {
			err := RenderAndExec(ctx, sink, execer, data)
			switch {
			case errors.Is(err, context.DeadlineExceeded):
			case errors.Is(err, render.ContentsIdentical):
			case err != nil:
				t.Errorf("RenderAndExec failed for %v", err)

			}

			mu.Lock()
			loopRunCounts[sink]++
			mu.Unlock()
			return nil
		})
		p := Proc{
			Logger:            newLogger(),
			configs:           configs,
			maxConsecFailures: 10,
			TickFunc:          tf,
		}

		if err := p.startTickLoops(ctx); fatal.Is(err) {
			t.Errorf("startTickLoops failed with error:%v", err)
		}

		NloopRuns := runFor / refreshInterval

		for name, lrc := range loopRunCounts {
			if lrc < int(NloopRuns-2) {
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

	t.Run("all configs render once", func(t *testing.T) {
		tmp := t.TempDir()
		p := Proc{
			Logger:   newLogger(),
			TickFunc: RenderAndExec,
			configs: []sinkExecConfig{{
				sinkConfig: sinkConfig{
					name:       "test-render",
					dest:       tmp + "/test.render",
					raw:        "hello foo",
					renderOnce: true,
				},
				execConfig: nil,
			}, {
				sinkConfig: sinkConfig{
					name:       "test2-render",
					dest:       tmp + "/test2.render",
					raw:        "hello bar",
					renderOnce: true,
				},
				execConfig: nil,
			}},
			maxConsecFailures: defaultMaxConsecFailures,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		if err := p.startTickLoops(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("test failed with error:%v", err)
		}
	})

	t.Run("refresh triggers", func(t *testing.T) {
		tests := map[string]struct {
			Configs   func(tmp string) []sinkExecConfig
			WantCount int32
		}{
			"render once is true": {
				Configs: func(tmp string) []sinkExecConfig {
					return []sinkExecConfig{
						{
							sinkConfig: sinkConfig{
								name:       "test-render",
								dest:       tmp + "/test.render",
								raw:        "hello foo",
								renderOnce: true,
							},
							execConfig: nil,
						}, {
							sinkConfig: sinkConfig{
								name:       "test2-render",
								dest:       tmp + "/test2.render",
								raw:        "hello bar",
								renderOnce: true,
							},
							execConfig: nil,
						}}
				},
				WantCount: 4,
			},
			"refresh interval is set": {
				Configs: func(tmp string) []sinkExecConfig {
					return []sinkExecConfig{{
						sinkConfig: sinkConfig{
							name:            "test-render",
							dest:            tmp + "/test.render",
							raw:             "hello foo",
							refreshInterval: 2500 * time.Millisecond,
						},
						execConfig: nil,
					}, {
						sinkConfig: sinkConfig{
							name:            "test2-render",
							dest:            tmp + "/test2.render",
							raw:             "hello bar",
							refreshInterval: 2500 * time.Millisecond,
						},
						execConfig: nil,
					}}
				},
				WantCount: 4,
			},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				tickCount := atomic.Int32{}

				tmp := t.TempDir()

				p := Proc{
					Logger: newLogger(),
					TickFunc: func(ctx context.Context, sink Renderer, execer CMDExecer, staticData any) error {
						tickCount.Add(1)
						return nil
					},
					configs:           tc.Configs(tmp),
					refreshTriggers:   make(map[string]triggerFlow),
					maxConsecFailures: defaultMaxConsecFailures,
				}

				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				done := make(chan struct{})
				go func() {
					defer close(done)
					if err := p.startTickLoops(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
						t.Errorf("test failed with error:%v", err)
					}
				}()

				wantTickCount := tc.WantCount
				time.Sleep(500 * time.Millisecond)
				if err := p.TriggerRefresh("test-render"); err != nil {
					t.Errorf("refresh trigger failed for %s:%v", "test-render", err)
				}

				if err := p.TriggerRefresh("test2-render"); err != nil {
					t.Errorf("refresh trigger failed for %s:%v", "test-render", err)
				}

				<-done
				gotTickCount := tickCount.Load()
				if wantTickCount != gotTickCount {
					t.Errorf("want count:%d got count:%d", wantTickCount, gotTickCount)
				}
				cancel()
			})
		}

	})
	
}

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}
