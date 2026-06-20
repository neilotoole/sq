package ioz_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
)

func TestCopyFile(t *testing.T) {
	const content = "In Xanadu did Kubla Khan"

	t.Run("happy", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "src.txt")
		require.NoError(t, os.WriteFile(src, []byte(content), 0o640))
		srcFi, err := os.Stat(src)
		require.NoError(t, err)

		dst := filepath.Join(t.TempDir(), "dst.txt")
		require.NoError(t, ioz.CopyFile(dst, src, false))

		got, err := os.ReadFile(dst)
		require.NoError(t, err)
		require.Equal(t, content, string(got))

		dstFi, err := os.Stat(dst)
		require.NoError(t, err)
		require.Equal(t, srcFi.Mode(), dstFi.Mode(), "dst must inherit src perms")
	})

	t.Run("mkdir_creates_parent", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "src.txt")
		require.NoError(t, os.WriteFile(src, []byte(content), 0o600))

		dst := filepath.Join(t.TempDir(), "newdir", "nested", "dst.txt")
		require.NoError(t, ioz.CopyFile(dst, src, true))
		require.True(t, ioz.FileAccessible(dst))
	})

	t.Run("mkdir_fails_when_parent_is_file", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "src.txt")
		require.NoError(t, os.WriteFile(src, []byte(content), 0o600))

		// A regular file sits where a parent dir would need to be created.
		blocker := filepath.Join(t.TempDir(), "blocker")
		require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))
		dst := filepath.Join(blocker, "sub", "dst.txt")
		require.Error(t, ioz.CopyFile(dst, src, true))
	})

	t.Run("mkdir_false_missing_parent_errors", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "src.txt")
		require.NoError(t, os.WriteFile(src, []byte(content), 0o600))

		dst := filepath.Join(t.TempDir(), "missingdir", "dst.txt")
		require.Error(t, ioz.CopyFile(dst, src, false))
	})

	t.Run("src_not_exist", func(t *testing.T) {
		err := ioz.CopyFile(filepath.Join(t.TempDir(), "dst.txt"), filepath.Join(t.TempDir(), "nope"), false)
		require.Error(t, err)
	})

	t.Run("src_is_dir_copy_fails", func(t *testing.T) {
		// os.Open on a dir succeeds, but reading it for io.Copy fails, exercising
		// the copy-failure cleanup path. The dst temp file must not survive.
		srcDir := t.TempDir()
		dst := filepath.Join(t.TempDir(), "dst.txt")
		require.Error(t, ioz.CopyFile(dst, srcDir, false))
		require.False(t, ioz.FileAccessible(dst))
	})

	t.Run("overwrites_existing_dst", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "src.txt")
		require.NoError(t, os.WriteFile(src, []byte(content), 0o600))
		dst := filepath.Join(t.TempDir(), "dst.txt")
		require.NoError(t, os.WriteFile(dst, []byte("old contents to be replaced"), 0o600))

		require.NoError(t, ioz.CopyFile(dst, src, false))
		got, err := os.ReadFile(dst)
		require.NoError(t, err)
		require.Equal(t, content, string(got))
	})
}

func TestPrintFile(t *testing.T) {
	// PrintFile writes to os.Stdout; capture it via a pipe to keep test output
	// clean and to assert on the content.
	const content = "stdout content"
	f := filepath.Join(t.TempDir(), "f.txt")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o600))

	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	printErr := ioz.PrintFile(f)
	require.NoError(t, w.Close())
	os.Stdout = orig

	require.NoError(t, printErr)
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, content, string(got))
}

func TestFPrintFile(t *testing.T) {
	const content = "hello world"
	f := filepath.Join(t.TempDir(), "f.txt")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o600))

	buf := &strings.Builder{}
	require.NoError(t, ioz.FPrintFile(buf, f))
	require.Equal(t, content, buf.String())
}

func TestFPrintFile_notExist(t *testing.T) {
	require.Error(t, ioz.FPrintFile(&strings.Builder{}, filepath.Join(t.TempDir(), "nope")))
}

func TestIsPathToRegularFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "f.txt")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o600))

	require.True(t, ioz.IsPathToRegularFile(f))
	require.False(t, ioz.IsPathToRegularFile(dir), "a dir is not a regular file")
	require.False(t, ioz.IsPathToRegularFile(filepath.Join(dir, "nope")))

	if runtime.GOOS != "windows" {
		link := filepath.Join(dir, "link")
		require.NoError(t, os.Symlink(f, link))
		require.True(t, ioz.IsPathToRegularFile(link), "symlink to a regular file resolves true")
	}
}

func TestFileAccessible(t *testing.T) {
	f := filepath.Join(t.TempDir(), "f.txt")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o600))
	require.True(t, ioz.FileAccessible(f))
	require.False(t, ioz.FileAccessible(filepath.Join(t.TempDir(), "nope")))
}

func TestReadFileToString(t *testing.T) {
	const content = "a stately pleasure dome"
	f := filepath.Join(t.TempDir(), "f.txt")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o600))

	got, err := ioz.ReadFileToString(f)
	require.NoError(t, err)
	require.Equal(t, content, got)

	_, err = ioz.ReadFileToString(filepath.Join(t.TempDir(), "nope"))
	require.Error(t, err)
}

func TestFilesize(t *testing.T) {
	f := filepath.Join(t.TempDir(), "f.txt")
	require.NoError(t, os.WriteFile(f, []byte("12345"), 0o600))

	size, err := ioz.Filesize(f)
	require.NoError(t, err)
	require.Equal(t, int64(5), size)

	t.Run("not_exist", func(t *testing.T) {
		_, err := ioz.Filesize(filepath.Join(t.TempDir(), "nope"))
		require.Error(t, err)
	})

	t.Run("is_dir", func(t *testing.T) {
		_, err := ioz.Filesize(t.TempDir())
		require.Error(t, err)
		require.Contains(t, err.Error(), "not a file")
	})
}

func TestFileInfoEqual(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "f.txt")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o600))
	fi1, err := os.Stat(f)
	require.NoError(t, err)
	fi2, err := os.Stat(f)
	require.NoError(t, err)

	other := filepath.Join(dir, "other.txt")
	require.NoError(t, os.WriteFile(other, []byte("yy"), 0o600))
	fiOther, err := os.Stat(other)
	require.NoError(t, err)

	require.True(t, ioz.FileInfoEqual(nil, nil))
	require.False(t, ioz.FileInfoEqual(fi1, nil))
	require.False(t, ioz.FileInfoEqual(nil, fi1))
	require.True(t, ioz.FileInfoEqual(fi1, fi2))
	require.False(t, ioz.FileInfoEqual(fi1, fiOther))
}

func TestWriteToFile_contextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fp := filepath.Join(t.TempDir(), "out.txt")
	_, err := ioz.WriteToFile(ctx, fp, strings.NewReader("some data"))
	require.Error(t, err)
}

func TestWriteToFile_truncatesExisting(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "out.txt")
	require.NoError(t, os.WriteFile(fp, []byte("a very long pre-existing payload"), 0o600))

	written, err := ioz.WriteToFile(context.Background(), fp, strings.NewReader("short"))
	require.NoError(t, err)
	require.Equal(t, int64(5), written)

	got, err := os.ReadFile(fp)
	require.NoError(t, err)
	require.Equal(t, "short", string(got))
}

func TestWriteToFile_readerError(t *testing.T) {
	wantErr := io.ErrUnexpectedEOF
	fp := filepath.Join(t.TempDir(), "out.txt")
	_, err := ioz.WriteToFile(context.Background(), fp, ioz.ErrReader{Err: wantErr})
	require.Error(t, err)
}

func TestWriteToFile_requireDirFails(t *testing.T) {
	// A regular file blocks creation of the parent dir.
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))

	fp := filepath.Join(blocker, "out.txt")
	_, err := ioz.WriteToFile(context.Background(), fp, strings.NewReader("data"))
	require.Error(t, err)
}

func TestWriteFileAtomic(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "atomic.txt")
	const content = "atomic write"
	require.NoError(t, ioz.WriteFileAtomic(fp, []byte(content), ioz.RWPerms))

	got, err := os.ReadFile(fp)
	require.NoError(t, err)
	require.Equal(t, content, string(got))
}
