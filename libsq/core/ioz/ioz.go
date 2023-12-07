// Package ioz contains supplemental io functionality.
package ioz

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"crypto/tls"
	"io"
	mrand "math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/a8m/tree"
	"github.com/a8m/tree/ostree"
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

// CopyAsync asynchronously copies from r to w, invoking
// non-nil callback when done.
func CopyAsync(w io.Writer, r io.Reader, callback func(written int64, err error)) {
	go func() {
		written, err := io.Copy(w, r)
		if callback != nil {
			callback(written, err)
		}
	}()
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

// FileAccessible returns true if path is a file that can be read.
func FileAccessible(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

var _ io.Reader = (*delayReader)(nil)

// DelayReader returns an io.Reader that delays on each read from r.
// This is primarily intended for testing.
// If jitter is true, a randomized jitter factor is added to the delay.
// If r implements io.Closer, the returned reader will also
// implement io.Closer; if r doesn't implement io.Closer,
// the returned reader will not implement io.Closer.
// If r is nil, nil is returned.
func DelayReader(r io.Reader, delay time.Duration, jitter bool) io.Reader {
	if r == nil {
		return nil
	}

	dr := delayReader{r: r, delay: delay, jitter: jitter}
	if _, ok := r.(io.Closer); ok {
		return delayReadCloser{dr}
	}
	return dr
}

var _ io.Reader = (*delayReader)(nil)

type delayReader struct {
	r      io.Reader
	delay  time.Duration
	jitter bool
}

// Read implements io.Reader.
func (d delayReader) Read(p []byte) (n int, err error) {
	delay := d.delay
	if d.jitter {
		delay += time.Duration(mrand.Int63n(int64(d.delay))) / 3 //nolint:gosec
	}

	time.Sleep(delay)
	return d.r.Read(p)
}

var _ io.ReadCloser = (*delayReadCloser)(nil)

type delayReadCloser struct {
	delayReader
}

// Close implements io.Closer.
func (d delayReadCloser) Close() error {
	if c, ok := d.r.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// LimitRandReader returns an io.Reader that reads up to limit bytes
// from crypto/rand.Reader.
func LimitRandReader(limit int64) io.Reader {
	return io.LimitReader(crand.Reader, limit)
}

// NotifyOnceWriter returns an io.Writer that invokes fn on the first
// invocation of Write. If w or fn is nil, w is returned.
func NotifyOnceWriter(w io.Writer, fn func()) io.Writer {
	if w == nil || fn == nil {
		return w
	}

	return &notifyOnceWriter{
		fn:     fn,
		w:      w,
		doneCh: make(chan struct{}),
	}
}

var _ io.Writer = (*notifyOnceWriter)(nil)

type notifyOnceWriter struct {
	w          io.Writer
	fn         func()
	doneCh     chan struct{}
	notifyOnce sync.Once
}

// Write implements [io.Writer]. On the first invocation of this
// method, the notify function is invoked, blocking until it returns.
// Subsequent invocations of Write do trigger the notify function.
func (w *notifyOnceWriter) Write(p []byte) (n int, err error) {
	w.notifyOnce.Do(func() {
		close(w.doneCh)
		w.fn()
	})

	<-w.doneCh
	return w.w.Write(p)
}

// WriteCloser returns w as an io.WriteCloser. If w implements
// io.WriteCloser, w is returned. Otherwise, w is wrapped in a
// no-op decorator that implements io.WriteCloser.
//
// WriteCloser is the missing sibling of io.NopCloser, which
// isn't implemented in stdlib. See: https://github.com/golang/go/issues/22823.
func WriteCloser(w io.Writer) io.WriteCloser {
	if wc, ok := w.(io.WriteCloser); ok {
		return wc
	}
	return toNopWriteCloser(w)
}

func toNopWriteCloser(w io.Writer) io.WriteCloser {
	if _, ok := w.(io.ReaderFrom); ok {
		return nopWriteCloserReaderFrom{w}
	}
	return nopWriteCloser{w}
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

type nopWriteCloserReaderFrom struct {
	io.Writer
}

func (nopWriteCloserReaderFrom) Close() error { return nil }

func (c nopWriteCloserReaderFrom) ReadFrom(r io.Reader) (int64, error) {
	return c.Writer.(io.ReaderFrom).ReadFrom(r)
}

// DirSize returns total size of all regular files in path.
func DirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fi.IsDir() && fi.Mode().IsRegular() {
			size += fi.Size()
		}
		return err
	})
	return size, err
}

// RequireDir ensures that dir exists and is a directory, creating
// it if necessary.
func RequireDir(dir string) error {
	return errz.Err(os.MkdirAll(dir, 0o750))
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

// DirExists returns true if dir exists and is a directory.
func DirExists(dir string) bool {
	fi, err := os.Stat(dir)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

// PrintTree prints the file tree structure at loc to w.
// This function uses the github.com/a8m/tree library, which is
// a Go implementation of the venerable "tree" command.
func PrintTree(w io.Writer, loc string, showSize, colorize bool) error {
	opts := &tree.Options{
		Fs:       new(ostree.FS),
		OutFile:  w,
		All:      true,
		UnitSize: showSize,
		Colorize: colorize,
	}

	inf := tree.New(loc)
	_, _ = inf.Visit(opts)
	inf.Print(opts)
	return nil
}

// NewHTTPClient returns a new HTTP client with the specified timeout.
// A timeout of zero means no timeout. If insecureSkipVerify is true, the
// client will skip TLS verification.
//
// REVISIT: Would it be better to just not set a timeout, and instead
// use context.WithTimeout for each request?
func NewHTTPClient(timeout time.Duration, insecureSkipVerify bool) *http.Client {
	client := *http.DefaultClient

	var tr *http.Transport
	if client.Transport == nil {
		tr = (http.DefaultTransport.(*http.Transport)).Clone()
	} else {
		tr = (client.Transport.(*http.Transport)).Clone()
	}

	if tr.TLSClientConfig == nil {
		tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS10}
	} else {
		tr.TLSClientConfig = tr.TLSClientConfig.Clone()
	}

	tr.TLSClientConfig.InsecureSkipVerify = insecureSkipVerify

	client.Timeout = timeout
	client.Transport = tr

	return &client
}
