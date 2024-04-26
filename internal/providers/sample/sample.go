package sample

import (
	"encoding/json"
	"fmt"
	"github.com/shubhang93/tplagent/internal/providers"
	"text/template"
)

type Config struct {
	GreetMessage string
}

type Provider struct {
	config *Config
}

func (p *Provider) FuncMap() template.FuncMap {
	return template.FuncMap{
		"greet": func(s string) string {
			return fmt.Sprintf("%s ==> %s", p.config.GreetMessage, s)
		},
	}
}

func (p *Provider) SetConfig(bs []byte) error {
	var c Config
	if err := json.Unmarshal(bs, &c); err != nil {
		return err
	}
	p.config = &c
	return nil
}

func init() {
	providers.Register("sample", func() providers.Interface {
		return &Provider{}
	})
}
