package source

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neilotoole/sq/libsq/core/progress"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/ioz/lockfile"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// OptCacheLockTimeout is the time allowed to acquire a cache lock.
//
// See also: [driver.OptIngestCache].
var OptCacheLockTimeout = options.NewDuration(
	"cache.lock.timeout",
	"",
	0,
	time.Second*5,
	"Wait timeout to acquire cache lock",
	`Wait timeout to acquire cache lock. During this period, retry will occur
if the lock is already held by another process. If zero, no retry occurs.`,
)

// CacheDir returns the cache dir. It is not guaranteed that the
// returned dir exists.
func (fs *Files) CacheDir() string {
	return fs.cacheDir
}

// TempDir returns the temp dir. It is not guaranteed that the
// returned dir exists.
func (fs *Files) TempDir() string {
	return fs.tempDir
}

// CacheDirFor gets the cache dir for handle. It is not guaranteed
// that the returned dir exists or is accessible.
func (fs *Files) CacheDirFor(src *Source) (dir string, err error) {
	handle := src.Handle
	if err = ValidHandle(handle); err != nil {
		return "", errz.Wrapf(err, "cache dir: invalid handle: %s", handle)
	}

	if handle == StdinHandle {
		// stdin is different input every time, so we need a unique
		// cache dir. In practice, stdin probably isn't using this function.
		handle += "_" + stringz.UniqN(32)
	}

	dir = filepath.Join(
		fs.cacheDir,
		"sources",
		filepath.Join(strings.Split(strings.TrimPrefix(handle, "@"), "/")...),
		fs.sourceHash(src),
	)

	return dir, nil
}

// WriteIngestChecksum is invoked (after successful ingestion) to write the
// checksum for the ingest cache db.
func (fs *Files) WriteIngestChecksum(ctx context.Context, src, backingSrc *Source) (err error) {
	log := lg.FromContext(ctx)
	ingestFilePath, err := fs.filepath(src)
	if err != nil {
		return err
	}

	// Write the checksums file.
	var sum checksum.Checksum
	if sum, err = checksum.ForFile(ingestFilePath); err != nil {
		log.Warn("Failed to compute checksum for source file; caching not in effect",
			lga.Src, src, lga.Dest, backingSrc, lga.Path, ingestFilePath, lga.Err, err)
		return err
	}

	var checksumsPath string
	if _, _, checksumsPath, err = fs.CachePaths(src); err != nil {
		return err
	}

	if err = checksum.WriteFile(checksumsPath, sum, ingestFilePath); err != nil {
		log.Warn("Failed to write checksum; file caching not in effect",
			lga.Src, src, lga.Dest, backingSrc, lga.Path, ingestFilePath, lga.Err, err)
	}
	return err
}

// CachedBackingSourceFor returns the underlying backing source for src, if
// it exists. If it does not exist, ok returns false.
func (fs *Files) CachedBackingSourceFor(ctx context.Context, src *Source) (backingSrc *Source, ok bool, err error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	switch getLocType(src.Location) {
	case locTypeLocalFile:
		return fs.cachedBackingSourceForFile(ctx, src)
	case locTypeRemoteFile:
		return fs.cachedBackingSourceForRemoteFile(ctx, src)
	default:
		return nil, false, errz.Errorf("caching not applicable for source: %s", src.Handle)
	}
}

