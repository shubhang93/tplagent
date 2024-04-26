package providers

import (
	"testing"
	"text/template"
)

type noopProvider struct{}

func (n noopProvider) FuncMap() template.FuncMap {
	return map[string]any{}
}

func (n noopProvider) SetConfig(bytes []byte) error {
	return nil
}

func TestRegister(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("should have panicked on duplicate registrations")
		}
	}()

	Register("noop", func() Interface {
		return &noopProvider{}
	})

	Register("noop", func() Interface {
		return &noopProvider{}
	})
}
