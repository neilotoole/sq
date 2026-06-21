package ioz_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
)

func TestNewDefaultBuffer(t *testing.T) {
	buf := ioz.NewDefaultBuffer()
	t.Cleanup(func() { require.NoError(t, buf.Close()) })

	const val = "In Xanadu did Kubla Khan"
	n, err := buf.Write([]byte(val))
	require.NoError(t, err)
	require.Equal(t, len(val), n)

	require.Equal(t, int64(len(val)), buf.Len())
	require.GreaterOrEqual(t, buf.Cap(), int64(len(val)))

	got, err := io.ReadAll(buf)
	require.NoError(t, err)
	require.Equal(t, val, string(got))

	// After draining, Len is zero.
	require.Zero(t, buf.Len())

	// Reset empties the buffer.
	_, err = buf.Write([]byte("more"))
	require.NoError(t, err)
	buf.Reset()
	require.Zero(t, buf.Len())
}

func TestNewBuffers_invalidDir(t *testing.T) {
	// A path whose parent is a regular file cannot be created as a dir.
	f := filepath.Join(t.TempDir(), "afile")
	require.NoError(t, ioz.WriteFileAtomic(f, []byte("x"), ioz.RWPerms))

	_, err := ioz.NewBuffers(filepath.Join(f, "subdir"), 8)
	require.Error(t, err)
}

func TestBuffers_NewMem2Disk_inMemory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bufs")
	bufs, err := ioz.NewBuffers(dir, 1024)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bufs.Close()) })

	buf := bufs.NewMem2Disk()
	t.Cleanup(func() { require.NoError(t, buf.Close()) })

	const val = "small payload that stays in memory"
	n, err := buf.Write([]byte(val))
	require.NoError(t, err)
	require.Equal(t, len(val), n)
	require.Equal(t, int64(len(val)), buf.Len())

	got, err := io.ReadAll(buf)
	require.NoError(t, err)
	require.Equal(t, val, string(got))
}

func TestBuffers_NewMem2Disk_spillToDisk(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bufs")
	const memBufSize = 16
	bufs, err := ioz.NewBuffers(dir, memBufSize)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bufs.Close()) })

	buf := bufs.NewMem2Disk()
	t.Cleanup(func() { require.NoError(t, buf.Close()) })

	// Write more than memBufSize so the buffer overflows to a file.
	want := strings.Repeat("x", memBufSize*4)
	n, err := buf.Write([]byte(want))
	require.NoError(t, err)
	require.Equal(t, len(want), n)
	require.Equal(t, int64(len(want)), buf.Len())

	// The overflow must actually spill to a backing file in dir; otherwise this
	// test would pass even if the payload stayed entirely in memory.
	spillEntries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.NotEmpty(t, spillEntries, "payload exceeding memBufSize must spill to a file on disk")
	// Cap on a spilled buffer reports the file buffer's capacity.
	require.Positive(t, buf.Cap())

	got, err := io.ReadAll(buf)
	require.NoError(t, err)
	require.Equal(t, want, string(got))

	// Reset empties the (spilled) buffer.
	buf.Reset()
	require.Zero(t, buf.Len())
}

func TestBuffers_NewMem2Disk_closeWithoutRead(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bufs")
	bufs, err := ioz.NewBuffers(dir, 1024)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bufs.Close()) })

	// Write a small payload and close without ever reading, so the lazy
	// file-backed buffer is never initialized (Close's nil-fileBuf path).
	buf := bufs.NewMem2Disk()
	_, err = buf.Write([]byte("tiny"))
	require.NoError(t, err)
	require.NoError(t, buf.Close())
}

func TestBuffers_Close_removesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bufs")
	bufs, err := ioz.NewBuffers(dir, 8)
	require.NoError(t, err)
	require.True(t, ioz.DirExists(dir))

	require.NoError(t, bufs.Close())
	require.False(t, ioz.DirExists(dir), "Close must remove the buffer dir")
}

func TestBuffers_NewMem2Disk_noFileUntilSpill(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bufs")
	bufs, err := ioz.NewBuffers(dir, 64)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bufs.Close()) })

	buf := bufs.NewMem2Disk()
	t.Cleanup(func() { require.NoError(t, buf.Close()) })

	// A payload smaller than memBufSize stays in memory. Measuring (Len) and
	// reading must not force creation of a backing file.
	_, err = buf.Write([]byte("small"))
	require.NoError(t, err)
	require.Equal(t, int64(5), buf.Len())
	// Cap reflects the would-be spill capacity without forcing a file.
	require.Positive(t, buf.Cap())

	got, err := io.ReadAll(buf)
	require.NoError(t, err)
	require.Equal(t, "small", string(got))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries, "an unspilled buffer must not create a backing file")
}

func TestBuffers_NewMem2Disk_spillErrorSurfaced(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bufs")
	bufs, err := ioz.NewBuffers(dir, 16)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bufs.Close()) })

	buf := bufs.NewMem2Disk()
	t.Cleanup(func() { require.NoError(t, buf.Close()) })

	// Remove the buffer dir so the file pool can't create a spill file.
	require.NoError(t, os.RemoveAll(dir))

	// Writing past memBufSize must return an error, not panic.
	require.NotPanics(t, func() {
		_, err = buf.Write([]byte(strings.Repeat("x", 64)))
	})
	require.Error(t, err)

	// A subsequent spill write returns the same cached error (no retry/panic).
	_, err2 := buf.Write([]byte(strings.Repeat("y", 64)))
	require.Error(t, err2)
	require.Equal(t, err.Error(), err2.Error(), "the cached init error should be returned, not a fresh one")
}

func TestBuffer_postCloseSafe(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bufs")
	bufs, err := ioz.NewBuffers(dir, 16)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bufs.Close()) })

	factories := map[string]func() ioz.Buffer{
		"default":  ioz.NewDefaultBuffer,
		"mem2disk": bufs.NewMem2Disk,
	}
	for name, factory := range factories {
		t.Run(name, func(t *testing.T) {
			buf := factory()
			_, err := buf.Write([]byte("data"))
			require.NoError(t, err)
			require.NoError(t, buf.Close())

			// Post-close calls must be safe (no panic) and consistent.
			require.NotPanics(t, func() {
				buf.Reset()
				require.Zero(t, buf.Len())
				require.Zero(t, buf.Cap())
				n, rerr := buf.Read(make([]byte, 4))
				require.Zero(t, n)
				require.ErrorIs(t, rerr, io.EOF)
			})
			// Write after close returns an error rather than silently accepting.
			_, err = buf.Write([]byte("x"))
			require.Error(t, err)

			// Close is idempotent.
			require.NoError(t, buf.Close())
		})
	}
}

func TestBuffers_multipleBuffers(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bufs")
	bufs, err := ioz.NewBuffers(dir, 4)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bufs.Close()) })

	const n = 5
	for i := range n {
		buf := bufs.NewMem2Disk()
		payload := bytes.Repeat([]byte{byte('a' + i)}, 32) // exceeds memBufSize
		_, err := buf.Write(payload)
		require.NoError(t, err)

		got, err := io.ReadAll(buf)
		require.NoError(t, err)
		require.Equal(t, payload, got)
		require.NoError(t, buf.Close())
	}
}
