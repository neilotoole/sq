package ioz_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
)

func TestRenameDir_simple(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "f.txt"), []byte("hi"), 0o600))

	dst := filepath.Join(t.TempDir(), "newname")
	require.NoError(t, ioz.RenameDir(src, dst))
	require.True(t, ioz.DirExists(dst))
	require.False(t, ioz.DirExists(src))

	got, err := os.ReadFile(filepath.Join(dst, "f.txt"))
	require.NoError(t, err)
	require.Equal(t, "hi", string(got))
}

func TestRenameDir_overExistingDir(t *testing.T) {
	// newpath already exists as a non-empty dir, exercising the staging path.
	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "new.txt"), []byte("new"), 0o600))

	dst := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dst, "old.txt"), []byte("old"), 0o600))

	require.NoError(t, ioz.RenameDir(src, dst))

	// dst now holds src's content; the old content is gone.
	require.True(t, ioz.FileAccessible(filepath.Join(dst, "new.txt")))
	require.False(t, ioz.FileAccessible(filepath.Join(dst, "old.txt")))
	require.False(t, ioz.DirExists(src))

	// No staging dir should be left behind. RenameDir stages newpath aside as
	// "<uniq8>_<base(newpath)>" in newpath's parent, so check for that suffix.
	parent := filepath.Dir(dst)
	entries, err := os.ReadDir(parent)
	require.NoError(t, err)
	stagingSuffix := "_" + filepath.Base(dst)
	for _, e := range entries {
		require.NotContains(t, e.Name(), stagingSuffix, "staging dir must be cleaned up")
	}
}

func TestRenameDir_samePathNoOp(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hi"), 0o600))

	// Renaming a dir onto itself is a no-op, matching os.Rename.
	require.NoError(t, ioz.RenameDir(dir, dir))
	require.True(t, ioz.FileAccessible(filepath.Join(dir, "f.txt")))
}

func TestRenameDir_errors(t *testing.T) {
	t.Run("source_not_exist", func(t *testing.T) {
		err := ioz.RenameDir(filepath.Join(t.TempDir(), "nope"), filepath.Join(t.TempDir(), "dst"))
		require.Error(t, err)
	})

	t.Run("source_not_dir", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "afile")
		require.NoError(t, os.WriteFile(f, []byte("x"), 0o600))
		err := ioz.RenameDir(f, filepath.Join(t.TempDir(), "dst"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "not a dir")
	})
}

func TestReadDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".hidden"), []byte("x"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o700))

	t.Run("default", func(t *testing.T) {
		paths, err := ioz.ReadDir(dir, false, false, false)
		require.NoError(t, err)
		sort.Strings(paths)
		require.Equal(t, []string{"a.txt", "b.txt", "sub"}, paths)
	})

	t.Run("includeDot", func(t *testing.T) {
		paths, err := ioz.ReadDir(dir, false, false, true)
		require.NoError(t, err)
		require.Contains(t, paths, ".hidden")
	})

	t.Run("markDirs", func(t *testing.T) {
		paths, err := ioz.ReadDir(dir, false, true, false)
		require.NoError(t, err)
		require.Contains(t, paths, "sub/")
	})

	t.Run("includeDirPath", func(t *testing.T) {
		paths, err := ioz.ReadDir(dir, true, false, false)
		require.NoError(t, err)
		for _, p := range paths {
			require.True(t, strings.HasPrefix(p, dir), "path %q should be prefixed with dir", p)
		}
	})

	t.Run("includeDirPath_markDirs_preservesSlash", func(t *testing.T) {
		paths, err := ioz.ReadDir(dir, true, true, false)
		require.NoError(t, err)
		var foundSub bool
		for _, p := range paths {
			if strings.HasSuffix(p, "sub/") {
				foundSub = true
				require.True(t, strings.HasPrefix(p, dir))
			}
		}
		require.True(t, foundSub, "marked dir with full path must retain trailing slash")
	})
}

func TestReadDir_errors(t *testing.T) {
	t.Run("not_exist", func(t *testing.T) {
		_, err := ioz.ReadDir(filepath.Join(t.TempDir(), "nope"), false, false, false)
		require.Error(t, err)
	})

	t.Run("not_a_dir", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "afile")
		require.NoError(t, os.WriteFile(f, []byte("x"), 0o600))
		_, err := ioz.ReadDir(f, false, false, false)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not a dir")
	})
}

func TestReadDir_symlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks are unreliable on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "realdir")
	require.NoError(t, os.Mkdir(target, 0o700))

	// A symlink that resolves to a directory.
	require.NoError(t, os.Symlink(target, filepath.Join(dir, "linktodir")))
	// A broken symlink, to exercise the EvalSymlinks error path under markDirs.
	brokenTarget := filepath.Join(t.TempDir(), "missing_target_dir")
	require.NoError(t, os.Symlink(brokenTarget, filepath.Join(dir, "broken")))

	t.Run("symlinked_dir_marked", func(t *testing.T) {
		// Drop the broken link so only the good symlink is present.
		d2 := t.TempDir()
		require.NoError(t, os.Symlink(target, filepath.Join(d2, "linktodir")))
		paths, err := ioz.ReadDir(d2, false, true, false)
		require.NoError(t, err)
		require.Contains(t, paths, "linktodir/")
	})

	t.Run("broken_symlink_yields_error_but_continues", func(t *testing.T) {
		paths, err := ioz.ReadDir(dir, false, true, false)
		require.Error(t, err, "broken symlink under markDirs should produce an error")
		// The error must identify the unresolvable target, proving it was
		// accumulated rather than swallowed.
		require.ErrorContains(t, err, "missing_target_dir")
		// The good symlink is still listed despite the broken one's error,
		// proving ReadDir accumulates the error and continues.
		require.Contains(t, paths, "linktodir/")
		require.NotContains(t, paths, "broken", "unresolvable symlink should be omitted from paths")
	})
}

