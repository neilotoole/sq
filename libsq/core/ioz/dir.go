package ioz

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/a8m/tree"
	"github.com/a8m/tree/ostree"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

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

	if filepath.Clean(oldDir) == filepath.Clean(newpath) {
		// Renaming to the same path is a no-op, matching os.Rename.
		return nil
	}

	if newFi, _ := os.Stat(newpath); newFi != nil && newFi.IsDir() {
		// os.Rename fails when newpath already exists and is a directory. So we
		// first move newpath aside to a staging path, then move oldDir to
		// newpath, then remove the staging dir. We move newpath aside (rather
		// than deleting it first) so that if any open files prevent removal, the
		// original is still recoverable.
		//
		// The staging path is created as a sibling of newpath (same directory,
		// hence same filesystem) so that os.Rename can't fail with EXDEV.
		// Staging via os.TempDir would break whenever TMPDIR is on a different
		// device than newpath.
		staging := filepath.Join(filepath.Dir(newpath), stringz.Uniq8()+"_"+filepath.Base(newpath))

		if err = os.Rename(newpath, staging); err != nil {
			return errz.Err(err)
		}

		if err = os.Rename(oldDir, newpath); err != nil {
			// Restore the original newpath from staging, so a failed rename
			// doesn't leave newpath missing. If the rollback also fails, surface
			// both errors plus the staging location, since that's now the only
			// copy of newpath's original contents.
			if rbErr := os.Rename(staging, newpath); rbErr != nil {
				return errz.Append(errz.Err(err),
					errz.Wrapf(rbErr, "rollback failed: original contents of %s remain at %s", newpath, staging))
			}
			return errz.Err(err)
		}

		// The staging dir (the previous newpath contents) is no longer needed.
		// If removal fails it's left in the same directory; not worth surfacing.
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

				linkedFi, err2 := os.Stat(linked)
				if err2 != nil {
					err = errz.Append(err, errz.Err(err2))
					continue
				}

				if linkedFi.IsDir() {
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

	// Note: err may be a non-nil multierr accumulated while resolving symlinks
	// above. Per the doc contract, paths is still returned alongside it.
	return paths, err
}

func countNonDirs(entries []os.DirEntry) (count int) {
	for _, entry := range entries {
		if !entry.IsDir() {
			count++
		}
	}
	return count
}

// DirExists returns true if dir exists and is a directory.
func DirExists(dir string) bool {
	fi, err := os.Stat(dir)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

// RequireDir ensures that dir exists and is a directory, creating
// it if necessary.
func RequireDir(dir string) error {
	return errz.Err(os.MkdirAll(dir, 0o700))
}

// PrintTree prints the file tree structure at loc to w.
// This function uses the github.com/a8m/tree library, which is
// a Go implementation of the venerable "tree" command. Per-entry errors
// encountered while walking are rendered inline in the tree output by the
// library; an error is returned only if loc itself can't be accessed.
func PrintTree(w io.Writer, loc string, showSize, colorize bool) error {
	if _, err := os.Stat(loc); err != nil {
		return errz.Err(err)
	}

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

// PruneEmptyDirTree prunes empty dirs, and dirs that contain only
// other empty dirs, from the directory tree rooted at dir. If a dir
// contains at least one non-dir entry, that dir is spared. Arg dir
// must be an absolute path.
func PruneEmptyDirTree(ctx context.Context, dir string) (count int, err error) {
	count, _, err = doPruneEmptyDirTree(ctx, dir, true)
	return count, err
}

// doPruneEmptyDirTree implements [PruneEmptyDirTree] via a single post-order
// pass. It returns the number of dirs removed in the subtree rooted at dir, and
// removedSelf reports whether dir itself was removed (always false for the root,
// which is never pruned). The parent uses removedSelf to decide whether it has
// in turn become empty, so chains of dirs containing only empty dirs collapse in
// one pass without re-reading any dir.
func doPruneEmptyDirTree(ctx context.Context, dir string, isRoot bool) (count int, removedSelf bool, err error) {
	if !filepath.IsAbs(dir) {
		return 0, false, errz.Errorf("dir must be absolute: %s", dir)
	}

	select {
	case <-ctx.Done():
		return 0, false, errz.Err(ctx.Err())
	default:
	}

	var entries []os.DirEntry
	if entries, err = os.ReadDir(dir); err != nil {
		return 0, false, errz.Err(err)
	}

	// If any entry is a non-dir, this dir holds real content and is spared (as
	// are its descendants). Note that countNonDirs treats a symlink, including a
	// symlink to a dir, as a non-dir, so a dir containing one is never pruned.
	if countNonDirs(entries) != 0 {
		return 0, false, nil
	}

	// Every entry (if any) is a dir. Recursively prune each, tracking whether
	// they all removed themselves. With no entries, the dir is already empty and
	// allChildrenRemoved stays true.
	allChildrenRemoved := true
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return count, false, errz.Err(ctx.Err())
		default:
		}

		var (
			n            int
			childRemoved bool
		)
		n, childRemoved, err = doPruneEmptyDirTree(ctx, filepath.Join(dir, entry.Name()), false)
		count += n
		if err != nil {
			return count, false, err
		}
		if !childRemoved {
			allChildrenRemoved = false
		}
	}

	// dir is now empty iff every child removed itself (or it had no children).
	// Prune it too, unless it's the root, which is never removed.
	if !isRoot && allChildrenRemoved {
		if err = os.RemoveAll(dir); err != nil {
			return count, false, errz.Err(err)
		}
		return count + 1, true, nil
	}

	return count, false, nil
}
