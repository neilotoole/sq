package source

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neilotoole/sq/libsq/core/ioz/lockfile"

	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/source/drivertype"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/stringz"
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
//
// REVISIT: This doesn't really do anything useful. It should instead
// sweep any abandoned cache dirs, i.e. cache dirs that don't have
// an associated source.
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
