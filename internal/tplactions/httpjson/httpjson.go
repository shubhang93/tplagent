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
	"strconv"
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

type Actions struct {
	Client *http.Client
	Conf   Config
}

func overrideConfigFromEnv(env tplactions.Env, c *Config) error {
	baseURL := env.Get("HTTPJSON_BASE_URL")
	authUser := env.Get("HTTPJSON_AUTH_USER")
	authPass := env.Get("HTTPJSON_AUTH_PASS")
	headers := env.Get("HTTPJSON_HEADERS")
	errorStatuses := env.Get("HTTPJSON_ERROR_STATUSES")
	timeout := env.Get("HTTPJSON_TIMEOUT")
	authToken := env.Get("HTTPJSON_AUTH_TOKEN")

	if baseURL != "" {
		c.BaseURL = baseURL
	}
	if authUser != "" && c.Auth.BasicAuth != nil {
		c.Auth.BasicAuth["username"] = authUser
	}

	if authPass != "" && c.Auth.BasicAuth != nil {
		c.Auth.BasicAuth["password"] = authPass
	}

	if authToken != "" {
		bt := BearerToken(authToken)
		c.Auth.BearerToken = &bt
	}

	if len(errorStatuses) > 0 {
		statuses, err := parseErrorStatuses(errorStatuses)
		if err != nil {
			return fmt.Errorf("error reading key:%s:%w", "HTTPJSON_ERROR_STATUSES", err)
		}
		c.ErrorStatuses = statuses
	}

	if len(timeout) > 0 {
		td, err := time.ParseDuration(timeout)
		if err != nil {
			return fmt.Errorf("error reading key:%s:%w", "HTTPJSON_TIMEOUT", err)
		}

		c.Timeout = duration.Duration(td)
	}

	if headers != "" {
		parts := strings.Split(headers, ";")
		for _, part := range parts {
			kvs := strings.Split(part, ":")
			k := strings.Trim(kvs[0], " ")
			v := strings.Trim(kvs[1], " ")
			c.Headers[k] = v
		}
	}

	return nil
}

func parseErrorStatuses(statuses string) ([]int, error) {
	parts := strings.Split(statuses, ";")
	temp := make([]int, len(statuses))
	for i, part := range parts {
		parsed, err := strconv.Atoi(part)
		if err != nil {
			return nil, err
		}
		temp[i] = parsed
	}
	return temp, nil
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

func (a *Actions) SetConfig(decoder tplactions.ConfigDecoder, env tplactions.Env) error {

	var c Config
	if err := decoder.Decode(&c); err != nil {
		return err
	}

	a.Conf = c

	if a.Client == nil {
		a.Client = &http.Client{
			Timeout: time.Duration(a.Conf.Timeout),
		}
	}

	if err := overrideConfigFromEnv(env, &a.Conf); err != nil {
		return err
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

func newAction() tplactions.Interface {
	return &Actions{
		Conf: Config{
			Auth:    &Auth{BasicAuth: map[string]string{}},
			Headers: map[string]string{},
		},
	}
}

func init() {
	tplactions.Register("httpjson", newAction)
}
