package source

import (
	"context"
	"io"
	"io/ioutil"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/djherbis/fscache"

	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/neilotoole/lg"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/cleanup"
	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/source/fetcher"
)

// Files is the centralized API for interacting with files.
type Files struct {
	// Note: It is expected that Files will in future have more
	//  capabilities, such as caching (on the filesystem) files that
	//  are fetched from URLs.

	log       lg.Log
	mu        sync.Mutex
	clnup     *cleanup.Cleanup
	fcache    *fscache.FSCache
	detectFns []TypeDetectorFunc
}

// NewFiles returns a new Files instance.
func NewFiles(log lg.Log) (*Files, error) {
	fs := &Files{log: log, clnup: cleanup.New()}

	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, errz.Err(err)
	}

	fs.clnup.AddE(func() error {
		return errz.Err(os.RemoveAll(tmpdir))
	})

	fcache, err := fscache.New(tmpdir, os.ModePerm, time.Hour)
	if err != nil {
		log.WarnIfFuncError(fs.clnup.Run)
		return nil, errz.Err(err)
	}

	fs.clnup.AddE(fcache.Clean)
	fs.fcache = fcache
	return fs, nil
}

// AddTypeDetectors adds type detectors.
func (fs *Files) AddTypeDetectors(detectFns ...TypeDetectorFunc) {
	fs.detectFns = append(fs.detectFns, detectFns...)
}

// Size returns the file size of src.Location. This exists
// as a convenience function and something of a replacement
// for using os.Stat to get the file size.
func (fs *Files) Size(src *Source) (size int64, err error) {
	r, err := fs.NewReader(context.Background(), src)
	if err != nil {
		return 0, err
	}

	defer fs.log.WarnIfCloseError(r)

	size, err = io.Copy(ioutil.Discard, r)
	if err != nil {
		return 0, errz.Err(err)
	}

	return size, nil
}

// AddStdin copies f to fs's cache: the stdin data in f
// is later accessible via fs.NewReader(src) where src.Handle
// is StdinHandle; f's type can be detected via TypeStdin.
// Note that f is closed by this method.
//
// DESIGN: it's possible we'll ditch AddStdin and TypeStdin
//  in some future version; this mechanism is a stopgap.
func (fs *Files) AddStdin(f *os.File) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// We don't need r, but we're responsible for closing it.
	r, err := fs.addFile(f, StdinHandle) // f is closed by addFile
	if err != nil {
		return err
	}

	return r.Close()
}

// TypeStdin detects the type of stdin as previously added
// by AddStdin. An error is returned if AddStdin was not
// first invoked. If the type cannot be detected, TypeNone and
// nil are returned.
func (fs *Files) TypeStdin(ctx context.Context) (Type, error) {
	if !fs.fcache.Exists(StdinHandle) {
		return TypeNone, errz.New("must invoke AddStdin before invoking TypeStdin")
	}

	typ, ok, err := fs.detectType(ctx, StdinHandle)
	if err != nil {
		return TypeNone, err
	}

	if !ok {
		return TypeNone, nil
	}

	return typ, nil
}

// add file copies f to fs's cache, returning a reader which the
// caller is responsible for closing. f is closed by this method.
func (fs *Files) addFile(f *os.File, key string) (fscache.ReadAtCloser, error) {
	r, w, err := fs.fcache.Get(key)
	if err != nil {
		return nil, errz.Err(err)
	}

	if w == nil {
		fs.log.WarnIfCloseError(r)
		return nil, errz.Errorf("failed to add to fscache (possibly previously added): %s", key)
	}

	copied, err := io.Copy(w, f)
	if err != nil {
		fs.log.WarnIfCloseError(r)
		return nil, errz.Err(err)
	}

	fs.log.Debugf("Copied %d bytes to fscache from: %s", copied, key)

	err = errz.Combine(w.Close(), f.Close())
	if err != nil {
		fs.log.WarnIfCloseError(r)
		return nil, err
	}

	return r, nil
}

// NewReader returns a new ReadCloser for src.Location.
// If src.Handle is StdinHandle, AddStdin must first have
// been invoked. The caller must close the reader.
func (fs *Files) NewReader(ctx context.Context, src *Source) (io.ReadCloser, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}

	return fs.newReader(ctx, src.Location)
}

