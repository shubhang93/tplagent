package render

import (
	"errors"
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

func TestSink_Render(t *testing.T) {
	tmp := t.TempDir()
	t.Run("render when dest file does not exist", func(t *testing.T) {

		renderPath := fmt.Sprintf("%s/test.render", tmp)
		rdr := Sink{
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

		expectedErr := os.ErrNotExist
		_, err = os.Open(fi.Name() + ".bak")
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error:%v got %v", expectedErr, err)
			return
		}

		_, err = os.Stat(rdr.WriteTo + ".temp")
		if !errors.Is(err, expectedErr) {
			t.Error("temp file found")
		}
	})

	t.Run("dest file already exists", func(t *testing.T) {
		renderPath := fmt.Sprintf("%s/%s", tmp, "test.render")
		fi, err := os.Create(renderPath)
		if err != nil {
			t.Errorf("file create error:%v", err)
			return
		}

		expectedBackupContent := "Name: foo"

		_, err = fi.WriteString(expectedBackupContent)
		if err != nil {
			t.Errorf("write error:%v", err)
			return
		}
		rdr := Sink{
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

		bs, err = os.ReadFile(renderPath + ".bak")
		if err != nil {
			t.Errorf("error reading backup:%v", err)
			return
		}

		if string(bs) != expectedBackupContent {
			t.Errorf("expected backup content: %s\n got:%s\n", expectedBackupContent, string(bs))
		}

		if _, err := os.Stat(rdr.WriteTo + ".temp"); !os.IsNotExist(err) {
			t.Error("temp file found")
		}

	})

	t.Run("create intermediate paths if none exists", func(t *testing.T) {
		dest := fmt.Sprintf("%s/%s/%s", tmp, "extradir", "test.render")
		s := Sink{
			Templ:   template.Must(template.New("test").Parse(`Name:{{.Name}}`)),
			WriteTo: dest,
		}
		err := s.Render(staticData{Name: "foo"})
		if err != nil {
			t.Errorf("render error:%v", err)
		}
	})

}
