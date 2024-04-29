package actionable

import (
	"github.com/shubhang93/tplagent/internal/tplactions"
	"html/template"
	"io"
)
import texttemp "text/template"

type Template struct {
	html          *template.Template
	text          *texttemp.Template
	activeActions []tplactions.Interface
}

func NewTemplate(name string, html bool, opts ...string) *Template {
	t := &Template{}
	if html {
		t.html = template.New(name)
		return t
	}
	t.text = texttemp.New(name)

	return t
}

func (tt *Template) Parse(text string) error {
	if tt.html != nil {
		_, err := tt.html.Parse(text)
		return err
	}
	_, err := tt.text.Parse(text)
	return err
}

func (tt *Template) Funcs(actions map[string]any) {
	if tt.html != nil {
		tt.html.Funcs(actions)
		return
	}

	tt.text.Funcs(actions)
}

func (tt *Template) SetMissingKeyBehaviour(value string) {
	if value == "" {
		return
	}
	if tt.html != nil {
		tt.html.Option("missingkey=" + value)
		return
	}
	tt.text.Option("missingkey=" + value)
}

func (tt *Template) Execute(writer io.Writer, data any) error {
	if tt.html != nil {
		return tt.html.Execute(writer, data)
	}
	return tt.text.Execute(writer, data)
}

func (tt *Template) Delims(l, r string) {
	if tt.html != nil {
		tt.html.Delims(l, r)
		return
	}
	tt.text.Delims(l, r)
}

func (tt *Template) AddAction(action tplactions.Interface) {
	tt.activeActions = append(tt.activeActions, action)
}

func (tt *Template) CloseActions() {
	for _, a := range tt.activeActions {
		a.Close()
	}
	clear(tt.activeActions)
}
