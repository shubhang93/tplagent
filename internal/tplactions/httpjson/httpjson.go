package httpjson

import (
	"encoding/json"
	"fmt"
	"github.com/shubhang93/tplagent/internal/duration"
	"github.com/shubhang93/tplagent/internal/tplactions"
	"io"
	"net/http"
	"net/url"
	"slices"
	"text/template"
	"time"
)

type BearerToken string

type Auth struct {
	BasicAuth   map[string]string `json:"omit_empty"`
	BearerToken *BearerToken      `json:"bearer_token,omitempty"`
}

type Config struct {
	BaseURL       string            `json:"base_url"`
	Auth          *Auth             `json:"auth"`
	Timeout       duration.Duration `json:"timeout"`
	ErrorStatuses []int             `json:"error_statuses"`
}

type Actions struct {
	*http.Client
	Conf Config
}

func (a *Actions) getAndReadBody(endpoint string) ([]byte, error) {
	fullURL, err := url.JoinPath(a.Conf.BaseURL, endpoint)
	if err != nil {
		return nil, err
	}
	resp, err := a.Client.Get(fullURL)
	if err != nil {
		return nil, err
	}

	if slices.Contains(a.Conf.ErrorStatuses, resp.StatusCode) {
		return nil, fmt.Errorf("req for %s failed with status %d", endpoint, resp.StatusCode)
	}

	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return bs, nil
}

func (a *Actions) FuncMap() template.FuncMap {
	return map[string]any{
		"GET_Map": func(endpoint string) (map[string]any, error) {
			bs, err := a.getAndReadBody(endpoint)
			if err != nil {
				return nil, err
			}

			var m map[string]any
			if err := json.Unmarshal(bs, &m); err != nil {
				return nil, err
			}
			return m, nil
		},
		"GET_Slice": func(endpoint string) ([]any, error) {
			bs, err := a.getAndReadBody(endpoint)
			if err != nil {
				return nil, err
			}
			var s []any
			if err := json.Unmarshal(bs, &s); err != nil {
				return nil, err
			}
			return s, nil
		},
	}
}

func (a *Actions) SetConfig(bs []byte) error {

	var c Config
	if err := json.Unmarshal(bs, &c); err != nil {
		return fmt.Errorf("error unmarshalling config:%w", err)
	}

	a.Conf = c

	a.Client = &http.Client{
		Timeout: time.Duration(a.Conf.Timeout),
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
