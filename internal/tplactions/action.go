package tplactions

import (
	"log/slog"
	"text/template"
)

type Interface interface {
	FuncMap() template.FuncMap
	SetConfig([]byte) error
	SetLogger(logger *slog.Logger)
	Close()
}
