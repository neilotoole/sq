package source

import (
	"os"
	"path/filepath"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// CacheDirFor gets the cache dir for handle, creating it if necessary.
// If handle is empty or invalid, a random value is generated.
func CacheDirFor(src *Source) (dir string, err error) {
	handle := src.Handle
	switch handle {
	case "":
		// FIXME: This is surely an error?
		return "", errz.Errorf("open cache dir: empty handle")
		// handle = "@cache_" + stringz.UniqN(32)
	case StdinHandle:
		// stdin is different input every time, so we need a unique
		// cache dir. In practice, stdin probably isn't using this function.
		handle += "_" + stringz.UniqN(32)
	default:
		if err = ValidHandle(handle); err != nil {
			return "", errz.Wrapf(err, "open cache dir: invalid handle: %s", handle)
		}
	}

	dir = CacheDirPath()
	sanitized := Handle2SafePath(handle)
	hash := src.Hash()
	dir = filepath.Join(dir, "sources", sanitized, hash)
	if err = os.MkdirAll(dir, 0o750); err != nil {
		return "", errz.Wrapf(err, "open cache dir: %s", dir)
	}

	return dir, nil
}

// CacheDirPath returns the sq cache dir. This is generally
// in USER_CACHE_DIR/sq/cache, but could also be in TEMP_DIR/sq/cache
// or similar. It is not guaranteed that the returned dir exists
// or is accessible.
func CacheDirPath() (dir string) {
	var err error
	if dir, err = os.UserCacheDir(); err != nil {
		// Some systems may not have a user cache dir, so we fall back
		// to the system temp dir.
		dir = filepath.Join(os.TempDir(), "sq", "cache")
		return dir
	}

	dir = filepath.Join(dir, "sq")
	return dir
}
