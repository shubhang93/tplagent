package httpjson

import (
	"encoding/json"
	"github.com/shubhang93/tplagent/internal/tplactions"
	"io"
	"net/http"
	"text/template"
	"time"
)

type Actions struct {
	*http.Client
}

func (a *Actions) FuncMap() template.FuncMap {
	return map[string]any{
		"getJSONMap": func(url string) map[string]any {
			resp, err := a.Client.Get(url)
			if err != nil {
				return nil
			}
			bs, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil
			}
			var m map[string]any
			if err := json.Unmarshal(bs, &m); err != nil {
				return nil
			}
			return m
		},
	}
}

func (a *Actions) SetConfig(_ []byte) error {
	a.Client = &http.Client{
		Timeout: 10 * time.Second,
	}
	return nil
}

func (a *Actions) Close() {
	a.Client.CloseIdleConnections()
}

func init() {
	tplactions.Register("httpjson", func() tplactions.Interface {
		return &Actions{}
	})
}
