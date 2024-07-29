package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/shubhang93/tplagent/internal/actionable"
	"github.com/shubhang93/tplagent/internal/config"
	"github.com/shubhang93/tplagent/internal/tplactions"
	"log/slog"
	"strings"
	"testing"
	"text/template"
)

type testActionConfig struct {
	GreetMessage string `json:"greet_message"`
}
type testAction struct {
	Opts   tplactions.SetConfigOpts
	Config testActionConfig
}

func (t *testAction) FuncMap() template.FuncMap {
	return template.FuncMap{
		"greet": func(s string) string {
			return fmt.Sprintf("%s %s", t.Config.GreetMessage, s)
		},
	}
}

func (t *testAction) SetConfig(configJSON []byte, opts tplactions.SetConfigOpts) error {
	t.Opts = opts

	if len(configJSON) < 1 {
		return nil
	}

	if err := json.Unmarshal(configJSON, &t.Config); err != nil {
		return err
	}

	return nil
}

func (t *testAction) SetLogger(logger *slog.Logger) {

}

func (t *testAction) Close() {

}

var _ tplactions.Interface = &testAction{}

func Test_template_helpers(t *testing.T) {
	t.Run("attachActions invalid action name", func(t *testing.T) {
		registry := map[string]tplactions.MakeFunc{
			"test": func() tplactions.Interface {
				return &testAction{}
			},
		}
		templ := actionable.NewTemplate("test", false)
		err := attachActions(templ, registry, newLogger(), []config.Actions{{
			Name:   "sample",
			Config: nil,
		}})

		if err == nil {
			t.Error("nil error")
		}

		if err != nil && !strings.Contains(err.Error(), "invalid action name") {
			t.Errorf("expected error did not match with %s", err.Error())
		}
	})
	t.Run("attachActions set config error", func(t *testing.T) {
		registry := map[string]tplactions.MakeFunc{
			"sample": func() tplactions.Interface {
				return &testAction{}
			},
		}
		templ := actionable.NewTemplate("test", false)
		err := attachActions(templ, registry, newLogger(), []config.Actions{{
			Name:   "sample",
			Config: json.RawMessage(`{"gree":`),
		}})
		if err == nil {
			t.Error("nil error")
		}
		if err != nil && !strings.Contains(err.Error(), "error setting config") {
			t.Errorf("expected error did not match with %s", err.Error())
		}
	})
	t.Run("validActions config", func(t *testing.T) {
		registry := map[string]tplactions.MakeFunc{
			"hey": func() tplactions.Interface {
				return &testAction{}
			},
			"hello": func() tplactions.Interface {
				return &testAction{}
			},
		}
		templ := actionable.NewTemplate("test", false)

		err := attachActions(templ, registry, newLogger(), []config.Actions{{
			Name:   "hey",
			Config: json.RawMessage(`{"greet_message":"hey"}`),
		}, {
			Name:   "hello",
			Config: json.RawMessage(`{"greet_message":"hello"}`),
		}})
		if err != nil {
			t.Error(err)
			return
		}
		raw := `Greetings: {{hey_greet "Foo"}}
{{hello_greet "Foo"}}`

		err = parseTemplate(raw, "", templ)
		if err != nil {
			t.Error(err)
			return
		}

		var buff bytes.Buffer
		err = templ.Execute(&buff, nil)
		if err != nil {
			t.Error(err)
			return
		}

		expected := `Greetings: hey Foo
hello Foo`

		got := buff.String()
		diff := cmp.Diff(expected, got)
		if diff != "" {
			t.Error(diff)
		}

	})
	t.Run("should not panic when setTemplateDelims is called with less than 2 delims", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Error("panicked")
			}
		}()
		templ := actionable.NewTemplate("test", false)
		setTemplateDelims(templ, []string{"<<"})
	})

	t.Run("when setTemplateDelims is called with different delimiters", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Error("panicked")
			}
		}()
		templ := actionable.NewTemplate("test", false)
		setTemplateDelims(templ, []string{"<<", ">>"})
		if err := parseTemplate(`hey <<.name>>`, "", templ); err != nil {
			t.Error(err)
			return
		}

		expected := `hey Foo`

		var buff bytes.Buffer
		err := templ.Execute(&buff, map[string]string{"name": "Foo"})
		if err != nil {
			t.Error(err)
			return
		}

		got := buff.String()
		diff := cmp.Diff(expected, got)
		if diff != "" {
			t.Error(diff)
		}
	})

	t.Run("SetConfigOpts test", func(t *testing.T) {
		ta := testAction{}
		registry := map[string]tplactions.MakeFunc{
			"sample": func() tplactions.Interface {
				return &ta
			},
		}
		templ := actionable.NewTemplate("sample", false)
		err := attachActions(templ, registry, newLogger(), []config.Actions{{
			Name:   "sample",
			Config: nil,
		}})

		if err != nil {
			t.Errorf("error attaching actions:%v\n", err)
			return
		}

		expectedOpts := tplactions.SetConfigOpts{
			EnvPrefix: "TPLA_SAMPLE",
		}

		if diff := cmp.Diff(expectedOpts, ta.Opts); diff != "" {
			t.Error(diff)
		}

	})
}