// ReadAll is a convenience method to read the bytes of a source.
func (fs *Files) ReadAll(src *Source) ([]byte, error) {
	r, err := fs.newReader(context.Background(), src.Location)
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(r)
	closeErr := r.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}

	return b, nil
}

func (fs *Files) newReader(ctx context.Context, loc string) (io.ReadCloser, error) {
	if loc == StdinHandle {
		r, w, err := fs.fcache.Get(StdinHandle)
		if err != nil {
			return nil, errz.Err(err)
		}
		if w != nil {
			return nil, errz.New("@stdin not cached: has AddStdin been invoked yet?")
		}

		return r, nil
	}

	if !fs.fcache.Exists(loc) {
		// cache miss
		f, err := fs.openLocation(loc)
		if err != nil {
			fs.log.WarnIfCloseError(f)
			return nil, err
		}

		// Note that addFile closes f
		r, err := fs.addFile(f, loc)
		if err != nil {
			return nil, err
		}
		return r, nil
	}

	r, _, err := fs.fcache.Get(loc)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// openLocation returns a file for loc. It is the caller's
// responsibility to close the returned file.
func (fs *Files) openLocation(loc string) (*os.File, error) {
	var fpath string
	var ok bool
	var err error

	fpath, ok = isFpath(loc)
	if !ok {
		// It's not a local file path, maybe it's remote (http)
		var u *url.URL
		u, ok = HTTPURL(loc)
		if !ok {
			// We're out of luck, it's not a valid file location
			return nil, errz.Errorf("invalid src location: %s", loc)
		}

		// It's a remote file
		fpath, err = fs.fetch(u.String())
		if err != nil {
			return nil, err
		}
	}

	// we have a legitimate fpath
	return fs.openFile(fpath)
}

// openFile opens the file at fpath. It is the caller's
// responsibility to close the returned file.
func (fs *Files) openFile(fpath string) (*os.File, error) {
	f, err := os.OpenFile(fpath, os.O_RDWR, 0666)
	if err != nil {
		return nil, errz.Err(err)
	}

	return f, nil
}

// fetch ensures that loc exists locally as a file. This may
// entail downloading the file via HTTPS etc.
func (fs *Files) fetch(loc string) (fpath string, err error) {
	// This impl is a vestigial abomination from an early
	// experiment. In particular, the FetchFile function
	// can be completely simplified.

	var ok bool
	if fpath, ok = isFpath(loc); ok {
		// loc is already a local file path
		return fpath, nil
	}

	var u *url.URL

	if u, ok = HTTPURL(loc); !ok {
		return "", errz.Errorf("not a valid file location: %q", loc)
	}

	f, mediatype, cleanFn, err := fetcher.FetchFile(fs.log, u.String())
	fs.clnup.AddC(f) // f is kept open until fs.Close is called.
	fs.clnup.AddE(cleanFn)
	if err != nil {
		return "", err
	}

	fs.log.Debugf("fetched %q: mediatype=%s", loc, mediatype)
	return f.Name(), nil
}

// Close closes any open files.
func (fs *Files) Close() error {
	return fs.clnup.Run()
}

// Type returns the source type of location.
func (fs *Files) Type(ctx context.Context, loc string) (Type, error) {
	ploc, err := parseLoc(loc)
	if err != nil {
		return TypeNone, err
	}

	if ploc.typ != TypeNone {
		return ploc.typ, nil
	}

	if ploc.ext != "" {
		mtype := mime.TypeByExtension(ploc.ext)
		if mtype == "" {
			fs.log.Debugf("unknown mime time for %q for %q", mtype, loc)
		} else {
			if typ, ok := typeFromMediaType(mtype); ok {
				return typ, nil
			}
			fs.log.Debugf("unknown source type for media type %q for %q", mtype, loc)
		}
	}

	// Fall back to the byte detectors
	typ, ok, err := fs.detectType(ctx, loc)
	if err != nil {
		return TypeNone, err
	}

	if !ok {
		return TypeNone, errz.Errorf("unable to determine type of %q", loc)
	}

	return typ, nil
}

func (fs *Files) detectType(ctx context.Context, loc string) (typ Type, ok bool, err error) {
	if len(fs.detectFns) == 0 {
		return TypeNone, false, nil
	}

	type result struct {
		typ   Type
		score float32
	}

	resultCh := make(chan result, len(fs.detectFns))
	readers := make([]io.ReadCloser, len(fs.detectFns))

	fs.mu.Lock()
	for i := 0; i < len(readers); i++ {
		readers[i], err = fs.newReader(ctx, loc)
		if err != nil {
			fs.mu.Unlock()
			return TypeNone, false, nil
		}
	}
	fs.mu.Unlock()

	g, gctx := errgroup.WithContext(ctx)

	for i, detectFn := range fs.detectFns {
		i, detectFn := i, detectFn

		g.Go(func() error {
			select {
			case <-gctx.Done():
				return gctx.Err()
			default:
			}

			r := readers[i]
			defer fs.log.WarnIfCloseError(r)

			typ, score, err := detectFn(gctx, r)
			if err != nil {
				return err
			}

			if score > 0 {
				resultCh <- result{typ: typ, score: score}
			}
			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		return TypeNone, false, errz.Err(err)
	}
	close(resultCh)

	var highestScore float32
	for res := range resultCh {
		if res.score > highestScore {
			highestScore = res.score
			typ = res.typ
		}
	}

	const detectScoreThreshold = 0.5
	if highestScore >= detectScoreThreshold {
		return typ, true, nil
	}

	return TypeNone, false, nil
}

// DetectMagicNumber is a TypeDetectorFunc that uses an external
// pkg (h2non/filetype) to detect the "magic number" from
// the start of files.
func DetectMagicNumber(ctx context.Context, r io.Reader) (detected Type, score float32, err error) {
	// We only have to pass the file header = first 261 bytes
	head := make([]byte, 261)
	_, err = r.Read(head)
	if err != nil {
		return TypeNone, 0, errz.Wrapf(err, "failed to read header")
	}

	ftype, err := filetype.Match(head)
	if err != nil {
		if err != nil {
			return TypeNone, 0, errz.Err(err)
		}
	}

	switch ftype {
	default:
		return TypeNone, 0, nil
	case matchers.TypeXlsx:
		// This doesn't seem to work, because .xlsx files are
		// zipped, so the type returns as a zip. Perhaps there's
		// something we can do about it, such as first extracting
		// the zip, and then reading the inner magic number, but
		// the xlsx.DetectXLSX func should catch the type anyway.
		return typeXLSX, 1.0, nil
	case matchers.TypeXls:
		// TODO: our xlsx driver doesn't yet support XLS
		return typeXLSX, 1.0, errz.Errorf("Microsoft XLS (%s) not currently supported", ftype)
	case matchers.TypeSqlite:
		return typeSL3, 1.0, nil
	}
}

// AbsLocation returns the absolute path of loc. That is, relative
// paths etc in loc are resolved. If loc is not a file path or
// it cannot be processed, loc is returned unmodified.
func AbsLocation(loc string) string {
	if fpath, ok := isFpath(loc); ok {
		return fpath
	}

	return loc
}

func isFpath(loc string) (fpath string, ok bool) {
	if strings.ContainsRune(loc, ':') {
		return "", false
	}

	fpath, err := filepath.Abs(loc)
	if err != nil {
		return "", false
	}

	return fpath, true
}

// HTTPURL tests if s is a well-structured HTTP or HTTPS url, and
// if so, returns the url and true.
func HTTPURL(s string) (u *url.URL, ok bool) {
	var err error
	u, err = url.Parse(s)
	if err != nil || u.Host == "" || !(u.Scheme == "http" || u.Scheme == "https") {
		return nil, false
	}

	return u, true
}

// TempDirFile creates a new temporary file in a new temp dir,
// opens the file for reading and writing, and returns the resulting *os.File,
// as well as the parent dir.
// It is the caller's responsibility to close the file and remove the temp
// dir, which the returned cleanFn encapsulates.
func TempDirFile(filename string) (dir string, f *os.File, cleanFn func() error, err error) {
	dir, err = ioutil.TempDir("", "sq_")
	if err != nil {
		return "", nil, nil, errz.Err(err)
	}

	name := filepath.Join(dir, filename)
	f, err = os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		// Silently delete the temp dir
		_ = os.RemoveAll(dir)

		return "", nil, nil, errz.Err(err)
	}

	cleanFn = func() error {
		return errz.Append(f.Close(), os.RemoveAll(dir))
	}

	return dir, f, cleanFn, nil
}
