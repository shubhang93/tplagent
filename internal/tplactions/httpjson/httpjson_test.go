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

func Test_Actions_Auth(t *testing.T) {
	t.Run("basic auth", func(t *testing.T) {
		a := Actions{Conf: Config{Auth: &Auth{
			BasicAuth: map[string]string{"username": "foo", "password": "bar"},
		}}}
		req, err := a.newRequest("/foo", http.MethodGet, nil)
		if err != nil {
			t.Error(err)
			return
		}
		user, pass, ok := req.BasicAuth()
		if !ok {
			t.Error("setting basic auth failed")
			return
		}
		if user != "foo" {
			t.Error("invalid user")
			return
		}
		if pass != "bar" {
			t.Error("invalid password")
		}
	})
	t.Run("bearer auth", func(t *testing.T) {
		token := "Bearer foo"
		a := Actions{Conf: Config{Auth: &Auth{
			BasicAuth:   map[string]string{},
			BearerToken: (*BearerToken)(&token),
		}}}
		req, err := a.newRequest("/foo", http.MethodGet, nil)
		if err != nil {
			t.Error(err)
			return
		}
		_, _, ok := req.BasicAuth()
		if ok {
			t.Error("basic auth should not be set")
			return
		}
		authHeader := req.Header.Get("Authorization")
		if authHeader != "Bearer foo" {
			t.Error("invalid authorization header")
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
