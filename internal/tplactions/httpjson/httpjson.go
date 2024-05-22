package httpjson

import (
	"encoding/json"
	"fmt"
	"github.com/shubhang93/tplagent/internal/duration"
	"github.com/shubhang93/tplagent/internal/tplactions"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
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
	Headers       map[string]string `json:"headers"`
	ErrorStatuses []int             `json:"error_statuses"`
}

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Actions struct {
	Client httpDoer
	Conf   Config
}

func (a *Actions) SetLogger(_ *slog.Logger) {}

func (a *Actions) getAndReadBody(endpoint string) ([]byte, error) {
	fullURL, err := url.JoinPath(a.Conf.BaseURL, endpoint)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix("http://", endpoint) || strings.HasPrefix("https://", endpoint) {
		fullURL = endpoint
	}

	req, err := a.newRequest(fullURL, http.MethodGet, nil)
	resp, err := a.Client.Do(req)
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

func (a *Actions) Close() {}

func (a *Actions) newRequest(url string, method string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if a.Conf.Auth != nil {
		setAuth(req, a.Conf.Auth)
	}

	if a.Conf.Headers != nil {
		setHeaders(req, a.Conf.Headers)
	}

	return req, nil
}

func setAuth(r *http.Request, a *Auth) {
	if a.BasicAuth != nil && len(a.BasicAuth) > 0 {
		uname, pswd := a.BasicAuth["username"], a.BasicAuth["password"]
		r.SetBasicAuth(os.ExpandEnv(uname), os.ExpandEnv(pswd))
		return
	}
	if a.BearerToken != nil {
		r.Header.Set("Authorization", string(*a.BearerToken))
	}
}

func setHeaders(r *http.Request, hs map[string]string) {
	for k, v := range hs {
		r.Header.Set(os.ExpandEnv(k), os.ExpandEnv(v))
	}
}

func init() {
	tplactions.Register("httpjson", func() tplactions.Interface {
		return &Actions{}
	})
}
