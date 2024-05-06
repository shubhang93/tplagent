package agent

import (
	"bytes"
	"encoding/json"
	"github.com/google/go-cmp/cmp"
	"github.com/shubhang93/tplagent/internal/actionable"
	"github.com/shubhang93/tplagent/internal/tplactions"
	"github.com/shubhang93/tplagent/internal/tplactions/sample"
	"strings"
	"testing"
)

func Test_template_helpers(t *testing.T) {
	t.Run("attachActions invalid action name", func(t *testing.T) {
		registry := map[string]tplactions.MakeFunc{
			"test": func() tplactions.Interface {
				return &sample.Actions{}
			},
		}
		templ := actionable.NewTemplate("test", false)
		err := attachActions(templ, registry, newLogger(), []ActionsConfig{{
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
				return &sample.Actions{}
			},
		}
		templ := actionable.NewTemplate("test", false)
		err := attachActions(templ, registry, newLogger(), []ActionsConfig{{
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
				return &sample.Actions{}
			},
			"hello": func() tplactions.Interface {
				return &sample.Actions{}
			},
		}
		templ := actionable.NewTemplate("test", false)

		err := attachActions(templ, registry, newLogger(), []ActionsConfig{{
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

	t.Run("when setTemplateDelims is called with less correct delims", func(t *testing.T) {
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

}
