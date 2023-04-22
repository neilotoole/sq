package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"image/gif"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestSmoke(t *testing.T) {
	t.Parallel()
	// Execute a bunch of smoke test cases.

	testCases := []struct {
		a []string
		// errBecause, if non-empty, indicates an error is expected.
		errBecause string
	}{
		{a: []string{"ls"}},
		{a: []string{"ls", "-v"}},
		{a: []string{"ls", "--help"}},
		{a: []string{"inspect"}, errBecause: "no active data source"},
		{a: []string{"inspect", "--help"}},
		{a: []string{"version"}},
		{a: []string{"--version"}},
		{a: []string{"help"}},
		{a: []string{"--help"}},
		{a: []string{"ping", "/"}},
		{a: []string{"ping", "--help"}},
		{a: []string{"ping"}, errBecause: "no active data source"},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(strings.Join(tc.a, "_"), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			rc, out, errOut := newTestRunCtx(ctx, t, nil)
			err := cli.ExecuteWith(ctx, rc, tc.a)

			// We log sq's output before doing assert, because it reads
			// better in testing's output that way.
			if out.Len() > 0 {
				t.Log(strings.TrimSuffix(out.String(), "\n"))
			}
			if errOut.Len() > 0 {
				t.Log(strings.TrimSuffix(errOut.String(), "\n"))
			}

			if tc.errBecause != "" {
				assert.Error(t, err, tc.errBecause)
			} else {
				assert.NoError(t, err, tc.errBecause)
			}
		})
	}
}

func TestCreateTblTestBytes(t *testing.T) {
	th, src, _, _ := testh.NewWith(t, sakila.Pg9)
	th.DiffDB(src)

	tblDef := sqlmodel.NewTableDef(
		stringz.UniqTableName("test_bytes"),
		[]string{"col_name", "col_bytes"},
		[]kind.Kind{kind.Text, kind.Bytes},
	)

	fBytes := proj.ReadFile(fixt.GopherPath)
	data := []any{fixt.GopherFilename, fBytes}

	require.Equal(t, int64(1), th.CreateTable(true, src, tblDef, data))
	th.DropTable(src, tblDef.Name)
}

// TestOutputRaw verifies that the raw output format works.
// We're particularly concerned that bytes output is correct.
func TestOutputRaw(t *testing.T) {
	t.Parallel()

	for _, handle := range sakila.SQLAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			// Sanity check
			wantBytes := proj.ReadFile(fixt.GopherPath)
			require.Equal(t, fixt.GopherSize, len(wantBytes))
			_, err := gif.Decode(bytes.NewReader(wantBytes))
			require.NoError(t, err)

			tblDef := sqlmodel.NewTableDef(
				stringz.UniqTableName("test_bytes"),
				[]string{"col_name", "col_bytes"},
				[]kind.Kind{kind.Text, kind.Bytes},
			)

			th, src, _, _ := testh.NewWith(t, handle)

			// Create the table and insert data
			insertRow := []any{fixt.GopherFilename, wantBytes}
			require.Equal(t, int64(1), th.CreateTable(true, src, tblDef, insertRow))
			defer th.DropTable(src, tblDef.Name)

			// 1. Query and check that libsq is returning bytes correctly.
			query := fmt.Sprintf("SELECT col_bytes FROM %s WHERE col_name = '%s'",
				tblDef.Name, fixt.GopherFilename)
			sink, err := th.QuerySQL(src, query)
			require.NoError(t, err)

			require.Equal(t, 1, len(sink.Recs))
			require.Equal(t, kind.Bytes, sink.RecMeta[0].Kind())
			dbBytes := *(sink.Recs[0][0].(*[]byte))
			require.Equal(t, fixt.GopherSize, len(dbBytes))
			require.Equal(t, wantBytes, dbBytes)

			// 1. Now that we've verified libsq, we'll test cli. First
			// using using --output=/path/to/file
			tmpDir, err := os.MkdirTemp("", "")
			require.NoError(t, err)
			outputPath := filepath.Join(tmpDir, "gopher.gif")
			t.Cleanup(func() {
				os.RemoveAll(outputPath)
			})

			ru := newRun(th.Context, t, nil).add(*src).hush()
			err = ru.Exec("sql", "--raw", "--output="+outputPath, query)
			require.NoError(t, err)

			outputBytes, err := os.ReadFile(outputPath)
			require.NoError(t, err)
			require.Equal(t, fixt.GopherSize, len(outputBytes))
			_, err = gif.Decode(bytes.NewReader(outputBytes))
			require.NoError(t, err)

			// 2. Now test that stdout also gets the same data
			ru = newRun(th.Context, t, nil).add(*src).hush()
			err = ru.Exec("sql", "--raw", query)
			require.NoError(t, err)
			require.Equal(t, wantBytes, ru.out.Bytes())
		})
	}
}
