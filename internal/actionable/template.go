package actionable

import (
	"html/template"
	"io"
)
import texttemp "text/template"

type Template struct {
	html *template.Template
	text *texttemp.Template
}

func NewTemplate(name string, html bool) *Template {
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
