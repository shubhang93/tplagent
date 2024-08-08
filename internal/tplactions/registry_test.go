package tplactions

import (
	"log/slog"
	"testing"
	"text/template"
)

type noopActions struct{}

func (n noopActions) FuncMap() template.FuncMap {
	return map[string]any{}
}

func (n noopActions) SetConfig(configJSON []byte, env Env) error {
	return nil
}

func (n noopActions) Close() {}

func (n noopActions) SetLogger(_ *slog.Logger) {}

func TestRegister(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("should have panicked on duplicate registrations")
		}
	}()

	Register("noop", func() Interface {
		return &noopActions{}
	})
	Register("noop", func() Interface {
		return &noopActions{}
	})
}
