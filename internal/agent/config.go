package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"
)

var allowedLogFmts = map[string]struct{}{
	"json": {},
	"text": {},
}

type Agent struct {
	LogLevel slog.Level `json:"log_level"`
	LogFmt   string     `json:"log_fmt"`
}

type RefreshInterval time.Duration

func (r *RefreshInterval) UnmarshalJSON(bs []byte) error {
	bs = bytes.Trim(bs, `"`)
	dur, err := time.ParseDuration(string(bs))
	if err != nil {
		return fmt.Errorf("invalid duration string:%w", err)
	}
	*r = RefreshInterval(dur)
	return nil
}

type Provider struct {
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
}

type Template struct {
	RefreshInterval  RefreshInterval `json:"refresh_interval"`
	BeforeRenderCMD  string          `json:"before_render_cmd"`
	AfterRenderCMD   string          `json:"after_render_cmd"`
	Source           string          `json:"source"`
	Raw              string          `json:"raw"`
	Destination      string          `json:"destination"`
	Providers        []Provider      `json:"providers"`
	ActionDelimiters [2]string       `json:"action_delimiters"`
}

type Config struct {
	Agent     Agent               `json:"agent"`
	Templates map[string]Template `json:"templates"`
}

func readConfigFromFile(path string) (Config, error) {
	confFile, err := os.OpenFile(path, os.O_RDONLY, 0755)
	if err != nil {
		return Config{}, fmt.Errorf("config file read error:%w", err)
	}
	return readConfig(confFile)
}

func readConfig(rr io.Reader) (Config, error) {
	var c Config
	if err := json.NewDecoder(rr).Decode(&c); err != nil {
		return Config{}, fmt.Errorf("config decode error:%w", err)
	}
	if err := validateConfig(&c); err != nil {
		return Config{}, err
	}
	return c, nil
}

func validateConfig(c *Config) error {
	var valErrs []error
	if _, ok := allowedLogFmts[c.Agent.LogFmt]; !ok {
		valErrs = append(valErrs, fmt.Errorf("agent config invalid invalid log level:%s", c.Agent.LogFmt))
	}

	for tmplName, tmplConfig := range c.Templates {
		if tmplConfig.RefreshInterval < RefreshInterval(1*time.Second) {
			refrIntErr := fmt.Errorf("refresh interval should be >= 1s tmpl name:%s", tmplName)
			valErrs = append(valErrs, refrIntErr)
		}

		if tmplConfig.Source == "" && tmplConfig.Raw == "" {
			srcEmptyErr := fmt.Errorf("expected one of Source OR Raw to be provided tmpl name:%s", tmplName)
			valErrs = append(valErrs, srcEmptyErr)
		}

		if len(tmplConfig.Providers) < 1 {
			continue
		}
		provValErr := validateProviderConfigs(tmplConfig.Providers)
		if provValErr != nil {
			provValErr = fmt.Errorf("provider validation error for %s:%w", tmplName, provValErr)
			valErrs = append(valErrs, provValErr)
		}
	}

	return errors.Join(valErrs...)

}

func validateProviderConfigs(provs []Provider) error {
	var provValErrs []error
	for i := range provs {
		if provs[i].Name == "" {
			provValErrs = append(provValErrs, fmt.Errorf("provider name cannot be empty for providers[%d]", i))
		}
	}
	return errors.Join(provValErrs...)
}
