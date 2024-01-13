package source

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/neilotoole/sq/libsq/core/ioz/lockfile"

	"github.com/neilotoole/fscache"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/ioz/download"
	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/progress"
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
	mu          sync.Mutex
	log         *slog.Logger
	cacheDir    string
	tempDir     string
	clnup       *cleanup.Cleanup
	optRegistry *options.Registry

	// cfgLockFn is the lock func for sq's config.
	cfgLockFn lockfile.LockFunc

	// downloads is a map of source handles the download.Download
	// for that source.
	downloads map[string]*download.Download

	// fillerWgs is used to wait for asynchronous filling of the cache
	// to complete (including downloads).
	fillerWgs *sync.WaitGroup

	// fscache is used to cache files, providing convenient access
	// to multiple readers via Files.newReader.
	fscache *fscache.FSCache

	// fscacheEntryMetas contains metadata about fscache entries.
	// Entries are added by Files.addStdin, and consumed by
	// Files.Filesize.
	fscacheEntryMetas map[string]*fscacheEntryMeta

	// detectFns is the set of functions that can detect
	// the type of a file.
	detectFns []DriverDetectFunc
}

// NewFiles returns a new Files instance. If cleanFscache is true, the fscache
// is cleaned on Files.Close.
func NewFiles(ctx context.Context,
	optReg *options.Registry,
	cfgLock lockfile.LockFunc,
	tmpDir, cacheDir string,
	cleanFscache bool,
) (*Files, error) {
	log := lg.FromContext(ctx)
	log.Debug("Creating new Files instance", "tmp_dir", tmpDir, "cache_dir", cacheDir)

	if optReg == nil {
		optReg = &options.Registry{}
	}

	fs := &Files{
		optRegistry:       optReg,
		cacheDir:          cacheDir,
		cfgLockFn:         cfgLock,
		tempDir:           tmpDir,
		clnup:             cleanup.New(),
		log:               lg.FromContext(ctx),
		downloads:         map[string]*download.Download{},
		fillerWgs:         &sync.WaitGroup{},
		fscacheEntryMetas: make(map[string]*fscacheEntryMeta),
	}

	// We want a unique dir for each execution. Note that fcache is deleted
	// on cleanup (unless something bad happens and sq doesn't
	// get a chance to clean up). But, why take the chance; we'll just give
	// fcache a unique dir each time.
	fscacheTmpDir := filepath.Join(
		cacheDir,
		"fscache",
		strconv.Itoa(os.Getpid())+"_"+checksum.Rand(),
	)

	if err := ioz.RequireDir(fscacheTmpDir); err != nil {
		return nil, errz.Err(err)
	}

	if cleanFscache {
		fs.clnup.AddE(func() error {
			return errz.Wrap(os.RemoveAll(fscacheTmpDir), "remove fscache dir")
		})
	}

	var err error
	if fs.fscache, err = fscache.New(fscacheTmpDir, os.ModePerm, time.Hour); err != nil {
		return nil, errz.Err(err)
	}
	fs.clnup.AddE(fs.fscache.Clean)

	fs.clnup.AddE(func() error {
		return errz.Wrap(os.RemoveAll(fs.tempDir), "remove files temp dir")
	})

	fs.clnup.Add(func() { fs.doCacheSweep(ctx) })

	return fs, nil
}

// Filesize returns the file size of src.Location. If the source is being
// loaded asynchronously, this function may block until loading completes.
// An error is returned if src is not a document/file source.
// For remote files, this method should only be invoked after the file
// has completed downloading, or an error will be returned.
func (fs *Files) Filesize(ctx context.Context, src *Source) (size int64, err error) {
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
		fs.mu.Lock()
		defer fs.mu.Unlock()
		var dl *download.Download
		if dl, err = fs.downloadFor(ctx, src); err != nil {
			return 0, err
		}

		return dl.Filesize(ctx)

	case locTypeSQL:
		return 0, errz.Errorf("invalid to get size of SQL source: %s", src.Handle)

	case locTypeStdin:
		fs.mu.Lock()
		entryMeta, ok := fs.fscacheEntryMetas[StdinHandle]
		fs.mu.Unlock()
		if !ok {
			return 0, errz.Errorf("stdin not present in cache")
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-entryMeta.done:
			return entryMeta.written, entryMeta.err
		}

	default:
		return 0, errz.Errorf("unknown source location type: %s", RedactLocation(src.Location))
	}
}

// fscacheEntryMeta contains metadata about a fscache entry.
// When the cache entry has been filled, the done channel
// is closed and the written and err fields are set.
// This mechanism allows Files.Filesize to block until
// the asynchronous filling of the cache entry has completed.
type fscacheEntryMeta struct {
	key     string
	done    chan struct{}
	written int64
	err     error
}

