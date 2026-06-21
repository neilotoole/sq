// Package templatez contains utilities for working with Go templates.
package templatez

import (
	"bytes"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// NewTemplate returns a new text template, with the sprig
// functions already loaded.
//
// Only sprig's hermetic (repeatable) functions are loaded. The
// non-hermetic functions (e.g. "env", "now", "randAlphaNum",
// "uuidv4") are excluded: they read the process environment or
// global state, which both leaks information into user-supplied
// templates and makes template output non-deterministic. Excluding
// them keeps a given template's output a pure function of its input.
func NewTemplate(name, tpl string) (*template.Template, error) {
	t, err := template.New(name).Funcs(sprig.HermeticTxtFuncMap()).Parse(tpl)
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
		return "", errz.Err(err)
	}

	return buf.String(), nil
}
