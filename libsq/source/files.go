package source

import (
	"context"
	"github.com/neilotoole/streamcache"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/download"
	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/libsq/core/ioz/lockfile"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
)

// Files is the centralized API for interacting with files.
//
// Why does Files exist? There's a need for functionality to
// transparently get a Reader for remote or local files, and most importantly,
// an ability for multiple goroutines to read/sample a file while
// it's being read (mainly to "sample" the file type, e.g. to determine
// if it's an XLSX file etc.).
//
// Currently we use fscache under the hood
// for this, but our implementation is not satisfactory: in particular,
// the implementation currently requires that we read the entire source
// file into fscache before it's available to be read (which is awful
// if we're reading long-running pipe from stdin). This entire thing
// needs to be revisited. Maybe Files even becomes a fs.FS.
// TODO: ^^ Change docs when switched to streamcache.
type Files struct {
	mu          sync.Mutex
	log         *slog.Logger
	cacheDir    string
	tempDir     string
	clnup       *cleanup.Cleanup
	optRegistry *options.Registry

	// streamCaches is a map of source handles to streamcache.Cache
	// instances: this is used to cache non-regular-file streams, such
	// as stdin, or downloads (via the resp.Body). This streamcache
	// mechanism permits multiple readers to access the stream. For
	// example:
	//
	//  $ cat FILE | sq .data
	//
	// In this scenario FILE is provided on os.Stdin, but sq needs
	// to read the stdin stream several times: first, to detect the type
	// of data on stdin (via Files.DetectStdinType), and then to actually
	// ingest the data.
	streamCaches map[string]*streamcache.Cache

	// cfgLockFn is the lock func for sq's config.
	cfgLockFn lockfile.LockFunc

	// downloads is a map of source handles to the download.Download
	// for that source.
	downloads map[string]*download.Download

	// downloadsWg is used to wait for any download goroutines
	// to complete.
	downloadsWg *sync.WaitGroup

	// downloadedFiles is a map of source handles to filepath of
	// already downloaded files. Consulting this map allows Files
	// to directly serve the downloaded file from disk instead of
	// using download.Download (which typically makes an HTTP
	// call to check the freshness of an already downloaded file).
	downloadedFiles map[string]string

	// fscache is used to cache files, providing convenient access
	// to multiple readers via Files.newReader.
	//fscache *fscache.FSCache

	//fscacheDir string

	// fscacheEntryMetas contains metadata about fscache entries.
	// Entries are added by Files.addStdin, and consumed by
	// Files.Filesize.
	//fscacheEntryMetas map[string]*fscacheEntryMeta

	// detectFns is the set of functions that can detect
	// the type of a file.
	detectFns []DriverDetectFunc
}

// NewFiles returns a new Files instance. If cleanFscache is true, the fscache
// is cleaned on Files.Close.
func NewFiles(
	ctx context.Context,
	optReg *options.Registry,
	cfgLock lockfile.LockFunc,
	tmpDir, cacheDir string,
) (*Files, error) {
	log := lg.FromContext(ctx)
	log.Debug("Creating new Files instance", "tmp_dir", tmpDir, "cache_dir", cacheDir)

	if optReg == nil {
		optReg = &options.Registry{}
	}

	fs := &Files{
		optRegistry: optReg,
		cacheDir:    cacheDir,
		cfgLockFn:   cfgLock,
		tempDir:     tmpDir,
		clnup:       cleanup.New(),
		log:         lg.FromContext(ctx),
		downloads:   map[string]*download.Download{},
		downloadsWg: &sync.WaitGroup{},
		//fscacheEntryMetas: make(map[string]*fscacheEntryMeta),
		streamCaches: make(map[string]*streamcache.Cache),
	}

	// We want a unique dir for each execution. Note that fcache is deleted
	// on cleanup (unless something bad happens and sq doesn't
	// get a chance to clean up). But, why take the chance; we'll just give
	// fcache a unique dir each time.
	//fs.fscacheDir = filepath.Join(cacheDir, "fscache", strconv.Itoa(os.Getpid())+"_"+checksum.Rand())

	//if err := ioz.RequireDir(fs.fscacheDir); err != nil {
	//	return nil, errz.Err(err)
	//}

	//var err error
	//if fs.fscache, err = fscache.New(fs.fscacheDir, os.ModePerm, time.Hour); err != nil {
	//	return nil, errz.Err(err)
	//}

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
		cache, ok := fs.streamCaches[StdinHandle]
		fs.mu.Unlock()
		if !ok {
			// This is a programming error; probably should panic here.
			return 0, errz.Errorf("stdin not present in cache")
		}
		return int64(cache.Size()), nil
		//lg.FromContext(ctx).Warn("Files.Filesize: waiting for stdin cache to be done")
		//select {
		//case <-ctx.Done():
		//	return 0, ctx.Err()
		//case <-cache.Done():
		//	lg.FromContext(ctx).Warn("Files.Filesize: stdin cache is done", lga.Size, cache.Size())
		//	return int64(cache.Size()), nil
		//default:
		//	// This is a programming error; probably should panic here.
		//	return 0, errz.Errorf("stdin cache not sealed?")
		//	//panic("stdin cache is not sealed?")
		//}

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
// is later accessible via fs.NewReader(src) where src.Handle
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

	//err := fs.addStdin(ctx, StdinHandle, f) // f is closed by addStdin

	if _, ok := fs.streamCaches[StdinHandle]; ok {
		return errz.Errorf("%s already added to reader cache", StdinHandle)
	}

	cache := streamcache.New(f)
	fs.streamCaches[StdinHandle] = cache
	lg.FromContext(ctx).With(lga.Handle, StdinHandle, lga.File, f.Name()).
		Debug("Added stdin to reader cache")
	return nil

	//return errz.Wrapf(err, "failed to add %s to fscache", StdinHandle)
}

