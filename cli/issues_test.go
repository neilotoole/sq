package cli_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/ioz/scannerz"
)

// See: https://github.com/neilotoole/sq/issues/446.
func TestGitHubIssue446_ScannerErrTooLong(t *testing.T) {
	// 1. Create a JSONL file with large tokens.
	// 2. Set the config for scanner buffer size to a known value (which will be too small).
	// 3. Attempt to ingest the JSONL file, which should fail with bufio.ErrTooLong.
	// 4. Set the config for scanner buffer size to a larger value.
	// 5. Attempt to ingest the JSONL file, which should succeed.

	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "test.jsonl"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	blob := generateJSONLinesBlobWithLargeTokens(bufio.MaxScanTokenSize+100, 5)
	_, err = f.Write(blob)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	tr := testrun.New(context.Background(), t, nil).Hush()
	require.NoError(t, tr.Exec(
		"config",
		"set",
		scannerz.OptScanBufLimit.Key(),
		strconv.Itoa(bufio.MaxScanTokenSize),
	))

	const handle = "@test/large_jsonl"
	err = tr.Reset().Exec("add", f.Name(), "--handle", handle)
	require.Error(t, err, "should fail with bufio.ErrTooLong")
	require.True(t, errors.Is(err, bufio.ErrTooLong))

	require.NoError(t, tr.Reset().Exec(
		"config",
		"set",
		scannerz.OptScanBufLimit.Key(),
		"10MB",
	))

	err = tr.Reset().Exec("add", f.Name(), "--handle", handle)
	require.NoError(t, err, "should succeed with increased buffer size")

	// Extra check to verify that ingest worked as expected.
	require.NoError(t, tr.Reset().Exec("inspect", "--json", handle))
	require.Equal(t, handle, tr.JQ(".handle"))
	require.Equal(t, "id", tr.JQ(".tables[0].columns[0].name"))
}

func generateJSONLinesBlobWithLargeTokens(tokenSize, lines int) []byte {
	buf := &bytes.Buffer{}
	for i := 0; i < lines; i++ {
		buf.WriteString(`{"id": "` + strconv.Itoa(i) + `", "name": "`)
		buf.WriteString(strings.Repeat("x", tokenSize))
		buf.WriteString(`"}`)
		buf.WriteString("\n")
	}
	return buf.Bytes()
}
