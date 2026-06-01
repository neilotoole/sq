// Package file is the file-contents backend for libsq/core/secret.
// A ${file:/path/to/secret} placeholder resolves to the contents of
// that file, with a single trailing newline (LF or CRLF) trimmed —
// the convention used by Docker/Kubernetes secret bind-mounts and by
// systemd LoadCredential.
//
// Paths must be absolute or start with "~/" (current user's home).
// Relative paths are rejected to avoid CWD-dependent surprises: at
// runtime, "sq" may be invoked from anywhere, so a ${file:secret.txt}
// reference that worked once may silently fail later.
package file

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
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
//
// path may start with "~/" (or be exactly "~") to refer to the current
// user's home directory. Otherwise path must be absolute. Relative
// paths return an error.
func (r *Resolver) Resolve(_ context.Context, path string) (string, error) {
	resolved, err := expandPath(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", secret.ErrNotFound
		}
		return "", errz.Err(err)
	}
	s := string(data)
	// Trim a single trailing LF or CRLF only — never a bare CR.
	switch {
	case strings.HasSuffix(s, "\r\n"):
		s = s[:len(s)-2]
	case strings.HasSuffix(s, "\n"):
		s = s[:len(s)-1]
	}
	return s, nil
}

// expandPath resolves "~" and "~/..." to the user's home directory.
// Other paths must be absolute; relative paths and URI forms (file://)
// are rejected.
func expandPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("empty path")
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", errz.Wrap(err, "expand ~")
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	if strings.HasPrefix(path, "~") {
		// "~user" style is not supported (no stdlib equivalent, and
		// /etc/passwd lookup is platform-specific).
		return "", fmt.Errorf("only ~/ (current user) tilde expansion is supported, got %q", path)
	}
	if strings.HasPrefix(path, "///") {
		// RFC 8089 file:// URI with empty authority: ${file:///etc/passwd}
		// is just sugar for ${file:/etc/passwd}. Strip the leading "//".
		path = path[2:]
	} else if strings.HasPrefix(path, "//") {
		// Two slashes only: either a URI with a non-empty authority
		// (file://host/path — remote, not supported) or an ambiguous
		// non-standard form. Either way, reject with a clear nudge.
		return "", fmt.Errorf(
			"remote file URIs are not supported (got %q); use a local absolute path "+
				"like ${file:/path/to/secret} or the empty-authority URI form ${file:///path/to/secret}",
			path,
		)
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("path must be absolute or start with ~/, got %q", path)
	}
	return path, nil
}