// cachedBackingSourceForFile returns the underlying cached backing
// source for src, if it exists.
func (fs *Files) cachedBackingSourceForFile(ctx context.Context, src *Source) (*Source, bool, error) {
	_, cacheDBPath, checksumsPath, err := fs.CachePaths(src)
	if err != nil {
		return nil, false, err
	}

	if !ioz.FileAccessible(checksumsPath) {
		return nil, false, nil
	}

	mChecksums, err := checksum.ReadFile(checksumsPath)
	if err != nil {
		return nil, false, err
	}

	srcFilepath, err := fs.filepath(src)
	if err != nil {
		return nil, false, err
	}

	cachedChecksum, ok := mChecksums[srcFilepath]
	if !ok {
		return nil, false, nil
	}

	srcChecksum, err := checksum.ForFile(srcFilepath)
	if err != nil {
		return nil, false, err
	}

	if srcChecksum != cachedChecksum {
		return nil, false, nil
	}

	// The checksums match, so we can use the cached DB,
	// if it exists.
	if !ioz.FileAccessible(cacheDBPath) {
		return nil, false, nil
	}

	backingSrc := &Source{
		Handle:   src.Handle + "_cached",
		Location: "sqlite3://" + cacheDBPath,
		Type:     drivertype.Type("sqlite3"),
	}

	lg.FromContext(ctx).Debug("Found cached backing source DB src", lga.Src, src, "backing_src", backingSrc)
	return backingSrc, true, nil
}

// cachedBackingSourceForRemoteFile returns the underlying cached backing
// source for src, if it exists.
func (fs *Files) cachedBackingSourceForRemoteFile(ctx context.Context, src *Source) (*Source, bool, error) {
	log := lg.FromContext(ctx)

	downloadedFile, r, err := fs.openRemoteFile(ctx, src, true)
	if err != nil {
		return nil, false, err
	}

	// We don't care about the reader, but we do need to close it.
	lg.WarnIfCloseError(log, lgm.CloseFileReader, r)
	if downloadedFile == "" {
		log.Debug("No cached download file for src", lga.Src, src)
		return nil, false, nil
	}

	log.Debug("Found cached download file for src", lga.Src, src, lga.Path, downloadedFile)
	return fs.cachedBackingSourceForFile(ctx, src)
}

// CachePaths returns the paths to the cache files for src.
// There is no guarantee that these files exist, or are accessible.
// It's just the paths.
func (fs *Files) CachePaths(src *Source) (srcCacheDir, cacheDB, checksums string, err error) {
	if srcCacheDir, err = fs.CacheDirFor(src); err != nil {
		return "", "", "", err
	}

	checksums = filepath.Join(srcCacheDir, "checksums.txt")
	cacheDB = filepath.Join(srcCacheDir, "cache.sqlite.db")
	return srcCacheDir, cacheDB, checksums, nil
}

// sourceHash generates a hash for src. The hash is based on the
// member fields of src, with special handling for src.Options.
// Only the opts that affect data ingestion (options.TagIngestMutate)
// are incorporated in the hash. The generated hash is used to
// determine the cache dir for src. Thus, if a source is mutated
// (e.g. the remote http location changes), a new cache dir results.
func (fs *Files) sourceHash(src *Source) string {
	if src == nil {
		return ""
	}

	buf := bytes.Buffer{}
	buf.WriteString(src.Handle)
	buf.WriteString(string(src.Type))
	buf.WriteString(src.Location)
	buf.WriteString(src.Catalog)
	buf.WriteString(src.Schema)

	mUsedKeys := make(map[string]any)
	if src.Options != nil {
		keys := src.Options.Keys()
		for _, k := range keys {
			opt := fs.optRegistry.Get(k)
			switch {
			case opt == nil,
				!opt.IsSet(src.Options),
				!opt.HasTag(options.TagIngestMutate):
				continue
			default:
			}

			buf.WriteString(k)
			v := src.Options[k]
			buf.WriteString(fmt.Sprintf("%v", v))
			mUsedKeys[k] = v
		}
	}

	sum := checksum.Sum(buf.Bytes())
	return sum
}

