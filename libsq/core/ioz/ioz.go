// Package ioz contains supplemental io functionality.
package ioz

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"fmt"
	"io"
	mrand "math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/a8m/tree"
	"github.com/a8m/tree/ostree"
	yaml "github.com/goccy/go-yaml"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// RWPerms is the default file mode used for creating files.
const RWPerms = os.FileMode(0o600)

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
	tmp, err := os.CreateTemp(filepath.Dir(dst), "")
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

// RenameDir is like os.Rename, but it works even if newpath
// already exists and is a directory (which os.Rename fails on).
func RenameDir(oldDir, newpath string) error {
	oldFi, err := os.Stat(oldDir)
	if err != nil {
		return errz.Err(err)
	}

	if !oldFi.IsDir() {
		return errz.Errorf("rename dir: not a dir: %s", oldDir)
	}

	if newFi, _ := os.Stat(newpath); newFi != nil && newFi.IsDir() {
		// os.Rename will fail when newpath is a directory.
		// So, we first move (rename) newpath to a temp staging path,
		// and then we move oldpath to newpath, and finally we remove
		// the staging dir. We do this because if there's open files
		// in newpath, os.RemoveAll may fail, so this technique is
		// more likely to succeed? Not completely sure about that.
		staging := filepath.Join(os.TempDir(), stringz.Uniq8()+"_"+filepath.Base(newpath))

		if err = os.Rename(newpath, staging); err != nil {
			return errz.Err(err)
		}

		if err = os.Rename(oldDir, newpath); err != nil {
			return errz.Err(err)
		}

		// If the staging deletion (i.e. the old dir) fails,
		// do we even care? It'll be left hanging around in
		// the tmp dir, which I guess could be a security
		// issue in some circumstances?
		_ = os.RemoveAll(staging)
		return nil
	}

	return errz.Err(os.Rename(oldDir, newpath))
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
	doneCh     chan struct{} // REVISIT: Do we need doneCh?
	notifyOnce sync.Once
}

// Write implements io.Writer. On the first invocation of this
// method, the notify function is invoked, blocking until it returns.
// Subsequent invocations of Write don't trigger the notify function.
func (w *notifyOnceWriter) Write(p []byte) (n int, err error) {
	w.notifyOnce.Do(func() {
		w.fn()
		close(w.doneCh)
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
	return errz.Err(os.MkdirAll(dir, 0o700))
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

// Drain drains r.
func Drain(r io.Reader) error {
	_, err := io.Copy(io.Discard, r)
	return err
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

// ReadCloserNotifier returns a new io.ReadCloser that invokes fn
// after Close is called, passing along any error from Close.
// If rc or fn is nil, rc is returned. Note that any subsequent
// calls to Close are no-op, and return the same error (if any)
// as the first invocation of Close.
func ReadCloserNotifier(rc io.ReadCloser, fn func(closeErr error)) io.ReadCloser {
	if rc == nil || fn == nil {
		return rc
	}
	return &readCloserNotifier{ReadCloser: rc, fn: fn}
}

type readCloserNotifier struct {
	closeErr error
	io.ReadCloser
	fn   func(error)
	once sync.Once
}

func (c *readCloserNotifier) Close() error {
	c.once.Do(func() {
		c.closeErr = c.ReadCloser.Close()
		c.fn(c.closeErr)
	})
	return c.closeErr
}

var _ io.Reader = (*errorAfterNReader)(nil)

// NewErrorAfterNReader returns an io.Reader that returns err after
// reading n random bytes from crypto/rand.Reader.
func NewErrorAfterNReader(n int, err error) io.Reader {
	return &errorAfterNReader{afterN: n, err: err}
}

type errorAfterNReader struct {
	err    error
	afterN int
	count  int
	mu     sync.Mutex
}

func (r *errorAfterNReader) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count >= r.afterN {
		return 0, r.err
	}

	// There's some bytes to read
	allowed := r.afterN - r.count
	if allowed > len(p) {
		n, _ = crand.Read(p)
		r.count += n
		return n, nil
	}
	n, _ = crand.Read(p[:allowed])
	if n != allowed {
		panic(fmt.Sprintf("expected to read %d bytes, got %d", allowed, n))
	}
	r.count += n
	return n, r.err
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

// WriteErrorCloser supplements io.WriteCloser with an Error method, indicating
// to the io.WriteCloser that an upstream error has interrupted the writing
// operation. Note that clients should invoke only one of Close or Error.
type WriteErrorCloser interface {
	io.WriteCloser

	// Error indicates that an upstream error has interrupted the
	// writing operation.
	Error(err error)
}

type writeErrorCloser struct {
	fn func(error)
	io.WriteCloser
}

// Error implements WriteErrorCloser.Error.
func (w *writeErrorCloser) Error(err error) {
	if w.fn != nil {
		w.fn(err)
	}
}

// NewFuncWriteErrorCloser returns a new WriteErrorCloser that wraps w, and
// invokes non-nil fn when WriteErrorCloser.Error is called.
func NewFuncWriteErrorCloser(w io.WriteCloser, fn func(error)) WriteErrorCloser {
	return &writeErrorCloser{WriteCloser: w, fn: fn}
}

// PruneEmptyDirTree prunes empty dirs, and dirs that contain only
// other empty dirs, from the directory tree rooted at dir. If a dir
// contains at least one non-dir entry, that dir is spared. Arg dir
// must be an absolute path.
func PruneEmptyDirTree(ctx context.Context, dir string) (count int, err error) {
	return doPruneEmptyDirTree(ctx, dir, true)
}

func doPruneEmptyDirTree(ctx context.Context, dir string, isRoot bool) (count int, err error) {
	if !filepath.IsAbs(dir) {
		return 0, errz.Errorf("dir must be absolute: %s", dir)
	}

	select {
	case <-ctx.Done():
		return 0, errz.Err(ctx.Err())
	default:
	}

	var entries []os.DirEntry
	if entries, err = os.ReadDir(dir); err != nil {
		return 0, errz.Err(err)
	}

	if len(entries) == 0 {
		if isRoot {
			return 0, nil
		}
		err = os.RemoveAll(dir)
		if err != nil {
			return 0, errz.Err(err)
		}
		return 1, nil
	}

	// We've got some entries... let's check what they are.
	if countNonDirs(entries) != 0 {
		// There are some non-dir entries, so this dir doesn't get deleted.
		return 0, nil
	}

	// Each of the entries is a dir. Recursively prune.
	var n int
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return count, errz.Err(ctx.Err())
		default:
		}

		n, err = doPruneEmptyDirTree(ctx, filepath.Join(dir, entry.Name()), false)
		count += n
		if err != nil {
			return count, err
		}
	}

	return count, nil
}

func countNonDirs(entries []os.DirEntry) (count int) {
	for _, entry := range entries {
		if !entry.IsDir() {
			count++
		}
	}
	return count
}
