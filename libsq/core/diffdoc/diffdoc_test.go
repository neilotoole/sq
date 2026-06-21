package diffdoc_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// readAllByChunks reads r in small fixed-size chunks, exercising the Read path
// with buffers smaller than the content. It returns the accumulated bytes.
func readAllByChunks(t *testing.T, r io.Reader, chunk int) []byte {
	t.Helper()
	var got []byte
	buf := make([]byte, chunk)
	zeroReads := 0
	for {
		n, err := r.Read(buf)
		got = append(got, buf[:n]...)
		if err == io.EOF {
			return got
		}
		require.NoError(t, err)
		// Fail fast rather than spin forever if a (buggy) reader keeps
		// returning (0, nil) for a non-empty buffer.
		if n == 0 {
			zeroReads++
			require.Less(t, zeroReads, 100, "reader returned (0, nil) repeatedly")
		} else {
			zeroReads = 0
		}
	}
}

// TestHunkDoc_ReadRoundTrip is the regression test for the bug where
// HunkDoc.Read emitted the full peek buffer (zero-padded) instead of only the
// bytes actually read, corrupting output with NUL bytes.
func TestHunkDoc_ReadRoundTrip(t *testing.T) {
	const (
		title = "TITLE\n"
		hdr   = "HEADER\n"
	)
	newDoc := func() *diffdoc.HunkDoc {
		doc := diffdoc.NewHunkDoc(diffdoc.Title(title), []byte(hdr))
		h1, err := doc.NewHunk(1)
		require.NoError(t, err)
		_, err = h1.Write([]byte(" ctx\n-old\n+new\n"))
		require.NoError(t, err)
		h1.Seal([]byte("@@ -1,2 +1,2 @@\n"), nil)

		h2, err := doc.NewHunk(10)
		require.NoError(t, err)
		_, err = h2.Write([]byte(" more\n"))
		require.NoError(t, err)
		h2.Seal([]byte("@@ -10,1 +10,1 @@\n"), nil)

		doc.Seal(nil)
		return doc
	}

	const want = title + hdr +
		"@@ -1,2 +1,2 @@\n ctx\n-old\n+new\n" +
		"@@ -10,1 +10,1 @@\n more\n"

	// io.ReadAll uses a large internal buffer; the original bug surfaced here.
	got, err := io.ReadAll(newDoc())
	require.NoError(t, err)
	require.Equal(t, want, string(got))
	require.NotContains(t, string(got), "\x00", "output must not contain NUL padding")

	// Reading in tiny chunks must produce the identical result.
	got = readAllByChunks(t, newDoc(), 3)
	require.Equal(t, want, string(got))
	require.NotContains(t, string(got), "\x00")
}

// TestHunkDoc_ZeroLengthRead guards against a regression where a zero-length
// first read ran the internal peek, observed a zero-byte read, and permanently
// poisoned the doc with an "unexpected zero read" error.
func TestHunkDoc_ZeroLengthRead(t *testing.T) {
	doc := diffdoc.NewHunkDoc(diffdoc.Title("T\n"), []byte("H\n"))
	h, err := doc.NewHunk(1)
	require.NoError(t, err)
	_, err = h.Write([]byte("body\n"))
	require.NoError(t, err)
	h.Seal([]byte("@@ -1 +1 @@\n"), nil)
	doc.Seal(nil)

	// A zero-length read must be a no-op, not poison the doc.
	n, err := doc.Read([]byte{})
	require.NoError(t, err)
	require.Zero(t, n)

	// A subsequent real read must still return the full content.
	got, err := io.ReadAll(doc)
	require.NoError(t, err)
	require.Equal(t, "T\nH\n@@ -1 +1 @@\nbody\n", string(got))
}

func TestHunkDoc_Empty(t *testing.T) {
	doc := diffdoc.NewHunkDoc(diffdoc.Title("TITLE\n"), []byte("HEADER\n"))
	doc.Seal(nil)

	got, err := io.ReadAll(doc)
	require.NoError(t, err)
	require.Empty(t, got, "a doc with no hunks emits nothing, not the title/header")
}

