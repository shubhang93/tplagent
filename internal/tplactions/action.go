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
	SetConfig([]byte, SetConfigOpts) error
	SetLogger(logger *slog.Logger)
	Close()
}
