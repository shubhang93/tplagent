package coll

import (
	"log/slog"
	"text/template"
)

type Actions struct{}

func (a Actions) FuncMap() template.FuncMap {
	return template.FuncMap{
		"MapGet": func(k string, m map[string]any) any {
			return m[k]
		},
		"SliceGet": func(index int, s []any) any {
			if index < len(s) {
				return s[index]
			}
			return nil
		},
	}
}

func (a Actions) SetConfig(_ []byte) error {
	return nil
}

func (a Actions) SetLogger(_ *slog.Logger) {}

func (a Actions) Close() {}