func TestHunkDoc_SealError(t *testing.T) {
	wantErr := errz.New("boom")
	doc := diffdoc.NewHunkDoc(diffdoc.Title("TITLE\n"), nil)
	doc.Seal(wantErr)

	require.ErrorIs(t, doc.Err(), wantErr)
	_, err := io.ReadAll(doc)
	require.ErrorIs(t, err, wantErr)
}

func TestHunkDoc_DoubleSealPanics(t *testing.T) {
	doc := diffdoc.NewHunkDoc(nil, nil)
	doc.Seal(nil)
	require.Panics(t, func() { doc.Seal(nil) })
}

func TestHunkDoc_NewHunkAfterSeal(t *testing.T) {
	doc := diffdoc.NewHunkDoc(nil, nil)
	doc.Seal(nil)
	_, err := doc.NewHunk(1)
	require.Error(t, err)
}

func TestHunk_DoubleSealPanics(t *testing.T) {
	doc := diffdoc.NewHunkDoc(nil, nil)
	h, err := doc.NewHunk(1)
	require.NoError(t, err)
	h.Seal(nil, nil)
	require.Panics(t, func() { h.Seal(nil, nil) })
}

func TestUnifiedDoc_ReadRoundTrip(t *testing.T) {
	const (
		title = "TITLE\n"
		body  = "--- a\n+++ b\n@@ -1 +1 @@\n-x\n+y\n"
	)
	doc := diffdoc.NewUnifiedDoc(diffdoc.Title(title))
	_, err := doc.Write([]byte(body))
	require.NoError(t, err)
	doc.Seal(nil)

	got, err := io.ReadAll(doc)
	require.NoError(t, err)
	require.Equal(t, title+body, string(got))
}

func TestUnifiedDoc_Empty(t *testing.T) {
	doc := diffdoc.NewUnifiedDoc(diffdoc.Title("TITLE\n"))
	doc.Seal(nil)

	got, err := io.ReadAll(doc)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestUnifiedDoc_CloseIdempotent(t *testing.T) {
	doc := diffdoc.NewUnifiedDoc(diffdoc.Title("TITLE\n"))
	_, err := doc.Write([]byte("body\n"))
	require.NoError(t, err)
	doc.Seal(nil)

	require.NoError(t, doc.Close())
	// A second Close must not panic and must return the same (nil) error.
	require.NotPanics(t, func() {
		require.NoError(t, doc.Close())
	})
}

// TestColorize_MalformedSection guards against the out-of-range panic on a
// line that starts with "@@ " but has no closing "@@".
func TestColorize_MalformedSection(t *testing.T) {
	clrs := diffdoc.NewColors()
	clrs.EnableColor(true)

	r := diffdoc.NewColorizer(context.Background(), clrs, strings.NewReader("@@ x\n"))
	require.NotPanics(t, func() {
		_, err := io.ReadAll(r)
		require.NoError(t, err)
	})
}

// TestExecute_NilDiffer ensures Execute (and its deferred cleanup) tolerates
// nil entries in the differs slice.
func TestExecute_NilDiffer(t *testing.T) {
	doc := diffdoc.NewUnifiedDoc(diffdoc.Title("TITLE\n"))
	_, err := doc.Write([]byte("body\n"))
	require.NoError(t, err)
	doc.Seal(nil)

	differ := diffdoc.NewDiffer(doc, func(_ context.Context, _ func(error)) {})
	differs := []*diffdoc.Differ{nil, differ, nil}

	buf := &bytes.Buffer{}
	hasDiffs, err := diffdoc.Execute(context.Background(), buf, 0, differs)
	require.NoError(t, err)
	require.True(t, hasDiffs)
	require.Equal(t, "TITLE\nbody\n", buf.String())
}
