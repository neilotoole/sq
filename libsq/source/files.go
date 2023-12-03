package source

import (
	"context"
	"io"
	"log/slog"
	"mime"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/fscache"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/source/drivertype"
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
// needs to be revisited. Maybe Files even becomes a fs.FS.
type Files struct {
	log       *slog.Logger
	mu        sync.Mutex
	clnup     *cleanup.Cleanup
	fcache    *fscache.FSCache
	detectFns []DriverDetectFunc

	// stdinLength is a func that returns number of bytes read from stdin.
	// It is nil if stdin has not been read. The func may block until reading
	// of stdin has completed.
	stdinLength func() int64
}

// NewFiles returns a new Files instance.
func NewFiles(ctx context.Context) (*Files, error) {
	fs := &Files{clnup: cleanup.New(), log: lg.FromContext(ctx)}

	tmpdir, err := os.MkdirTemp("", "sq_files_fscache_*")
	if err != nil {
		return nil, errz.Err(err)
	}

	fcache, err := fscache.New(tmpdir, os.ModePerm, time.Hour)
	if err != nil {
		return nil, errz.Err(err)
	}

	fs.clnup.AddE(fcache.Clean)
	fs.fcache = fcache
	return fs, nil
}

// AddDriverDetectors adds driver type detectors.
func (fs *Files) AddDriverDetectors(detectFns ...DriverDetectFunc) {
	fs.detectFns = append(fs.detectFns, detectFns...)
}

// DetectStdinType detects the type of stdin as previously added
// by AddStdin. An error is returned if AddStdin was not
// first invoked. If the type cannot be detected, TypeNone and
// nil are returned.
func (fs *Files) DetectStdinType(ctx context.Context) (drivertype.Type, error) {
	if !fs.fcache.Exists(StdinHandle) {
		return drivertype.None, errz.New("must invoke Files.AddStdin before invoking DetectStdinType")
	}

	typ, ok, err := fs.detectType(ctx, StdinHandle)
	if err != nil {
		return drivertype.None, err
	}

	if !ok {
		return drivertype.None, nil
	}

	return typ, nil
}

// Size returns the file size of src.Location. If the source is being
// loaded asynchronously, this function may block until loading completes.
func (fs *Files) Size(ctx context.Context, src *Source) (size int64, err error) {
	locTyp := getLocType(src.Location)
	switch locTyp {
	case locTypeLocalFile:
		// It's a filepath
		var fi os.FileInfo
		if fi, err = os.Stat(src.Location); err != nil {
			return 0, errz.Err(err)
		}
		return fi.Size(), nil
	case locTypeRemoteFile:
		// FIXME: implement remote file size.
		return 0, errz.Errorf("remote file size not implemented: %s", src.Location)
	case locTypeSQL:
		return 0, errz.Errorf("cannot get size of SQL source: %s", src.Handle)
	case locTypeStdin:
		// Special handling for stdin.
		if fs.stdinLength == nil {
			return 0, errz.Errorf("stdin not yet added")
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			return fs.stdinLength(), nil
		}
	default:
		return 0, errz.Errorf("unknown source location type: %s", RedactLocation(src.Location))
	}
}

// AddStdin copies f to fs's cache: the stdin data in f
// is later accessible via fs.Open(src) where src.Handle
// is StdinHandle; f's type can be detected via DetectStdinType.
// Note that f is closed by this method.
func (fs *Files) AddStdin(ctx context.Context, f *os.File) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	err := fs.addStdin(ctx, f) // f is closed by addStdin
	return errz.Wrap(err, "failed to read stdin")
}