// AddStdin copies f to fs's cache: the stdin data in f
// is later accessible via fs.Open(src) where src.Handle
// is StdinHandle; f's type can be detected via DetectStdinType.
// Note that f is ultimately closed by a goroutine spawned by
// this method, but may not be closed at the time of return.
func (fs *Files) AddStdin(ctx context.Context, f *os.File) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// FIXME: This might be the spot where we can add the cleanup
	// for the stdin cache dir, because it should always be deleted
	// when sq exits. But, first we probably need to refactor the
	// interaction with driver.Grips.

	err := fs.addStdin(ctx, StdinHandle, f) // f is closed by addStdin
	return errz.Wrapf(err, "failed to add %s to fscache", StdinHandle)
}

// addStdin synchronously copies f to fs's cache. f is closed
// when the async copy completes. This method should only be used
// for stdin; for regular files, use Files.addRegularFile.
func (fs *Files) addStdin(ctx context.Context, handle string, f *os.File) error {
	log := lg.FromContext(ctx).With(lga.Handle, handle, lga.File, f.Name())

	if _, ok := fs.fscacheEntryMetas[handle]; ok {
		return errz.Errorf("%s already added to fscache", handle)
	}

	cacheRdr, cacheWrtr, cacheWrtrErrFn, err := fs.fscache.GetWithErr(handle)
	if err != nil {
		return errz.Err(err)
	}

	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, cacheRdr)

	if cacheWrtr == nil {
		// Shouldn't happen
		if cacheRdr != nil {
			return errz.Errorf("no fscache writer for %s (but fscache reader exists when it shouldn't)", handle)
		}

		return errz.Errorf("no fscache writer for %s", handle)
	}

	// We create an entry meta for this handle. This entry will be
	// filled asynchronously in the ioz.CopyAsync callback below.
	// The entry can then be consumed by Files.Filesize.
	entryMeta := &fscacheEntryMeta{
		key:  handle,
		done: make(chan struct{}),
	}
	fs.fscacheEntryMetas[handle] = entryMeta

	fs.fillerWgs.Add(1)
	start := time.Now()
	pw := progress.NewWriter(ctx, "Reading "+handle, -1, cacheWrtr)
	ioz.CopyAsync(pw, contextio.NewReader(ctx, f),
		func(written int64, err error) {
			defer fs.fillerWgs.Done()
			defer lg.WarnIfCloseError(log, lgm.CloseFileReader, f)
			entryMeta.written = written
			entryMeta.err = err
			close(entryMeta.done)

			elapsed := time.Since(start)
			if err == nil {
				log.Info("Async fscache fill: completed", lga.Copied, written, lga.Elapsed, elapsed)
				lg.WarnIfCloseError(log, "Close fscache writer", cacheWrtr)
				pw.Stop()
				return
			}

			log.Error("Async fscache fill: failure", lga.Copied, written, lga.Elapsed, elapsed, lga.Err, err)

			pw.Stop()
			cacheWrtrErrFn(err)
			// We deliberately don't close cacheWrtr here,
			// because cacheWrtrErrFn handles that work.
		},
	)

	log.Debug("Async fscache fill: dispatched")
	return nil
}

// addRegularFile maps f to fs's cache, returning a reader which the
// caller is responsible for closing. f is closed by this method.
// Do not add stdin via this function; instead use addStdin.
func (fs *Files) addRegularFile(ctx context.Context, f *os.File, key string) (fscache.ReadAtCloser, error) {
	log := lg.FromContext(ctx)
	log.Debug("Adding regular file", lga.Key, key, lga.Path, f.Name())

	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, f)

	if key == StdinHandle {
		// This is a programming error; the caller should have
		// instead invoked addStdin. Probably should panic here.
		return nil, errz.New("illegal to add stdin via Files.addRegularFile")
	}

	if fs.fscache.Exists(key) {
		return nil, errz.Errorf("file already exists in cache: %s", key)
	}

	if err := fs.fscache.MapFile(f.Name()); err != nil {
		return nil, errz.Wrapf(err, "failed to map file into fscache: %s", f.Name())
	}

	r, _, err := fs.fscache.Get(key)
	return r, errz.Err(err)
}

