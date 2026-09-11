package checksum_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
)

// errWriter always fails Write.
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("write boom") }

func TestSum(t *testing.T) {
	got := checksum.Sum(nil)
	require.Equal(t, "", got)
	got = checksum.Sum([]byte{})
	require.Equal(t, "", got)
	got = checksum.Sum([]byte("hello world"))
	assert.Equal(t, "d4a1185", got)
}

func TestChecksums(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "sq-test-*")
	require.NoError(t, err)
	_, err = io.WriteString(f, "huzzah")
	require.NoError(t, err)
	assert.NoError(t, f.Close())

	buf := &bytes.Buffer{}

	gotSum1, err := checksum.ForFile(f.Name())
	require.NoError(t, err)
	t.Logf("gotSum1: %s  %s", gotSum1, f.Name())
	require.NoError(t, checksum.Write(buf, gotSum1, f.Name()))

	gotSums, err := checksum.Read(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	require.Len(t, gotSums, 1)
	require.Equal(t, gotSum1, gotSums[f.Name()])

	// Make some changes to the file and verify that the checksums differ.
	f, err = os.OpenFile(f.Name(), os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = io.WriteString(f, "more huzzah")
	require.NoError(t, err)
	assert.NoError(t, f.Close())
	gotSum2, err := checksum.ForFile(f.Name())
	require.NoError(t, err)
	t.Logf("gotSum2: %s  %s", gotSum2, f.Name())
	require.NoError(t, checksum.Write(buf, gotSum1, f.Name()))
	require.NotEqual(t, gotSum1, gotSum2)
}

func TestSumAll(t *testing.T) {
	// Deterministic for the same inputs.
	require.Equal(t, checksum.SumAll("a", "b", "c"), checksum.SumAll("a", "b", "c"))
	// Single arg works.
	require.NotEmpty(t, checksum.SumAll("solo"))
	// Order/content matters.
	require.NotEqual(t, checksum.SumAll("a", "b"), checksum.SumAll("b", "a"))
	// Note: SumAll concatenates without a delimiter, so SumAll("ab") and
	// SumAll("a","b") intentionally collide (both hash "ab").
	require.Equal(t, checksum.SumAll("ab"), checksum.SumAll("a", "b"))

	// Works for a defined ~string type.
	type myStr string
	require.NotEmpty(t, checksum.SumAll(myStr("x"), myStr("y")))
}

func TestRand(t *testing.T) {
	got := checksum.Rand()
	require.NotEmpty(t, got)
	// Two calls are overwhelmingly unlikely to collide.
	require.NotEqual(t, got, checksum.Rand())
}

func TestWrite_error(t *testing.T) {
	err := checksum.Write(errWriter{}, "abc123", "file.txt")
	require.Error(t, err)
}

func TestWriteFile_and_ReadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checksums.txt")
	require.NoError(t, checksum.WriteFile(path, "3610a686", "file.txt"))

	got, err := checksum.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, map[string]checksum.Checksum{"file.txt": "3610a686"}, got)

	// WriteFile overwrites prior contents.
	require.NoError(t, checksum.WriteFile(path, "deadbeef", "other.txt"))
	got, err = checksum.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, map[string]checksum.Checksum{"other.txt": "deadbeef"}, got)
}

func TestWriteFile_error(t *testing.T) {
	// Parent dir doesn't exist, so the file can't be created.
	path := filepath.Join(t.TempDir(), "no_such_dir", "checksums.txt")
	require.Error(t, checksum.WriteFile(path, "abc", "f.txt"))
}

func TestWriteFile_writeError(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("/dev/full is Linux-only")
	}
	// /dev/full opens fine but every write fails with ENOSPC, exercising the
	// branch where the file opens but Write fails (file is closed, err returned).
	require.Error(t, checksum.WriteFile("/dev/full", "abc", "f.txt"))
}

func TestReadFile_notExist(t *testing.T) {
	_, err := checksum.ReadFile(filepath.Join(t.TempDir(), "nope.txt"))
	require.Error(t, err)
}

func TestRead(t *testing.T) {
	t.Run("comments_and_blanks_ignored", func(t *testing.T) {
		const input = "# a comment\n\n  \n3610a686  file.txt\ndeadbeef  other.txt\n"
		got, err := checksum.Read(strings.NewReader(input))
		require.NoError(t, err)
		require.Equal(t, map[string]checksum.Checksum{
			"file.txt":  "3610a686",
			"other.txt": "deadbeef",
		}, got)
	})

	t.Run("name_with_internal_spaces", func(t *testing.T) {
		got, err := checksum.Read(strings.NewReader("abc123  my file.txt\n"))
		require.NoError(t, err)
		require.Equal(t, checksum.Checksum("abc123"), got["my file.txt"])
	})

	t.Run("invalid_line", func(t *testing.T) {
		_, err := checksum.Read(strings.NewReader("no-double-space-separator\n"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid checksum line")
	})

	t.Run("reader_error", func(t *testing.T) {
		// A reader that errors after yielding a line; the scanner error must surface.
		r := ioz.NewErrorAfterBytesReader([]byte("3610a686  file.txt\n"), errors.New("read boom"))
		_, err := checksum.Read(r)
		require.Error(t, err)
	})

	t.Run("empty_input", func(t *testing.T) {
		got, err := checksum.Read(strings.NewReader(""))
		require.NoError(t, err)
		require.Empty(t, got)
	})
}

func TestForFile_notExist(t *testing.T) {
	_, err := checksum.ForFile(filepath.Join(t.TempDir(), "nope"))
	require.Error(t, err)
}

func TestForFile_dir(t *testing.T) {
	// A directory is a valid target (IsDir branch).
	sum, err := checksum.ForFile(t.TempDir())
	require.NoError(t, err)
	require.NotEmpty(t, sum)
}