// addStdin synchronously copies f (stdin) to fs's cache. f is closed
// when the async copy completes. This method should only be used
// for stdin; for regular files, use Files.addFile.
func (fs *Files) addStdin(ctx context.Context, f *os.File) error {
	log := lg.FromContext(ctx).With(lga.File, f.Name())

	if fs.stdinLength != nil {
		return errz.Errorf("stdin already added")
	}

	// Special handling for stdin
	r, w, wErrFn, err := fs.fcache.GetWithErr(StdinHandle)
	if err != nil {
		return errz.Err(err)
	}

	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

	if w == nil {
		// Shouldn't happen
		return errz.Errorf("no cache writer for %s", StdinHandle)
	}

	lw := ioz.NewWrittenWriter(w)
	fs.stdinLength = lw.Written

	df := ioz.DelayReader(f, time.Microsecond*500, true) // FIXME: Delete
	cr := contextio.NewReader(ctx, df)
	pw := progress.NewWriter(ctx, "Reading stdin", -1, lw)

	start := time.Now()
	ioz.CopyAsync(pw, cr, func(written int64, err error) {
		defer lg.WarnIfCloseError(log, lgm.CloseFileReader, f)
		elapsed := time.Since(start)
		if err == nil {
			log.Debug("Async stdin cache fill: completed", lga.Copied, written, lga.Elapsed, elapsed)
			lg.WarnIfCloseError(log, "Close stdin cache", w)
			pw.Stop()
			return
		}

		log.Error("Async stdin cache fill: failure",
			lga.Err, err,
			lga.Copied, written,
			lga.Elapsed, elapsed,
		)
		pw.Stop()
		wErrFn(err)
		// We deliberately don't close "w" here, because wErrFn handles that work.
	})
	log.Debug("Async stdin cache fill: dispatched")
	return nil
}

// addFile maps f to fs's cache, returning a reader which the
// caller is responsible for closing. f is closed by this method.
// Do not add stdin via this function; instead use addStdin.
func (fs *Files) addFile(ctx context.Context, f *os.File, key string) (fscache.ReadAtCloser, error) {
	log := lg.FromContext(ctx)
	log.Debug("Adding file", lga.Key, key, lga.Path, f.Name())

	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, f)

	if key == StdinHandle {
		// This is a programming error; the caller should have
		// instead invoked addStdin. Probably should panic here.
		return nil, errz.New("illegal to add stdin via Files.addFile")
	}

	if fs.fcache.Exists(key) {
		return nil, errz.Errorf("file already exists in cache: %s", key)
	}

	if err := fs.fcache.MapFile(f.Name()); err != nil {
		return nil, errz.Wrapf(err, "failed to map file into fscache: %s", f.Name())
	}

	r, _, err := fs.fcache.Get(key)
	return r, errz.Err(err)
}

// Filepath returns the file path of src.Location.
// An error is returned the source's driver type
// is not a file type (i.e. it is a SQL driver).
func (fs *Files) Filepath(_ context.Context, src *Source) (string, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	loc := src.Location

	if fp, ok := isFpath(loc); ok {
		return fp, nil
	}

	u, ok := httpURL(loc)
	if !ok {
		return "", errz.Errorf("not a valid file location: %s", loc)
	}

	_ = u
	// It's a remote file. We really should download it here.
	// FIXME: implement downloading.
	return "", errz.Errorf("Files.Filepath not implemented for remote files: %s", loc)
}

// Open returns a new io.ReadCloser for src.Location.
// If src.Handle is StdinHandle, AddStdin must first have
// been invoked. The caller must close the reader.
func (fs *Files) Open(ctx context.Context, src *Source) (io.ReadCloser, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	lg.FromContext(ctx).Debug("Files.Open", lga.Src, src)
	return fs.newReader(ctx, src.Location)
}

// OpenFunc returns a func that invokes fs.Open for src.Location.
func (fs *Files) OpenFunc(src *Source) func(ctx context.Context) (io.ReadCloser, error) {
	return func(ctx context.Context) (io.ReadCloser, error) {
		return fs.Open(ctx, src)
	}
}

