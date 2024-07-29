package tplactions

import (
	"log/slog"
	"text/template"
)

type SetConfigOpts struct {
	EnvPrefix string
}

type Interface interface {
	FuncMap() template.FuncMap
	SetConfig(configJSON []byte, opts SetConfigOpts) error
	SetLogger(logger *slog.Logger)
	Close()
}
