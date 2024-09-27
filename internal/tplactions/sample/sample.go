package sample

import (
	"fmt"
	"github.com/shubhang93/tplagent/internal/tplactions"
	"log/slog"
	"text/template"
)

type Config struct {
	GreetMessage string `json:"greet_message"`
}

type Actions struct {
	Config *Config
}

func (sa *Actions) FuncMap() template.FuncMap {
	return template.FuncMap{
		"greet": func(s string) string {
			return fmt.Sprintf("%s %s", sa.Config.GreetMessage, s)
		},
	}
}

func (sa *Actions) SetConfig(configJSON tplactions.ConfigDecoder, env tplactions.Env) error {
	var c Config
	if err := configJSON.Decode(&c); err != nil {
		return err
	}
	sa.Config = &c
	return nil
}

func (sa *Actions) SetLogger(_ *slog.Logger) {}

func (sa *Actions) Close() {}

func init() {
	tplactions.Register("sample", func() tplactions.Interface {
		return &Actions{}
	})
}
