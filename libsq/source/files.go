package source

import (
	"context"
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"github.com/neilotoole/sq/libsq/core/lg"

	"github.com/djherbis/fscache"
	"golang.org/x/exp/slog"

	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source/fetcher"
)

// Files is the centralized API for interacting with files.
//
// Why does Files exist? There's a need for functionality to
// transparently get a Reader for remote or local files, and most importantly,
// an ability for multiple goroutines to read/sample a file while
// it's being read (mainly to "sample" the file type, e.g. to determine
// if it's an XLSX file etc.). Currently we use fscache under the hood
// for this, but our implementation is not satisfactory: in particular,
// the implementation currently requires that we read the entire source
// file into fscache before it's available to be read (which is awful
// if we're reading long-running pipe from stdin). This entire thing
// needs to be revisited.
type Files struct {
	log       *slog.Logger
	mu        sync.Mutex
	clnup     *cleanup.Cleanup
	fcache    *fscache.FSCache
	detectFns []TypeDetectFunc
}

// NewFiles returns a new Files instance.
func NewFiles(log *slog.Logger) (*Files, error) {
	fs := &Files{log: log, clnup: cleanup.New()}

	tmpdir, err := os.MkdirTemp("", "sq_files_fscache_*")
	if err != nil {
		return nil, errz.Err(err)
	}

	fcache, err := fscache.New(tmpdir, os.ModePerm, time.Hour)
	if err != nil {
		return nil, errz.Err(err)
	}

	fs.clnup.AddE(func() error {
		log.Debug("About to clean fscache from dir: %s", tmpdir)
		err = fcache.Clean()
		lg.WarnIfError(log, "", err)

		return err
	})
	fs.fcache = fcache
	return fs, nil
}

// AddTypeDetectors adds type detectors.
func (fs *Files) AddTypeDetectors(detectFns ...TypeDetectFunc) {
	fs.detectFns = append(fs.detectFns, detectFns...)
}

// Size returns the file size of src.Location. This exists
// as a convenience function and something of a replacement
// for using os.Stat to get the file size.
func (fs *Files) Size(src *Source) (size int64, err error) {
	r, err := fs.Open(src)
	if err != nil {
		return 0, err
	}

	defer lg.WarnIfCloseError(fs.log, lgm.CloseFileReader, r)

	size, err = io.Copy(io.Discard, r)
	if err != nil {
		return 0, errz.Err(err)
	}

	return size, nil
}

// AddStdin copies f to fs's cache: the stdin data loc f
// is later accessible via fs.Open(src) where src.Handle
// is StdinHandle; f's type can be detected via TypeStdin.
// Note that f is closed by this method.
//
// DESIGN: it's possible we'll ditch AddStdin and TypeStdin
//
//	loc some future version; this mechanism is a stopgap.
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
	fs.log.Debug("Adding file with key %q: %s", key, f.Name())
	r, w, err := fs.fcache.Get(key)
	if err != nil {
		return nil, errz.Err(err)
	}

	if w == nil {
		lg.WarnIfCloseError(fs.log, lgm.CloseFileReader, r)
		return nil, errz.Errorf("failed to add to fscache (possibly previously added): %s", key)
	}

	// TODO: Problematically, we copy the entire contents of f into fscache.
	// If f is a large file (e.g. piped over stdin), this means that
	// everything is held up until f is fully copied. Hopefully we can
	// do something with fscache so that the readers returned from
	// fscache can lazily read from f.
	copied, err := io.Copy(w, f)
	if err != nil {
		lg.WarnIfCloseError(fs.log, lgm.CloseFileReader, r)
		return nil, errz.Err(err)
	}

	fs.log.Debug("Copied %d bytes to fscache from: %s", copied, key)

	err = errz.Combine(w.Close(), f.Close())
	if err != nil {
		lg.WarnIfCloseError(fs.log, lgm.CloseFileReader, r)
		return nil, err
	}

	return r, nil
}

// Open returns a new io.ReadCloser for src.Location.
// If src.Handle is StdinHandle, AddStdin must first have
// been invoked. The caller must close the reader.
func (fs *Files) Open(src *Source) (io.ReadCloser, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.newReader(src.Location)
}

// OpenFunc returns a func that invokes fs.Open for src.Location.
func (fs *Files) OpenFunc(src *Source) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		return fs.Open(src)
	}
}

// ReadAll is a convenience method to read the bytes of a source.
func (fs *Files) ReadAll(src *Source) ([]byte, error) {
	r, err := fs.newReader(src.Location)
	if err != nil {
		return nil, err
	}

	var data []byte
	data, err = io.ReadAll(r)
	closeErr := r.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}

	return data, nil
}

func (fs *Files) newReader(loc string) (io.ReadCloser, error) {
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
		u, ok = httpURL(loc)
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
	f, err := os.OpenFile(fpath, os.O_RDWR, 0o666)
	if err != nil {
		return nil, errz.Err(err)
	}

	return f, nil
}

