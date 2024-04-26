package templ

import (
	"fmt"
	"io"
	"os"
	"testing"
	"text/template"
	"time"
)

var testTmpl = template.Must(template.New("test").Parse(`Name: {{.Name}}`))

type staticData struct {
	Name string
}

func TestRenderer_Render(t *testing.T) {
	t.Run("render when dest file does not exist", func(t *testing.T) {

		tmp := t.TempDir()
		renderPath := fmt.Sprintf("%s/test.render", tmp)
		rdr := Renderer{
			Templ:   testTmpl,
			WriteTo: renderPath,
		}

		startTime := time.Now()
		if err := rdr.Render(staticData{Name: "foo"}); err != nil {
			t.Errorf("failed to render:%v", err)
			return
		}
		endTime := time.Now()
		t.Logf("render took:%s\n", endTime.Sub(startTime))
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

	t.Run("dest file already exists", func(t *testing.T) {
		tmp := t.TempDir()
		renderPath := fmt.Sprintf("%s/%s", tmp, "test.render")
		fi, err := os.Create(renderPath)
		if err != nil {
			t.Errorf("file create error:%v", err)
			return
		}
		_, err = fi.WriteString("Name: foo")
		if err != nil {
			t.Errorf("write error:%v", err)
			return
		}
		rdr := Renderer{
			Templ:   testTmpl,
			WriteTo: renderPath,
		}

		startTime := time.Now()
		err = rdr.Render(staticData{Name: "baz"})

		if err != nil {
			t.Errorf("render error:%v", err)
			return
		}
		endTime := time.Now()
		t.Logf("redner took:%s", endTime.Sub(startTime))

		fi, err = os.Open(renderPath)
		bs, err := io.ReadAll(fi)
		if err != nil {
			t.Errorf("read error:%v", err)
			return
		}
		expectedContent := `Name: baz`
		if expectedContent != string(bs) {
			t.Errorf("expected:%s got:%s", expectedContent, string(bs))
		}

	})

}
