package sample

import (
	"encoding/json"
	"fmt"
	"github.com/shubhang93/tplagent/internal/tplactions"
	"text/template"
)

type Config struct {
	GreetMessage string
}

type SampleAction struct {
	config *Config
}

func (sa *SampleAction) FuncMap() template.FuncMap {
	return template.FuncMap{
		"greet": func(s string) string {
			return fmt.Sprintf("%s %s", sa.config.GreetMessage, s)
		},
	}
}

func (sa *SampleAction) SetConfig(bs []byte) error {
	var c Config
	if err := json.Unmarshal(bs, &c); err != nil {
		return err
	}
	sa.config = &c
	return nil
}

func (sa *SampleAction) Close() {}

func init() {
	tplactions.Register("sample", func() tplactions.Interface {
		return &SampleAction{}
	})
}