// fetch ensures that loc exists locally as a file. This may
// entail downloading the file via HTTPS etc.
func (fs *Files) fetch(loc string) (fpath string, err error) {
	// This impl is a vestigial abomination from an early
	// experiment.

	var ok bool
	if fpath, ok = isFpath(loc); ok {
		// loc is already a local file path
		return fpath, nil
	}

	var u *url.URL
	if u, ok = httpURL(loc); !ok {
		return "", errz.Errorf("not a valid file location: %q", loc)
	}

	var dlFile *os.File
	dlFile, err = os.CreateTemp("", "")
	if err != nil {
		return "", errz.Err(err)
	}

	fetchr := &fetcher.Fetcher{}
	// TOOD: ultimately should be passing a real context here
	err = fetchr.Fetch(context.Background(), u.String(), dlFile)
	if err != nil {
		return "", errz.Err(err)
	}

	// dlFile is kept open until fs is closed.
	fs.clnup.AddC(dlFile)

	return dlFile.Name(), nil
}

// Close closes any open resources.
func (fs *Files) Close() error {
	fs.log.Debug("Files.Close invoked: has %d clean funcs", fs.clnup.Len())

	return fs.clnup.Run()
}

// CleanupE adds fn to the cleanup sequence invoked by fs.Close.
func (fs *Files) CleanupE(fn func() error) {
	fs.clnup.AddE(fn)
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
			fs.log.Debug("unknown mime time for %q for %q", mtype, loc)
		} else {
			if typ, ok := typeFromMediaType(mtype); ok {
				return typ, nil
			}
			fs.log.Debug("unknown source type for media type %q for %q", mtype, loc)
		}
	}

	// Fall back to the byte detectors
	typ, ok, err := fs.detectType(ctx, loc)
	if err != nil {
		return TypeNone, err
	}

	if !ok {
		return TypeNone, errz.Errorf("unable to determine source type: %s", loc)
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
	openFn := func() (io.ReadCloser, error) {
		fs.mu.Lock()
		defer fs.mu.Unlock()

		return fs.newReader(loc)
	}

	select {
	case <-ctx.Done():
		return TypeNone, false, ctx.Err()
	default:
	}

	g, gctx := errgroup.WithContext(ctx)

	for _, detectFn := range fs.detectFns {
		detectFn := detectFn

		g.Go(func() error {
			select {
			case <-gctx.Done():
				return gctx.Err()
			default:
			}

			gTyp, gScore, gErr := detectFn(gctx, fs.log, openFn)
			if gErr != nil {
				return gErr
			}

			if gScore > 0 {
				resultCh <- result{typ: gTyp, score: gScore}
			}
			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		fs.log.Error(err.Error())
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

// FileOpenFunc returns a func that opens a ReadCloser. The caller
// is responsible for closing the returned ReadCloser.
type FileOpenFunc func() (io.ReadCloser, error)

// TypeDetectFunc interrogates a byte stream to determine
// the source driver type. A score is returned indicating
// the confidence that the driver type has been detected.
// A score <= 0 is failure, a score >= 1 is success; intermediate
// values indicate some level of confidence.
// An error is returned only if an IO problem occurred.
// The implementation gets access to the byte stream by invoking openFn,
// and is responsible for closing any reader it opens.
type TypeDetectFunc func(ctx context.Context, log *slog.Logger,
	openFn FileOpenFunc) (detected Type, score float32, err error)

var _ TypeDetectFunc = DetectMagicNumber

// DetectMagicNumber is a TypeDetectFunc that uses an external
// pkg (h2non/filetype) to detect the "magic number" from
// the start of files.
func DetectMagicNumber(_ context.Context, log *slog.Logger, openFn FileOpenFunc,
) (detected Type, score float32, err error) {
	var r io.ReadCloser
	r, err = openFn()
	if err != nil {
		return TypeNone, 0, errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

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

// httpURL tests if s is a well-structured HTTP or HTTPS url, and
// if so, returns the url and true.
func httpURL(s string) (u *url.URL, ok bool) {
	var err error
	u, err = url.Parse(s)
	if err != nil || u.Host == "" || !(u.Scheme == "http" || u.Scheme == "https") {
		return nil, false
	}

	return u, true
}

// TempDirFile creates a new temporary file loc a new temp dir,
// opens the file for reading and writing, and returns the resulting *os.File,
// as well as the parent dir.
// It is the caller's responsibility to close the file and remove the temp
// dir, which the returned cleanFn encapsulates.
func TempDirFile(filename string) (dir string, f *os.File, cleanFn func() error, err error) {
	dir, err = os.MkdirTemp("", "sq_")
	if err != nil {
		return "", nil, nil, errz.Err(err)
	}

	name := filepath.Join(dir, filename)
	f, err = os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
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
