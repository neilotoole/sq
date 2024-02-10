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

	if newFi, _ := os.Stat(newpath); newFi != nil && newFi.IsDir() {
		// os.Rename will fail when newpath is a directory.
		// So, we first move (rename) newpath to a temp staging path,
		// and then we move oldpath to newpath, and finally we remove
		// the staging dir. We do this because if there's open files
		// in newpath, os.RemoveAll may fail, so this technique is
		// more likely to succeed? Not completely sure about that.
		staging := filepath.Join(os.TempDir(), stringz.Uniq8()+"_"+filepath.Base(newpath))

		if err = os.Rename(newpath, staging); err != nil {
			return errz.Err(err)
		}

		if err = os.Rename(oldDir, newpath); err != nil {
			return errz.Err(err)
		}

		// If the staging deletion (i.e. the old dir) fails,
		// do we even care? It'll be left hanging around in
		// the tmp dir, which I guess could be a security
		// issue in some circumstances?
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

				fi, err2 = os.Stat(linked)
				if err2 != nil {
					err = errz.Append(err, errz.Err(err2))
					continue
				}

				if fi.IsDir() {
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

	return paths, nil
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
// a Go implementation of the venerable "tree" command.
func PrintTree(w io.Writer, loc string, showSize, colorize bool) error {
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
	return doPruneEmptyDirTree(ctx, dir, true)
}

func doPruneEmptyDirTree(ctx context.Context, dir string, isRoot bool) (count int, err error) {
	if !filepath.IsAbs(dir) {
		return 0, errz.Errorf("dir must be absolute: %s", dir)
	}

	select {
	case <-ctx.Done():
		return 0, errz.Err(ctx.Err())
	default:
	}

	var entries []os.DirEntry
	if entries, err = os.ReadDir(dir); err != nil {
		return 0, errz.Err(err)
	}

	if len(entries) == 0 {
		if isRoot {
			return 0, nil
		}
		err = os.RemoveAll(dir)
		if err != nil {
			return 0, errz.Err(err)
		}
		return 1, nil
	}

	// We've got some entries... let's check what they are.
	if countNonDirs(entries) != 0 {
		// There are some non-dir entries, so this dir doesn't get deleted.
		return 0, nil
	}

	// Each of the entries is a dir. Recursively prune.
	var n int
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return count, errz.Err(ctx.Err())
		default:
		}

		n, err = doPruneEmptyDirTree(ctx, filepath.Join(dir, entry.Name()), false)
		count += n
		if err != nil {
			return count, err
		}
	}

	return count, nil
}