//// addStdin asynchronously copies f to fs's cache. f is closed
//// when the async copy completes. This method should only be used
//// for stdin; for regular files, use Files.addRegularFile.
//func (fs *Files) addStdin(ctx context.Context, handle string, f *os.File) error {
//	log := lg.FromContext(ctx).With(lga.Handle, StdinHandle, lga.File, f.Name())
//
//	//
//	//cacheRdr, cacheWrtr, cacheWrtrErrFn, err := fs.fscache.GetWithErr(handle)
//	//if err != nil {
//	//	return errz.Err(err)
//	//}
//	//
//	//defer lg.WarnIfCloseError(log, lgm.CloseFileReader, cacheRdr)
//	//
//	//if cacheWrtr == nil {
//	//	// Shouldn't happen
//	//	if cacheRdr != nil {
//	//		return errz.Errorf("no fscache writer for %s (but fscache reader exists when it shouldn't)", handle)
//	//	}
//	//
//	//	return errz.Errorf("no fscache writer for %s", handle)
//	//}
//	//
//	//// We create an entry meta for this handle. This entry will be
//	//// filled asynchronously in the ioz.CopyAsync callback below.
//	//// The entry can then be consumed by Files.Filesize.
//	//entryMeta := &fscacheEntryMeta{
//	//	key:  handle,
//	//	done: make(chan struct{}),
//	//}
//	//fs.fscacheEntryMetas[handle] = entryMeta
//	//
//	//fs.fillerWgs.Add(1)
//	//start := time.Now()
//	//pw := progress.NewWriter(ctx, "Reading "+handle, -1, cacheWrtr)
//	//ioz.CopyAsync(pw, contextio.NewReader(ctx, f),
//	//	func(written int64, err error) {
//	//		defer fs.fillerWgs.Done()
//	//		defer lg.WarnIfCloseError(log, lgm.CloseFileReader, f)
//	//		entryMeta.written = written
//	//		entryMeta.err = err
//	//		close(entryMeta.done)
//	//
//	//		elapsed := time.Since(start)
//	//		if err == nil {
//	//			log.Info("Async fscache fill: completed", lga.Copied, written, lga.Elapsed, elapsed)
//	//			lg.WarnIfCloseError(log, "Close fscache writer", cacheWrtr)
//	//			pw.Stop()
//	//			return
//	//		}
//	//
//	//		log.Error("Async fscache fill: failure", lga.Copied, written, lga.Elapsed, elapsed, lga.Err, err)
//	//
//	//		pw.Stop()
//	//		cacheWrtrErrFn(err)
//	//		// We deliberately don't close cacheWrtr here,
//	//		// because cacheWrtrErrFn handles that work.
//	//	},
//	//)
//	//
//	//log.Debug("Async fscache fill: dispatched")
//	//return nil
//}

