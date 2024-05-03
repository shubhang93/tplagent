package httpjson

import (
	"bytes"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"net/http"
	"testing"
	"text/template"
	"time"
)

func Test_Actions(t *testing.T) {
	t.Run("GET_Map", func(t *testing.T) {

		go func() {
			startMockServer()
		}()

		time.Sleep(500 * time.Millisecond)
		a := &Actions{}
		err := a.SetConfig([]byte(`{"base_url":"http://localhost:5001","timeout":"5s"}`))
		if err != nil {
			t.Error(err)
			return
		}
		fm := a.FuncMap()

		templ := template.New("test").Funcs(fm)
		templ.Option("missingkey=error")
		if err != nil {
			t.Error(err)
			return
		}
		templ, err = templ.Parse(`{{with GET_Map "/json_map" -}}
 Name: {{.name }}
 ID: {{.id -}} 
{{end}}`)
		var buff bytes.Buffer
		err = templ.Execute(&buff, nil)
		if err != nil {
			t.Error(err)
		}

		expected := `Name: ABCD
 ID: 1234`
		diff := cmp.Diff(expected, buff.String())
		if diff != "" {
			t.Error(diff)
		}

	})

	t.Run("GET_Slice", func(t *testing.T) {

		go func() {
			startMockServer()
		}()

		time.Sleep(500 * time.Millisecond)
		a := &Actions{}
		err := a.SetConfig([]byte(`{"base_url":"http://localhost:5001","timeout":"5s"}`))
		if err != nil {
			t.Error(err)
			return
		}
		fm := a.FuncMap()

		templ := template.New("test").Funcs(fm)
		templ.Option("missingkey=error")
		if err != nil {
			t.Error(err)
			return
		}
		templ, err = templ.Parse(`{{with GET_Slice "/json_slice" -}}
{{range $index,$ele := .}}
{{$index}} => {{$ele -}}
{{end -}}
{{end -}}`)
		var buff bytes.Buffer
		err = templ.Execute(&buff, nil)
		if err != nil {
			t.Error(err)
		}

		t.Log(buff.String())
		expected := `
0 => abcd
1 => efgh
2 => hijk
3 => oj`
		diff := cmp.Diff(expected, buff.String())
		if diff != "" {
			t.Error(diff)
		}

	})

}

func startMockServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /json_map", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = fmt.Fprint(writer, `{"id":"1234","name":"ABCD"}`)
	})

	mux.HandleFunc("GET /json_slice", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = fmt.Fprint(writer, `["abcd","efgh","hijk","oj"]`)
	})

	mux.HandleFunc("GET /any_value", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = fmt.Fprint(writer, "Foo")
	})

	_ = http.ListenAndServe("localhost:5001", mux)
}