// filepath returns the file path of src.Location. An error is returned
// if the source's driver type is not a document type (e.g. it is a
// SQL driver). If src is a remote (http) location, the returned filepath
// is that of the cached download file. If that file is not present, an
// error is returned.
func (fs *Files) filepath(src *Source) (string, error) {
	switch getLocType(src.Location) {
	case locTypeLocalFile:
		return src.Location, nil
	case locTypeRemoteFile:
		dlDir, err := fs.downloadDirFor(src)
		if err != nil {
			return "", err
		}
		dlFile := filepath.Join(dlDir, "body")
		if !ioz.FileAccessible(dlFile) {
			return "", errz.Errorf("remote file for %s not downloaded at: %s", src.Handle, dlFile)
		}
		return dlFile, nil
	case locTypeSQL:
		return "", errz.Errorf("cannot get filepath of SQL source: %s", src.Handle)
	case locTypeStdin:
		return "", errz.Errorf("cannot get filepath of stdin source: %s", src.Handle)
	default:
		return "", errz.Errorf("unknown source location type for %s: %s", src.Handle, RedactLocation(src.Location))
	}
}

// Open returns a new io.ReadCloser for src.Location.
// If src.Handle is StdinHandle, AddStdin must first have
// been invoked. The caller must close the reader.
func (fs *Files) Open(ctx context.Context, src *Source) (io.ReadCloser, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	lg.FromContext(ctx).Debug("Files.Open", lga.Src, src)
	return fs.newReader(ctx, src)
}

// OpenFunc returns a func that invokes fs.Open for src.Location.
func (fs *Files) OpenFunc(src *Source) FileOpenFunc {
	return func(ctx context.Context) (io.ReadCloser, error) {
		return fs.Open(ctx, src)
	}
}

func (fs *Files) newReader(ctx context.Context, src *Source) (io.ReadCloser, error) {
	loc := src.Location
	locTyp := getLocType(loc)
	switch locTyp {
	case locTypeUnknown:
		return nil, errz.Errorf("unknown source location type: %s", loc)
	case locTypeSQL:
		return nil, errz.Errorf("invalid to read SQL source: %s", loc)
	case locTypeStdin:
		r, w, err := fs.fscache.Get(StdinHandle)
		if err != nil {
			return nil, errz.Err(err)
		}
		if w != nil {
			return nil, errz.New("@stdin not cached: has AddStdin been invoked yet?")
		}

		return r, nil
	}

	// Well, it's either a local or remote file.
	// Let's see if it's cached.
	if fs.fscache.Exists(loc) {
		r, _, err := fs.fscache.Get(loc)
		if err != nil {
			return nil, err
		}

		return r, nil
	}

	// It's not cached.
	if locTyp == locTypeLocalFile {
		f, err := os.Open(loc)
		if err != nil {
			return nil, errz.Err(err)
		}
		// fs.addRegularFile closes f, so we don't have to do it.
		r, err := fs.addRegularFile(ctx, f, loc)
		if err != nil {
			return nil, err
		}
		return r, nil
	}

	_, r, err := fs.openRemoteFile(ctx, src, false)
	return r, err
}

// Ping implements a ping mechanism for document
// sources (local or remote files).
func (fs *Files) Ping(ctx context.Context, src *Source) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	switch getLocType(src.Location) {
	case locTypeLocalFile:
		if _, err := os.Stat(src.Location); err != nil {
			return errz.Wrapf(err, "ping: failed to stat file source %s: %s", src.Handle, src.Location)
		}
		return nil

	case locTypeRemoteFile:
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, src.Location, nil)
		if err != nil {
			return errz.Wrapf(err, "ping: %s", src.Handle)
		}
		c := fs.httpClientFor(ctx, src)
		resp, err := c.Do(req) //nolint:bodyclose
		if err != nil {
			return errz.Wrapf(err, "ping: %s", src.Handle)
		}
		defer lg.WarnIfCloseError(fs.log, lgm.CloseHTTPResponseBody, resp.Body)
		if resp.StatusCode != http.StatusOK {
			return errz.Errorf("ping: %s: expected {%s} but got {%s}",
				src.Handle, httpz.StatusText(http.StatusOK), httpz.StatusText(resp.StatusCode))
		}
		return nil

	default:
		// Shouldn't happen
		return errz.Errorf("ping: %s is not a document source", src.Handle)
	}
}

// Close closes any open resources and waits for any goroutines
// to complete.
func (fs *Files) Close() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fs.log.Debug("Files.Close: waiting for goroutines to complete")
	fs.fillerWgs.Wait()

	// TODO: Should delete the tmp dir
	// TODO: Should sweep the cache

	fs.log.Debug("Files.Close: executing clean funcs", lga.Count, fs.clnup.Len())
	return fs.clnup.Run()
}

// CleanupE adds fn to the cleanup sequence invoked by fs.Close.
//
// REVISIT: This CleanupE method really is an odd fish. It's only used
// by the test helper. Probably it can we removed?
func (fs *Files) CleanupE(fn func() error) {
	fs.clnup.AddE(fn)
}

// FileOpenFunc returns a func that opens a ReadCloser. The caller
// is responsible for closing the returned ReadCloser.
type FileOpenFunc func(ctx context.Context) (io.ReadCloser, error)