// addRegularFile adds f to fs's cache.
// Do not add stdin via this function; instead use addStdin.
// Deprecated: regular files shouldn't be cached.
//func (fs *Files) addRegularFile(ctx context.Context, f *os.File, key string) (*streamcache.Cache, error) {
//	log := lg.FromContext(ctx)
//	log.Debug("Adding regular file to cache", lga.Key, key, lga.Path, f.Name())
//
//	//defer lg.WarnIfCloseError(log, lgm.CloseFileReader, f)
//
//	if key == StdinHandle {
//		// This is a programming error; the caller should have
//		// instead invoked addStdin. Probably should panic here.
//		errz.New("illegal to add stdin via Files.addRegularFile")
//	}
//
//	if _, ok := fs.rdrCaches[key]; ok {
//		return nil, errz.Errorf("file already exists in cache: %s", key)
//	}
//
//	cache := streamcache.New(f)
//	fs.rdrCaches[key] = cache
//	return cache, nil
//}

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

		// FIXME: We shouldn't be depending on knowledge of the internal
		// workings of download.Download here. Instead we should call
		// some method?
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

// NewReader returns a new io.ReadCloser for src.Location.
// Arg ingesting is a performance hint that indicates that
// the reader is being used to ingest data (as opposed to,
// say, sampling the data for type detection). After invoking
// NewReader with ingesting=true for a particular source, it's
// an error to invoke NewReader again for that source.
//
// If src.Handle is StdinHandle, AddStdin must first have
// been invoked. The caller must close the reader.
func (fs *Files) NewReader(ctx context.Context, src *Source, ingesting bool) (io.ReadCloser, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	lg.FromContext(ctx).Debug("Files.NewReader", lga.Src, src)
	return fs.newReader(ctx, src, ingesting)
}

func (fs *Files) hasRdrCache(key string) bool { // FIXME: do we need Files.hasRdrCache?
	_, ok := fs.streamCaches[key]
	return ok
}

// newReader returns a new io.ReadCloser for src.Location. If finalRdr is
// true, and src is using a streamcache.Cache, that cache is sealed
// after the reader is created. That means that newReader should not be
// called again for src in the lifetime of the Files instance.
func (fs *Files) newReader(ctx context.Context, src *Source, finalRdr bool) (io.ReadCloser, error) {
	loc := src.Location
	locTyp := getLocType(loc)
	switch locTyp {
	case locTypeUnknown:
		return nil, errz.Errorf("unknown source location type: %s", loc)
	case locTypeSQL:
		return nil, errz.Errorf("invalid to read SQL source: %s", loc)
	case locTypeLocalFile:
		return errz.Return(os.Open(loc))
	case locTypeStdin:
		cache, ok := fs.streamCaches[StdinHandle]
		if !ok {
			// This is a programming error: AddStdin should have been invoked first.
			// Probably should panic here.
			return nil, errz.New("@stdin not cached: has AddStdin been invoked yet?")
		}
		r := cache.NewReader(ctx)
		if finalRdr {
			cache.Seal()
		}
		return r, nil
	default:
		// It's a remote file.
	}

	// Let's see if it's cached (which happens for downloads).
	if cache, ok := fs.streamCaches[src.Handle]; ok {
		r := cache.NewReader(ctx)
		if finalRdr {
			cache.Seal()
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
	case locTypeStdin:
		// Stdin is always available.
		return nil
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

		// This shouldn't be necessary because the request method was HEAD,
		// so resp.Body should be nil?
		lg.WarnIfCloseError(fs.log, lgm.CloseHTTPResponseBody, resp.Body)

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
	fs.downloadsWg.Wait()

	fs.log.Debug("Files.Close: executing cleanup", lga.Count, fs.clnup.Len())
	err := fs.clnup.Run()
	//err = errz.Append(err, fs.fscache.Clean())
	//err = errz.Append(err, errz.Wrap(os.RemoveAll(fs.fscacheDir), "remove fscache dir"))
	err = errz.Append(err, errz.Wrap(os.RemoveAll(fs.tempDir), "remove files temp dir"))
	fs.doCacheSweep()

	return err
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
// REVISIT: rename to ReaderOpenFunc?
type FileOpenFunc func(ctx context.Context) (io.ReadCloser, error)
