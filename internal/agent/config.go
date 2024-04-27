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

var noTimeout = time.Duration(0)

type AgentConfig struct {
	LogLevel slog.Level `json:"log_level"`
	LogFmt   string     `json:"log_fmt"`
}

type Duration time.Duration

func (r *Duration) UnmarshalJSON(bs []byte) error {
	bs = bytes.Trim(bs, `"`)
	dur, err := time.ParseDuration(string(bs))
	if err != nil {
		return fmt.Errorf("invalid duration string:%w", err)
	}
	*r = Duration(dur)
	return nil
}

type ActionConfig struct {
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
}

type TemplateConfig struct {
	// required for
	// creation of template
	Actions            []ActionConfig `json:"actions"`
	TemplateDelimiters []string       `json:"template_delimiters"`
	Source             string         `json:"source"`
	Raw                string         `json:"raw"`
	Destination        string         `json:"destination"`

	StaticData      any      `json:"static_data"`
	RefreshInterval Duration `json:"refresh_interval"`
	RenderOnce      bool     `json:"render_once"`

	// command exec config
	ExecCMD     string   `json:"exec_cmd"`
	ExecTimeout Duration `json:"exec_timeout"`
}

type Config struct {
	Agent         AgentConfig                `json:"agent"`
	TemplateSpecs map[string]*TemplateConfig `json:"templates"`
}

func ReadConfigFromFile(path string) (Config, error) {
	confFile, err := os.Open(path)
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

	for tmplName, tmplConfig := range c.TemplateSpecs {
		if tmplConfig.RefreshInterval < Duration(1*time.Second) {
			refrIntErr := fmt.Errorf("refresh interval should be >= 1s tmpl name:%s", tmplName)
			valErrs = append(valErrs, refrIntErr)
		}

		if tmplConfig.Source == "" && tmplConfig.Raw == "" {
			srcEmptyErr := fmt.Errorf("expected one of Source OR Raw to be provided tmpl name:%s", tmplName)
			valErrs = append(valErrs, srcEmptyErr)
		}

		if len(tmplConfig.Actions) < 1 {
			continue
		}
		actionValErrs := validateActionConfigs(tmplConfig.Actions)
		if actionValErrs != nil {
			actionValErrs = fmt.Errorf("action validation error for %s:%w", tmplName, actionValErrs)
			valErrs = append(valErrs, actionValErrs)
		}

		delimLen := len(tmplConfig.TemplateDelimiters)
		if delimLen > 0 && delimLen != 2 {
			valErrs = append(valErrs, fmt.Errorf("ivalid tplactions delimiters for %s", tmplName))
		}
	}

	return errors.Join(valErrs...)

}

func validateActionConfigs(actions []ActionConfig) error {
	var provValErrs []error
	for i := range actions {
		if actions[i].Name == "" {
			provValErrs = append(provValErrs, fmt.Errorf("action name cannot be empty for actions[%d]", i))
		}
	}
	return errors.Join(provValErrs...)
}
