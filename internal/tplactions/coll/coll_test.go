package coll

import (
	"bytes"
	"testing"
	"text/template"
)

func TestActions_FuncMap(t *testing.T) {
	t.Run("MapGet", func(t *testing.T) {
		as := Actions{}
		templ := template.New("test")
		templ.Funcs(as.FuncMap())
		templ = template.Must(templ.Parse(`name: {{. | MapGet "foo"}}`))
		var buff bytes.Buffer
		err := templ.Execute(&buff, map[string]any{"foo": "bar"})
		if err != nil {
			t.Error(err)
			return
		}
		want := `name: bar`
		if want != buff.String() {
			t.Errorf("want %s got %s", want, buff.String())
		}
	})

	t.Run("SliceGet", func(t *testing.T) {
		as := Actions{}
		templ := template.New("test")
		templ.Funcs(as.FuncMap())
		templ = template.Must(templ.Parse(`name: {{. | SliceGet 1}}`))
		var buff bytes.Buffer
		err := templ.Execute(&buff, []any{"hello", "foo"})
		if err != nil {
			t.Error(err)
			return
		}
		want := `name: foo`
		if want != buff.String() {
			t.Errorf("want %s got %s", want, buff.String())
		}
	})
}
