package tplactions

import (
	"fmt"
	"log/slog"
	"os"
	"text/template"
)

type Env struct {
	Prefix string
}

func (e Env) Get(name string) string {
	key := fmt.Sprintf("%s_%s", e.Prefix, name)
	return os.Getenv(key)
}

type ConfigDecoder interface {
	Decode(v any) error
}

type Interface interface {
	FuncMap() template.FuncMap
	SetConfig(decoder ConfigDecoder, env Env) error
	SetLogger(logger *slog.Logger)
	Close()
}
