package agent

import (
	"context"
	"fmt"
	"github.com/shubhang93/tplagent/internal/actionable"
	"github.com/shubhang93/tplagent/internal/render"
	"github.com/shubhang93/tplagent/internal/tplactions"
	"log/slog"
	"os"
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

func Test_initTemplates(t *testing.T) {
	t.Run("invalid template file name", func(t *testing.T) {
		config := map[string]*TemplateConfig{
			"testconfig": {
				Actions: []ActionConfig{{
					Name:   "foo",
					Config: []byte{},
				}},
				Source: "/tmp/foo",
			},
			"testconfig2": {
				Source: "/tmp/bar",
			},
		}

		_, err := initTemplates(config)
		if err == nil {
			t.Errorf("expected error got nil")
			return
		}
		t.Log(err)
	})
	t.Run("invalid template from Raw", func(t *testing.T) {
		config := map[string]*TemplateConfig{
			"testconfig": {
				Raw: "Name: {{}",
			},
		}
		_, err := initTemplates(config)
		if err == nil {
			t.Errorf("expected error got nil")
			return
		}
		t.Log(err)
	})

	t.Run("invalid action name", func(t *testing.T) {
		config := map[string]*TemplateConfig{
			"testconfig": {
				Actions: []ActionConfig{{
					Name:   "unknownAction",
					Config: []byte{},
				}},
				Raw:        "Name: {{.Name}}",
				StaticData: nil,
			},
			"testconfig2": {
				Actions: []ActionConfig{{
					Name:   "unknownAction",
					Config: []byte{},
				}},
				Raw:        "Name: {{.Name}}",
				StaticData: nil,
			},
		}
		_, err := initTemplates(config)
		if err == nil {
			t.Errorf("expected error got nil")
			return
		}
		t.Log(err)
	})

	t.Run("valid config", func(t *testing.T) {

		tplactions.Register("fooAction", func() tplactions.Interface {
			return mockAction{}
		})

		config := map[string]*TemplateConfig{
			"testconfig": {
				Actions: []ActionConfig{{
					Name:   "fooAction",
					Config: []byte{},
				}},
				Raw:        "Name: {{.Name}}",
				StaticData: nil,
			},
			"testconfig2": {
				Actions: []ActionConfig{{
					Name:   "fooAction",
					Config: []byte{},
				}},
				Raw:        "Name: {{.Name}}",
				StaticData: nil,
			},
		}
		sinkExecCfgs, err := initTemplates(config)
		if err != nil {
			t.Errorf("init failed with:%v", err)
			return
		}
		if len(sinkExecCfgs) != len(config) {
			t.Errorf("expected len %d got %d", len(config), len(sinkExecCfgs))
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

			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

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

}