// ReadAll is a convenience method to read the bytes of a source.
func (fs *Files) ReadAll(ctx context.Context, src *Source) ([]byte, error) {
	r, err := fs.newReader(ctx, src.Location)
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

func (fs *Files) newReader(ctx context.Context, loc string) (io.ReadCloser, error) {
	log := lg.FromContext(ctx).With(lga.Loc, loc)
	log.Debug("Files.newReader", lga.Loc, loc)

	locTyp := getLocType(loc)

	if loc == StdinHandle {
		r, w, err := fs.fcache.Get(StdinHandle)
		log.Debug("Returned from fs.fcache.Get", lga.Err, err)
		if err != nil {
			return nil, errz.Err(err)
		}
		if w != nil {
			return nil, errz.New("@stdin not cached: has AddStdin been invoked yet?")
		}

		return r, nil
	}

	if !fs.fcache.Exists(loc) {
		r, _, err := fs.fcache.Get(loc)
		if err != nil {
			return nil, err
		}

		return r, nil
	}

	// cache miss
	f, err := fs.openLocation(ctx, loc)
	if err != nil {
		return nil, err
	}

	// Note that addFile closes f
	r, err := fs.addFile(ctx, f, loc)
	if err != nil {
		return nil, err
	}
	return r, nil

}

// openLocation returns a file for loc. It is the caller's
// responsibility to close the returned file.
func (fs *Files) openLocation(ctx context.Context, loc string) (*os.File, error) {
	var fpath string
	var ok bool
	var err error

	fpath, ok = isFpath(loc)
	if ok {
		// we have a legitimate fpath
		f, err := os.Open(fpath)
		return f, errz.Err(err)
	}
	// It's not a local file path, maybe it's remote (http)
	var u *url.URL
	u, ok = httpURL(loc)
	if !ok {
		// We're out of luck, it's not a valid file location
		return nil, errz.Errorf("invalid src location: %s", loc)
	}

	// It's a remote file
	fpath, err = fs.fetch(ctx, u.String())
	if err != nil {
		return nil, err
	}

	f, err := os.Open(fpath)
	return f, errz.Err(err)
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
func (fs *Files) fetch(ctx context.Context, loc string) (fpath string, err error) {
	// This impl is a vestigial abomination from an early
	// experiment.

	var ok bool
	if fpath, ok = isFpath(loc); ok {
		// loc is already a local file path
		return fpath, nil
	}

	var u *url.URL
	if u, ok = httpURL(loc); !ok {
		return "", errz.Errorf("not a valid file location: %s", loc)
	}

	var dlFile *os.File
	dlFile, err = os.CreateTemp("", "")
	if err != nil {
		return "", errz.Err(err)
	}

	fetchr := &fetcher.Fetcher{}
	// TOOD: ultimately should be passing a real context here
	err = fetchr.Fetch(ctx, u.String(), dlFile)
	if err != nil {
		return "", errz.Err(err)
	}

	// dlFile is kept open until fs is closed.
	fs.clnup.AddC(dlFile)

	return dlFile.Name(), nil
}

// Close closes any open resources.
func (fs *Files) Close() error {
	fs.log.Debug("Files.Close invoked: executing clean funcs", lga.Count, fs.clnup.Len())

	return fs.clnup.Run()
}

// CleanupE adds fn to the cleanup sequence invoked by fs.Close.
func (fs *Files) CleanupE(fn func() error) {
	fs.clnup.AddE(fn)
}

// DriverType returns the driver type of loc.
func (fs *Files) DriverType(ctx context.Context, loc string) (drivertype.Type, error) {
	log := lg.FromContext(ctx).With(lga.Loc, loc)
	ploc, err := parseLoc(loc)
	if err != nil {
		return drivertype.None, err
	}

	if ploc.typ != drivertype.None {
		return ploc.typ, nil
	}

	if ploc.ext != "" {
		mtype := mime.TypeByExtension(ploc.ext)
		if mtype == "" {
			log.Debug("unknown mime type", lga.Type, mtype)
		} else {
			if typ, ok := typeFromMediaType(mtype); ok {
				return typ, nil
			}
			log.Debug("unknown driver type for media type", lga.Type, mtype)
		}
	}

	// Fall back to the byte detectors
	typ, ok, err := fs.detectType(ctx, loc)
	if err != nil {
		return drivertype.None, err
	}

	if !ok {
		return drivertype.None, errz.Errorf("unable to determine driver type: %s", loc)
	}

	return typ, nil
}

func (fs *Files) detectType(ctx context.Context, loc string) (typ drivertype.Type, ok bool, err error) {
	if len(fs.detectFns) == 0 {
		return drivertype.None, false, nil
	}
	log := lg.FromContext(ctx).With(lga.Loc, loc)
	start := time.Now()

	type result struct {
		typ   drivertype.Type
		score float32
	}

	resultCh := make(chan result, len(fs.detectFns))
	openFn := func(ctx context.Context) (io.ReadCloser, error) {
		fs.mu.Lock()
		defer fs.mu.Unlock()

		return fs.newReader(ctx, loc)
	}

	select {
	case <-ctx.Done():
		return drivertype.None, false, ctx.Err()
	default:
	}

	g, gCtx := errgroup.WithContext(ctx)

	for _, detectFn := range fs.detectFns {
		detectFn := detectFn

		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
			}

			gTyp, gScore, gErr := detectFn(gCtx, openFn)
			if gErr != nil {
				return gErr
			}

			if gScore > 0 {
				resultCh <- result{typ: gTyp, score: gScore}
			}
			return nil
		})
	}

	// REVISIT: We shouldn't have to wait for all goroutines to complete.
	// This logic could be refactored to return as soon as a single
	// goroutine returns a score >= 1.0 (then cancelling the other detector
	// goroutines).

	err = g.Wait()
	if err != nil {
		log.Error(err.Error())
		return drivertype.None, false, errz.Err(err)
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
		log.Debug("Type detected", lga.Type, typ, lga.Elapsed, time.Since(start))
		return typ, true, nil
	}

	log.Warn("No type detected", lga.Type, typ, lga.Elapsed, time.Since(start))
	return drivertype.None, false, nil
}

