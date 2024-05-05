package tplactions

import (
	"log/slog"
	"text/template"
)

type Interface interface {
	FuncMap() template.FuncMap
	SetConfig([]byte) error
	ReceiveLogger(logger *slog.Logger)
	Close()
}
