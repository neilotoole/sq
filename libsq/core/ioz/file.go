package ioz

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/google/renameio/v2/maybe"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
)

// CopyFile copies the contents from src to dst atomically.
// If dst does not exist, CopyFile creates it with src's perms.
// If the copy fails, CopyFile aborts and dst is preserved.
// If mkdir is true, the directory for dst is created if it
// doesn't exist.
func CopyFile(dst, src string, mkdir bool) error {
	if mkdir {
		if err := RequireDir(filepath.Dir(dst)); err != nil {
			return err
		}
	}

	fi, err := os.Stat(src)
	if err != nil {
		return errz.Err(err)
	}

	in, err := os.Open(src)
	if err != nil {
		return errz.Err(err)
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), "*_"+filepath.Base(src))
	if err != nil {
		return errz.Err(err)
	}
	_, err = io.Copy(tmp, in)
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return errz.Err(err)
	}
	if err = tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return errz.Err(err)
	}
	if err = os.Chmod(tmp.Name(), fi.Mode()); err != nil {
		_ = os.Remove(tmp.Name())
		return errz.Err(err)
	}
	return errz.Err(os.Rename(tmp.Name(), dst))
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

// FileAccessible returns true if path is a file that can be read.
func FileAccessible(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ReadFileToString reads the file at name and returns its contents
// as a string.
func ReadFileToString(name string) (string, error) {
	b, err := os.ReadFile(name)
	if err != nil {
		return "", errz.Err(err)
	}
	return string(b), nil
}

// Filesize returns the size of the file at fp. An error is returned
// if fp doesn't exist or is a directory.
func Filesize(fp string) (int64, error) {
	fi, err := os.Stat(fp)
	if err != nil {
		return 0, errz.Err(err)
	}

	if fi.IsDir() {
		return 0, errz.Errorf("not a file: %s", fp)
	}

	return fi.Size(), nil
}

// FileInfoEqual returns true if a and b are equal.
// The FileInfo.Sys() field is ignored.
func FileInfoEqual(a, b os.FileInfo) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	return a.Name() == b.Name() &&
		a.Size() == b.Size() &&
		a.ModTime().Equal(b.ModTime()) &&
		a.Mode() == b.Mode() &&
		a.IsDir() == b.IsDir()
}

// WriteToFile writes the contents of r to fp. If fp doesn't exist,
// the file is created (including any parent dirs). If fp exists, it is
// truncated. The write operation is context-aware.
func WriteToFile(ctx context.Context, fp string, r io.Reader) (written int64, err error) {
	if err = RequireDir(filepath.Dir(fp)); err != nil {
		return 0, err
	}

	f, err := os.OpenFile(fp, os.O_RDWR|os.O_CREATE|os.O_TRUNC, RWPerms)
	if err != nil {
		return 0, errz.Err(err)
	}

	cr := contextio.NewReader(ctx, r)
	written, err = io.Copy(f, cr)
	closeErr := f.Close()
	if err == nil {
		return written, errz.Err(closeErr)
	}

	return written, errz.Err(err)
}

// WriteFileAtomic writes data to fp atomically, but not on Windows.
func WriteFileAtomic(fp string, data []byte, mode os.FileMode) error {
	return errz.Err(maybe.WriteFile(fp, data, mode))
}
