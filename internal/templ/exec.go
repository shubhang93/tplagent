package templ

import (
	"errors"
	"fmt"
	"os"
	"text/template"
)

type Renderer struct {
	Templ   *template.Template
	WriteTo string
}

func (provTmpl *Renderer) Render(staticData any) error {
	_, err := os.Stat(provTmpl.WriteTo)
	if errors.Is(err, os.ErrNotExist) {
		fi, err := os.Create(provTmpl.WriteTo)
		if err != nil {
			return fmt.Errorf("error creating dest file:%w", err)
		}
		return execTempl(provTmpl.Templ, fi, staticData)

	}
	fi, err := os.OpenFile(provTmpl.WriteTo, os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("error opening dest file:%w", err)
	}
	return execTempl(provTmpl.Templ, fi, staticData)
}

func execTempl(t *template.Template, fi *os.File, staticData any) error {
	if err := t.Execute(fi, staticData); err != nil {
		return fmt.Errorf("error writing dest file:%w", err)
	}
	return nil
}