// cacheLockFor returns the lock file for src's cache.
func (fs *Files) cacheLockFor(src *Source) (lockfile.Lockfile, error) {
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

// CacheLockAcquire acquires the cache lock for src. The caller must invoke
// the returned unlock func.
func (fs *Files) CacheLockAcquire(ctx context.Context, src *Source) (unlock func(), err error) {
	lock, err := fs.cacheLockFor(src)
	if err != nil {
		return nil, err
	}

	lockTimeout := OptCacheLockTimeout.Get(options.FromContext(ctx))
	log := lg.FromContext(ctx).With(lga.Src, src, lga.Timeout, lockTimeout, lga.Lock, lock)
	log.Debug("Acquiring cache lock for source")

	bar := progress.FromContext(ctx).NewTimeoutWaiter(
		src.Handle+": acquire lock",
		time.Now().Add(lockTimeout),
	)

	err = lock.Lock(ctx, lockTimeout)
	bar.Stop()
	if err != nil {
		return nil, errz.Wrap(err, src.Handle+": acquire cache lock")
	}

	return func() {
		if err = lock.Unlock(); err != nil {
			log.Warn("Failed to release cache lock", lga.Err, err)
		}
	}, nil
}

// CacheClearAll clears the entire cache dir.
// Note that this operation is distinct from [Files.doCacheSweep].
func (fs *Files) CacheClearAll(ctx context.Context) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.doCacheClearAll(ctx)
}

// CacheClearSource clears the ingest cache for src. If arg downloads is true,
// the source's download dir is also cleared. The caller should typically
// first acquire the cache lock for src via [Files.cacheLockFor].
func (fs *Files) CacheClearSource(ctx context.Context, src *Source, clearDownloads bool) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.doCacheClearSource(ctx, src, clearDownloads)
}

func (fs *Files) doCacheClearSource(ctx context.Context, src *Source, clearDownloads bool) error {
	cacheDir, err := fs.CacheDirFor(src)
	if err != nil {
		return err
	}

	if !ioz.DirExists(cacheDir) {
		return nil
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return errz.Wrapf(err, "%s: clear cache", src.Handle)
	}

	for _, entry := range entries {
		switch entry.Name() {
		case "pid.lock":
			continue
		case "download":
			if !clearDownloads {
				continue
			}
		default:
		}

		if err = os.RemoveAll(filepath.Join(cacheDir, entry.Name())); err != nil {
			return errz.Wrapf(err, "%s: clear cache", src.Handle)
		}
	}

	lg.FromContext(ctx).
		With("clear_downloads", clearDownloads, lga.Src, src, lga.Dir, cacheDir).
		Info("Cleared source cache")
	return nil
}

func (fs *Files) doCacheClearAll(ctx context.Context) error {
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

// doCacheSweep sweeps the cache dir, making a best-effort attempt
// to remove any empty directories. Note that this operation is
// distinct from [Files.CacheClearAll].
//
// REVISIT: This doesn't really do as much as desired. It should
// also be able to detect orphaned src cache dirs and delete those.
func (fs *Files) doCacheSweep(ctx context.Context) {
	dir := fs.cacheDir
	log := lg.FromContext(ctx).With(lga.Dir, dir)
	log.Debug("Sweep cache dir: acquiring config lock")

	if unlock, err := fs.cfgLockFn(ctx); err != nil {
		log.Error("Sweep cache dir: failed to acquire config lock", lga.Lock, fs.cfgLockFn, lga.Err, err)
		return
	} else {
		defer unlock()
	}

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

// DefaultCacheDir returns the sq cache dir. This is generally
// in USER_CACHE_DIR/*/sq, but could also be in TEMP_DIR/*/sq/cache
// or similar. It is not guaranteed that the returned dir exists
// or is accessible.
func DefaultCacheDir() (dir string) {
	var err error
	if dir, err = os.UserCacheDir(); err != nil {
		// Some systems may not have a user cache dir, so we fall back
		// to the system temp dir.
		dir = filepath.Join(DefaultTempDir(), "cache")
		return dir
	}

	dir = filepath.Join(dir, "sq")
	return dir
}

// DefaultTempDir returns the default sq temp dir. It is not
// guaranteed that the returned dir exists or is accessible.
func DefaultTempDir() (dir string) {
	return filepath.Join(os.TempDir(), "sq")
}