// FileOpenFunc returns a func that opens a ReadCloser. The caller
// is responsible for closing the returned ReadCloser.
type FileOpenFunc func(ctx context.Context) (io.ReadCloser, error)

// DriverDetectFunc interrogates a byte stream to determine
// the source driver type. A score is returned indicating
// the confidence that the driver type has been detected.
// A score <= 0 is failure, a score >= 1 is success; intermediate
// values indicate some level of confidence.
// An error is returned only if an IO problem occurred.
// The implementation gets access to the byte stream by invoking openFn,
// and is responsible for closing any reader it opens.
type DriverDetectFunc func(ctx context.Context, openFn FileOpenFunc) (
	detected drivertype.Type, score float32, err error)

var _ DriverDetectFunc = DetectMagicNumber

// DetectMagicNumber is a DriverDetectFunc that uses an external
// pkg (h2non/filetype) to detect the "magic number" from
// the start of files.
func DetectMagicNumber(ctx context.Context, openFn FileOpenFunc,
) (detected drivertype.Type, score float32, err error) {
	log := lg.FromContext(ctx)
	var r io.ReadCloser
	r, err = openFn(ctx)
	if err != nil {
		return drivertype.None, 0, errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

	// We only have to pass the file header = first 261 bytes
	head := make([]byte, 261)
	_, err = r.Read(head)
	if err != nil {
		return drivertype.None, 0, errz.Wrapf(err, "failed to read header")
	}

	ftype, err := filetype.Match(head)
	if err != nil {
		if err != nil {
			return drivertype.None, 0, errz.Err(err)
		}
	}

	switch ftype {
	default:
		return drivertype.None, 0, nil
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
