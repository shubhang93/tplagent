package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/shubhang93/tplagent/internal/duration"
	"github.com/shubhang93/tplagent/internal/fatal"
	"io"
	"log/slog"
	"os"
	"time"
)

var allowedLogFmts = map[string]struct{}{
	"json": {},
	"text": {},
}

type AgentConfig struct {
	LogLevel               slog.Level `json:"log_level"`
	LogFmt                 string     `json:"log_fmt"`
	MaxConsecutiveFailures int        `json:"max_consecutive_failures"`
}

type ActionsConfig struct {
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
}

type ExecConfig struct {
	Cmd        string            `json:"cmd"`
	CmdArgs    []string          `json:"cmd_args"`
	CmdTimeout duration.Duration `json:"cmd_timeout"`
}

type TemplateConfig struct {
	// required for
	// creation of template
	Actions            []ActionsConfig   `json:"actions,omitempty"`
	TemplateDelimiters []string          `json:"template_delimiters,omitempty"`
	Source             string            `json:"source,omitempty"`
	Raw                string            `json:"raw,omitempty"`
	Destination        string            `json:"destination,omitempty"`
	HTML               bool              `json:"html"`
	StaticData         any               `json:"static_data,omitempty"`
	RefreshInterval    duration.Duration `json:"refresh_interval,omitempty"`
	RenderOnce         bool              `json:"render_once,omitempty"`
	MissingKey         string            `json:"missing_key"`

	Exec *ExecConfig `json:"exec"`
}

type Config struct {
	Agent         AgentConfig                `json:"agent"`
	TemplateSpecs map[string]*TemplateConfig `json:"templates"`
}

func ReadConfigFromFile(path string) (Config, error) {
	confFile, err := os.Open(os.ExpandEnv(path))
	if err != nil {
		return Config{}, fatal.NewError(fmt.Errorf("read config:%w", err))
	}
	return readConfig(confFile)
}

func readConfig(rr io.Reader) (Config, error) {
	var c Config
	if err := json.NewDecoder(rr).Decode(&c); err != nil {
		return Config{}, fatal.NewError(fmt.Errorf("config decode error:%w", err))
	}
	if err := validateConfig(&c); err != nil {
		return Config{}, fatal.NewError(err)
	}
	return c, nil
}

func validateConfig(c *Config) error {
	var valErrs []error
	if _, ok := allowedLogFmts[c.Agent.LogFmt]; !ok {
		valErrs = append(valErrs, fmt.Errorf("validate:invalid log level"))
	}

	for tmplName, tmplConfig := range c.TemplateSpecs {

		if tmplName == "" {
			return errors.New(`validate:found "" as the template key`)
		}

		refrInterval := tmplConfig.RefreshInterval
		if refrInterval > 0 && refrInterval < duration.Duration(1*time.Second) {
			refrIntErr := fmt.Errorf("validate:refresh interval should be >= 1s tmpl name:%s", tmplName)
			valErrs = append(valErrs, refrIntErr)
		}

		if tmplConfig.Source == "" && tmplConfig.Raw == "" {
			srcEmptyErr := fmt.Errorf("validate:expected one of Source OR Raw to be provided tmpl %s", tmplName)
			valErrs = append(valErrs, srcEmptyErr)
		}

		if len(tmplConfig.Actions) < 1 {
			continue
		}
		actionValErrs := validateActionConfigs(tmplConfig.Actions)
		if actionValErrs != nil {
			actionValErrs = fmt.Errorf("validate:action invalid for %s:%w", tmplName, actionValErrs)
			valErrs = append(valErrs, actionValErrs)
		}

		delimLen := len(tmplConfig.TemplateDelimiters)
		if delimLen > 0 && delimLen != 2 {
			valErrs = append(valErrs, fmt.Errorf("validate: invalid tplactions delimiters for %s", tmplName))
		}
	}

	return errors.Join(valErrs...)

}

func validateActionConfigs(actions []ActionsConfig) error {
	var provValErrs []error
	for i := range actions {
		if actions[i].Name == "" {
			provValErrs = append(provValErrs, fmt.Errorf("validate: action name cannot be empty for actions[%d]", i))
		}
	}
	return errors.Join(provValErrs...)
}
