package source

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/neilotoole/sq/libsq/core/ioz/download"
	"github.com/neilotoole/sq/libsq/core/ioz/httpz"

	"github.com/neilotoole/fscache"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/ioz/lockfile"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/stringz"
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
	coll        *Collection

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
func NewFiles(ctx context.Context, coll *Collection, optReg *options.Registry, tmpDir, cacheDir string, cleanFscache bool) (*Files, error) {
	log := lg.FromContext(ctx)
	log.Debug("Creating new Files instance", "tmp_dir", tmpDir, "cache_dir", cacheDir)
	if tmpDir == "" {
		return nil, errz.Errorf("tmpDir is empty")
	}
	if cacheDir == "" {
		return nil, errz.Errorf("cacheDir is empty")
	}

	if optReg == nil {
		optReg = &options.Registry{}
	}

	fs := &Files{
		coll:              coll,
		optRegistry:       optReg,
		cacheDir:          cacheDir,
		fscacheEntryMetas: make(map[string]*fscacheEntryMeta),
		tempDir:           tmpDir,
		clnup:             cleanup.New(),
		log:               lg.FromContext(ctx),
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

	// REVISIT: We could automatically sweep the cache dir on Close?
	// fs.clnup.Add(func() { fs.CacheSweep(ctx) })

	return fs, nil
}

// Filesize returns the file size of src.Location. If the source is being
// loaded asynchronously, this function may block until loading completes.
// An error is returned if src is not a document/file source.
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
		// FIXME: implement remote file size.
		return 0, errz.Errorf("remote file size not implemented: %s", src.Location)

	case locTypeSQL:
		return 0, errz.Errorf("cannot get size of SQL source: %s", src.Handle)

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
// Note that f is closed by this method.
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

	start := time.Now()
	pw := progress.NewWriter(ctx, "Reading "+handle, -1, cacheWrtr)
	ioz.CopyAsync(pw, contextio.NewReader(ctx, f),
		func(written int64, err error) {
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

	if strings.Contains(f.Name(), "cached.db") {
		log.Error("oh no, shouldn't be happening") // FIXME: delete this
	}

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

func (fs *Files) addRemoteFile(ctx context.Context, handle, loc string) (io.ReadCloser, error) {
	dlDir := fs.downloadCacheDirFor(loc)
	if err := ioz.RequireDir(dlDir); err != nil {
		return nil, err
	}

	errCh := make(chan error, 1)
	rdrCh := make(chan io.ReadCloser, 1)

	h := download.Handler{
		Cached: func(fp string) {
			if err := fs.fscache.MapFile(fp); err != nil {
				errCh <- errz.Wrapf(err, "failed to map file into fscache: %s", fp)
				return
			}

			r, _, err := fs.fscache.Get(fp)
			if err != nil {
				errCh <- errz.Err(err)
				return
			}
			rdrCh <- r
		},
		Uncached: func() (dest ioz.WriteErrorCloser) {
			r, w, err := fs.fscache.Get(loc)
			if err != nil {
				errCh <- errz.Err(err)
				return nil
			}

			wec := ioz.NewFuncWriteErrorCloser(w, func(err error) {
				log := lg.FromContext(ctx)
				lg.WarnIfError(log, "Remove damaged cache entry", fs.fscache.Remove(loc))
			})

			rdrCh <- r
			return wec
		},
		Error: func(err error) {
			errCh <- err
		},
	}

	c := httpz.NewDefaultClient()
	dl, err := download.New(handle, c, loc, dlDir)
	if err != nil {
		return nil, err
	}

	go dl.Get(ctx, h)

	select {
	case <-ctx.Done():
		return nil, errz.Err(ctx.Err())
	case err = <-errCh:
		return nil, err
	case rdr := <-rdrCh:
		return rdr, nil
	}
}

// Filepath returns the file path of src.Location.
// An error is returned the source's driver type
// is not a file type (i.e. it is a SQL driver).
// FIXME: Implement Files.Filepath fully.
func (fs *Files) Filepath(_ context.Context, src *Source) (string, error) {
	// fs.mu.Lock()
	// defer fs.mu.Unlock()

	switch getLocType(src.Location) {
	case locTypeLocalFile:
		return src.Location, nil
	case locTypeRemoteFile:
		// FIXME: implement remote file location.
		// It's a remote file. We really should download it here.
		// FIXME: implement downloading.
		return "", errz.Errorf("not implemented for remote source: %s", src.Handle)
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
	return fs.newReader(ctx, src.Handle, src.Location)
}

// CacheLockFor returns the lock file for src's cache.
func (fs *Files) CacheLockFor(src *Source) (lockfile.Lockfile, error) {
	cacheDir, err := fs.CacheDirFor(src)
	if err != nil {
		return "", errz.Wrapf(err, "cache lock for %s", src.Handle)
	}

	lf, err := lockfile.New(filepath.Join(cacheDir, "pid.lock"))
	if err != nil {
		return "", errz.Wrapf(err, "cache lock for %s", src.Handle)
	}

	return lf, nil
}

// OpenFunc returns a func that invokes fs.Open for src.Location.
func (fs *Files) OpenFunc(src *Source) FileOpenFunc {
	return func(ctx context.Context) (io.ReadCloser, error) {
		return fs.Open(ctx, src)
	}
}

func (fs *Files) newReader(ctx context.Context, handle, loc string) (io.ReadCloser, error) {
	// log := lg.FromContext(ctx).With(lga.Loc, loc)

	locTyp := getLocType(loc)
	switch locTyp {
	case locTypeUnknown:
		return nil, errz.Errorf("unknown source location type: %s", loc)
	case locTypeSQL:
		return nil, errz.Errorf("cannot read SQL source: %s", loc)
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

	//if loc == StdinHandle {
	//	r, w, err := fs.fscache.Get(StdinHandle)
	//	log.Debug("Returned from fs.fcache.Get", lga.Err, err)
	//	if err != nil {
	//		return nil, errz.Err(err)
	//	}
	//	if w != nil {
	//		return nil, errz.New("@stdin not cached: has AddStdin been invoked yet?")
	//	}
	//
	//	return r, nil
	//}

	//// It's an uncached remote file.
	//if !fs.fscache.Exists(loc) {
	//	r, _, err := fs.fscache.Get(loc)
	//	if err != nil {
	//		return nil, err
	//	}
	//
	//	return r, nil
	//}
	//
	//// cache miss
	//f, err := fs.openLocation(ctx, loc)
	//if err != nil {
	//	return nil, err
	//}
	//
	//// Note that addRegularFile closes f
	//r, err := fs.addRegularFile(ctx, f, loc)
	//if err != nil {
	//	return nil, err
	//}
	//return r, nil

	return fs.addRemoteFile(ctx, handle, loc)
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
		return errz.Return(os.Open(fpath))
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

// Close closes any open resources.
func (fs *Files) Close() error {
	fs.log.Debug("Files.Close invoked: executing clean funcs", lga.Count, fs.clnup.Len())
	return fs.clnup.Run()
}

// CleanupE adds fn to the cleanup sequence invoked by fs.Close.
func (fs *Files) CleanupE(fn func() error) {
	fs.clnup.AddE(fn)
}

// CacheClear clears the cache dir. This wipes the entire contents
// of the cache dir, so it should be used with caution. Note that
// this operation is distinct from [Files.CacheSweep].
func (fs *Files) CacheClear(ctx context.Context) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	log := lg.FromContext(ctx).With(lga.Dir, fs.cacheDir)
	log.Debug("Clearing cache dir")
	if !ioz.DirExists(fs.cacheDir) {
		log.Debug("Cache dir does not exist")
		return nil
	}

	// Instead of directly deleting the existing cache dir, we first
	// move it to /tmp, and then try to delete it. This should probably
	// help with the situation where another sq instance has an open pid
	// lock in the cache dir.

	tmpDir := DefaultTempDir()
	if err := ioz.RequireDir(tmpDir); err != nil {
		return errz.Wrap(err, "cache clear")
	}
	relocateDir := filepath.Join(tmpDir, "dead_cache_"+stringz.Uniq8())
	if err := os.Rename(fs.cacheDir, relocateDir); err != nil {
		return errz.Wrap(err, "cache clear: relocate")
	}

	if err := os.RemoveAll(relocateDir); err != nil {
		log.Warn("Could not delete relocated cache dir", lga.Path, relocateDir, lga.Err, err)
	}

	// Recreate the cache dir.
	if err := ioz.RequireDir(fs.cacheDir); err != nil {
		return errz.Wrap(err, "cache clear")
	}

	return nil
}

// CacheSweep sweeps the cache dir, making a best-effort attempt
// to remove any empty directories. Note that this operation is
// distinct from [Files.CacheClear].
func (fs *Files) CacheSweep(ctx context.Context) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	dir := fs.cacheDir
	log := lg.FromContext(ctx).With(lga.Dir, dir)
	log.Debug("Sweeping cache dir")
	var count int
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err != nil {
			log.Warn("Problem sweeping cache dir", lga.Path, path, lga.Err, err)
			return nil
		}

		if !info.IsDir() {
			return nil
		}

		files, err := os.ReadDir(path)
		if err != nil {
			log.Warn("Problem reading dir", lga.Dir, path, lga.Err, err)
			return nil
		}

		if len(files) != 0 {
			return nil
		}

		err = os.Remove(path)
		if err != nil {
			log.Warn("Problem removing empty dir", lga.Dir, path, lga.Err, err)
		}
		count++

		return nil
	})
	if err != nil {
		log.Warn("Problem sweeping cache dir", lga.Dir, dir, lga.Err, err)
	}
	log.Info("Swept cache dir", lga.Dir, dir, lga.Count, count)
}

// FileOpenFunc returns a func that opens a ReadCloser. The caller
// is responsible for closing the returned ReadCloser.
type FileOpenFunc func(ctx context.Context) (io.ReadCloser, error)

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
