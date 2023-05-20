// Package ioz contains supplemental io functionality.
package ioz

import (
	"bytes"
	"context"
	"io"
	"os"

	"github.com/neilotoole/sq/libsq/core/lg"

	"github.com/goccy/go-yaml"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// Close is a convenience function to close c, logging a warning
// if c.Close returns an error. This is useful in defer, e.g.
//
//	defer ioz.Close(ctx, c)
func Close(ctx context.Context, c io.Closer) {
	if c == nil {
		return
	}

	err := c.Close()
	if ctx == nil {
		return
	}

	log := lg.FromContext(ctx)
	lg.WarnIfError(log, "Close", err)
}

// PrintFile reads file from name and writes it to stdout.
func PrintFile(name string) error {
	return FPrintFile(os.Stdout, name)
}

// FPrintFile reads file from name and writes it to w.
func FPrintFile(w io.Writer, name string) error {
	b, err := os.ReadFile(name)
	if err != nil {
		return errz.Err(err)
	}

	_, err = io.Copy(w, bytes.NewReader(b))
	return errz.Err(err)
}

// marshalYAMLTo is our standard mechanism for encoding YAML.
func marshalYAMLTo(w io.Writer, v any) (err error) {
	// We copy our indent style from kubectl.
	// - 2 spaces
	// - Don't indent sequences.
	const yamlIndent = 2

	enc := yaml.NewEncoder(w,
		yaml.Indent(yamlIndent),
		yaml.IndentSequence(false),
		yaml.UseSingleQuote(false))
	if err = enc.Encode(v); err != nil {
		return errz.Wrap(err, "failed to encode YAML")
	}

	if err = enc.Close(); err != nil {
		return errz.Wrap(err, "close YAML encoder")
	}

	return nil
}

// MarshalYAML is our standard mechanism for encoding YAML.
func MarshalYAML(v any) ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := marshalYAMLTo(buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshallYAML is our standard mechanism for decoding YAML.
func UnmarshallYAML(data []byte, v any) error {
	return errz.Err(yaml.Unmarshal(data, v))
}