func TestDirExists(t *testing.T) {
	require.True(t, ioz.DirExists(t.TempDir()))
	require.False(t, ioz.DirExists(filepath.Join(t.TempDir(), "nope")))

	f := filepath.Join(t.TempDir(), "afile")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o600))
	require.False(t, ioz.DirExists(f), "a regular file is not a dir")
}

func TestRequireDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b", "c")
	require.NoError(t, ioz.RequireDir(dir))
	require.True(t, ioz.DirExists(dir))
	// Idempotent.
	require.NoError(t, ioz.RequireDir(dir))
}

func TestDirSize(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a"), []byte("12345"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "b"), []byte("678"), 0o600))

	size, err := ioz.DirSize(dir)
	require.NoError(t, err)
	require.Equal(t, int64(8), size)
}

func TestDirSize_notExist(t *testing.T) {
	_, err := ioz.DirSize(filepath.Join(t.TempDir(), "nope"))
	require.Error(t, err)
}

func TestPrintTree(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hi"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o700))

	buf := &strings.Builder{}
	require.NoError(t, ioz.PrintTree(buf, dir, true, false))
	out := buf.String()
	require.Contains(t, out, "a.txt")
	require.Contains(t, out, "sub")
}

func TestPrintTree_missingLoc(t *testing.T) {
	buf := &strings.Builder{}
	err := ioz.PrintTree(buf, filepath.Join(t.TempDir(), "nope"), false, false)
	require.Error(t, err, "an inaccessible loc must return an error, not silent empty output")
}

func TestPruneEmptyDirTree(t *testing.T) {
	t.Run("not_absolute", func(t *testing.T) {
		_, err := ioz.PruneEmptyDirTree(context.Background(), "relative/path")
		require.Error(t, err)
		require.Contains(t, err.Error(), "must be absolute")
	})

	t.Run("collapses_empty_chains_spares_files", func(t *testing.T) {
		root := t.TempDir()
		// root/a            (empty leaf)
		// root/b/c          (b holds only the empty dir c)
		// root/d/file       (d holds a file, spared)
		require.NoError(t, os.MkdirAll(filepath.Join(root, "a"), 0o700))
		require.NoError(t, os.MkdirAll(filepath.Join(root, "b", "c"), 0o700))
		require.NoError(t, os.MkdirAll(filepath.Join(root, "d"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(root, "d", "file"), []byte("x"), 0o600))

		count, err := ioz.PruneEmptyDirTree(context.Background(), root)
		require.NoError(t, err)
		require.Equal(t, 3, count, "a, c, and (now-empty) b should all be pruned")

		require.False(t, ioz.DirExists(filepath.Join(root, "a")))
		require.False(t, ioz.DirExists(filepath.Join(root, "b")), "dir containing only empty dirs must be pruned")
		require.False(t, ioz.DirExists(filepath.Join(root, "b", "c")))
		require.True(t, ioz.DirExists(filepath.Join(root, "d")), "dir with a file must be spared")
		require.True(t, ioz.DirExists(root), "the root itself is never pruned")
	})

	t.Run("empty_root_not_removed", func(t *testing.T) {
		root := t.TempDir()
		count, err := ioz.PruneEmptyDirTree(context.Background(), root)
		require.NoError(t, err)
		require.Zero(t, count)
		require.True(t, ioz.DirExists(root))
	})

	t.Run("symlink_to_dir_spares_parent", func(t *testing.T) {
		// countNonDirs treats a symlink (even to a dir) as a non-dir, so a dir
		// whose only entry is a symlink-to-dir must be spared. This pins the
		// invariant the post-order removal relies on.
		if runtime.GOOS == "windows" {
			t.Skip("symlinks are unreliable on Windows")
		}
		root := t.TempDir()
		realDir := filepath.Join(t.TempDir(), "real")
		require.NoError(t, os.Mkdir(realDir, 0o700))

		sub := filepath.Join(root, "sub")
		require.NoError(t, os.Mkdir(sub, 0o700))
		require.NoError(t, os.Symlink(realDir, filepath.Join(sub, "link")))

		count, err := ioz.PruneEmptyDirTree(context.Background(), root)
		require.NoError(t, err)
		require.Zero(t, count)
		require.True(t, ioz.DirExists(sub), "dir holding a symlink must not be pruned")
	})

	t.Run("context_cancelled", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, "a"), 0o700))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := ioz.PruneEmptyDirTree(ctx, root)
		require.Error(t, err)
	})
}
