// Package file is the file-contents backend for libsq/core/secret.
// A ${file:/path/to/secret} placeholder resolves to the contents of
// that file, with a single trailing newline (LF or CRLF) trimmed —
// the convention used by Docker/Kubernetes secret bind-mounts and by
// systemd LoadCredential.
package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/neilotoole/sq/libsq/core/secret"
)

// Resolver implements secret.Resolver by reading file contents.
type Resolver struct{}

// New returns a Resolver. Callers register the result with a
// secret.Registry under the "file" scheme.
func New() *Resolver {
	return &Resolver{}
}

// Resolve returns the contents of the file at path with a single
// trailing "\n" or "\r\n" trimmed. Returns secret.ErrNotFound when the
// file does not exist. Other read errors are wrapped and returned.
func (r *Resolver) Resolve(_ context.Context, path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", secret.ErrNotFound
		}
		return "", err
	}
	s := string(data)
	s = strings.TrimSuffix(s, "\n")
	s = strings.TrimSuffix(s, "\r")
	return s, nil
}
