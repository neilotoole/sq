// Package templatez contains utilities for working with Go templates.
package templatez

import (
	"bytes"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// NewTemplate returns a new text template, with the sprig
// functions already loaded.
func NewTemplate(name, tpl string) (*template.Template, error) {
	t, err := template.New(name).Funcs(sprig.FuncMap()).Parse(tpl)
	if err != nil {
		return nil, errz.Err(err)
	}
	return t, nil
}

// ValidTemplate is a convenience wrapper around NewTemplate. It
// returns an error if the tpl is not a valid text template.
func ValidTemplate(name, tpl string) error {
	_, err := NewTemplate(name, tpl)
	return err
}

// ExecuteTemplate is a convenience function that constructs
// and executes a text template, returning the string value.
func ExecuteTemplate(name, tpl string, data any) (string, error) {
	t, err := NewTemplate(name, tpl)
	if err != nil {
		return "", err
	}

	buf := &bytes.Buffer{}
	if err = t.Execute(buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
