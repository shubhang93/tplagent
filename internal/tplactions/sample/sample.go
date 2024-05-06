package sample

import (
	"encoding/json"
	"fmt"
	"github.com/shubhang93/tplagent/internal/tplactions"
	"log/slog"
	"text/template"
)

type Config struct {
	GreetMessage string `json:"greet_message"`
}

type Actions struct {
	config *Config
}

func (sa *Actions) FuncMap() template.FuncMap {
	return template.FuncMap{
		"greet": func(s string) string {
			return fmt.Sprintf("%s %s", sa.config.GreetMessage, s)
		},
	}
}

func (sa *Actions) SetConfig(bs []byte) error {
	var c Config
	if err := json.Unmarshal(bs, &c); err != nil {
		return err
	}
	sa.config = &c
	return nil
}

func (sa *Actions) SetLogger(_ *slog.Logger) {}

func (sa *Actions) Close() {}

func init() {
	tplactions.Register("sample", func() tplactions.Interface {
		return &Actions{}
	})
}
