package templ

import (
	"fmt"
	"io"
	"os"
	"testing"
	"text/template"
)

func TestRenderer_Render(t *testing.T) {
	t.Run("render when file does not exist", func(t *testing.T) {
		tmpl, err := template.New("test").Parse(`Name: {{.Name}}`)
		if err != nil {
			t.Errorf("error parsing templ:%v", err)
			return
		}
		tmp := t.TempDir()
		renderPath := fmt.Sprintf("%s/test.render", tmp)
		rdr := Renderer{
			Templ:   tmpl,
			WriteTo: renderPath,
		}

		type staticData struct {
			Name string
		}

		if err := rdr.Render(staticData{Name: "foo"}); err != nil {
			t.Errorf("failed to render:%v", err)
			return
		}
		fi, err := os.Open(renderPath)
		if err != nil {
			t.Errorf("error opening file:%v\n", err)
			return
		}
		bs, err := io.ReadAll(fi)
		if err != nil {
			t.Errorf("error reading rendered file contents:%v\n", err)
			return
		}

		expectedContents := `Name: foo`
		if string(bs) != expectedContents {
			t.Errorf("expected %s got %s", expectedContents, string(bs))
		}
	})

}
