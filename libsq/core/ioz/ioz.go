// Package ioz contains supplemental io functionality.
package ioz

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	yaml "github.com/goccy/go-yaml"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
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

// ReadDir lists the contents of dir, returning the relative paths
// of the files. If markDirs is true, directories are listed with
// a "/" suffix (including symlinked dirs). If includeDirPath is true,
// the listing is of the form "dir/name". If includeDot is true,
// files beginning with period (dot files) are included. The function
// attempts to continue in the present of errors: the returned paths
// may contain values even in the presence of a returned error (which
// may be a multierr).
func ReadDir(dir string, includeDirPath, markDirs, includeDot bool) (paths []string, err error) {
	fi, err := os.Stat(dir)
	if err != nil {
		return nil, errz.Err(err)
	}

	if !fi.Mode().IsDir() {
		return nil, errz.Errorf("not a dir: %s", dir)
	}

	var entries []os.DirEntry
	if entries, err = os.ReadDir(dir); err != nil {
		return nil, errz.Err(err)
	}

	var name string
	for _, entry := range entries {
		name = entry.Name()
		if strings.HasPrefix(name, ".") && !includeDot {
			// Skip invisible files
			continue
		}

		mode := entry.Type()
		if !mode.IsRegular() && markDirs {
			if entry.IsDir() {
				name += "/"
			} else if mode&os.ModeSymlink != 0 {
				// Follow the symlink to detect if it's a dir
				linked, err2 := filepath.EvalSymlinks(filepath.Join(dir, name))
				if err2 != nil {
					err = errz.Append(err, errz.Err(err2))
					continue
				}

				fi, err2 = os.Stat(linked)
				if err2 != nil {
					err = errz.Append(err, errz.Err(err2))
					continue
				}

				if fi.IsDir() {
					name += "/"
				}
			}
		}

		paths = append(paths, name)
	}

	if includeDirPath {
		for i := range paths {
			// filepath.Join strips the "/" suffix, so we need to preserve it.
			hasSlashSuffix := strings.HasSuffix(paths[i], "/")
			paths[i] = filepath.Join(dir, paths[i])
			if hasSlashSuffix {
				paths[i] += "/"
			}
		}
	}

	return paths, nil
}

// IsPathToRegularFile return true if path is a regular file or
// a symlink that resolves to a regular file. False is returned on
// any error.
func IsPathToRegularFile(path string) bool {
	dest, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}

	fi, err := os.Stat(dest)
	if err != nil {
		return false
	}

	return fi.Mode().IsRegular()
}
