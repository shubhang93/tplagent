package tplactions

import "text/template"

type Interface interface {
	FuncMap() template.FuncMap
	SetConfig([]byte) error
	Close()
}
