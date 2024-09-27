package httpjson

import (
	"bytes"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/shubhang93/tplagent/internal/config"
	"github.com/shubhang93/tplagent/internal/tplactions"
	"io"
	"net/http"
	"strings"
	"testing"
	"text/template"
)

type mockTransport struct{}

func (m mockTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	makeBody := func(content string) io.ReadCloser {
		return io.NopCloser(strings.NewReader(content))
	}
	path := request.URL.Path
	switch path {
	case "/any_value":
		return &http.Response{
			StatusCode: 200,
			Body:       makeBody(`Foo`),
		}, nil

	case "/json_slice":
		return &http.Response{
			StatusCode: 200,
			Body:       makeBody(`["abcd","efgh","hijk","oj"]`),
		}, nil
	case "/json_map":
		return &http.Response{
			StatusCode: 200,
			Body:       makeBody(`{"id":"1234","name":"ABCD"}`),
		}, nil
	default:
		return nil, fmt.Errorf("unknown path:%s", path)
	}
}

func Test_Actions(t *testing.T) {
	t.Run("GET_Map", func(t *testing.T) {

		a := &Actions{
			Client: &http.Client{
				Transport: mockTransport{},
			},
		}

		rm := config.NewJSONRawMessage([]byte(`{"base_url":"http://localhost:5001","timeout":"5s"}`))

		err := a.SetConfig(rm, tplactions.Env{})
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

		a := &Actions{
			Client: &http.Client{Transport: mockTransport{}},
		}

		c := config.NewJSONRawMessage([]byte(`{"base_url":"http://localhost:5001","timeout":"5s"}`))

		err := a.SetConfig(c, tplactions.Env{})
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
		}},
			Client: &http.Client{Transport: mockTransport{}},
		}
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
		a := Actions{
			Client: &http.Client{
				Transport: mockTransport{},
			},
			Conf: Config{Auth: &Auth{
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
