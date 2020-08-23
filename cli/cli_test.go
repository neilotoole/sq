package cli_test

import (
	"bytes"
	"fmt"
	"image/gif"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neilotoole/lg/testlg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/sqlmodel"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestSmoke(t *testing.T) {
	t.Parallel()
	// Execute a bunch of smoke test cases.

	sqargs := func(a ...string) []string {
		return append([]string{"sq"}, a...)
	}

	testCases := []struct {
		a []string
		// errBecause, if non-empty, indicates an error is expected.
		errBecause string
	}{
		{a: sqargs("ls")},
		{a: sqargs("ls", "-v")},
		{a: sqargs("ls", "--help")},
		{a: sqargs("inspect"), errBecause: "no active data source"},
		{a: sqargs("inspect", "--help")},
		{a: sqargs("version")},
		{a: sqargs("--version")},
		{a: sqargs("help")},
		{a: sqargs("--help")},
		{a: sqargs("ping", "all")},
		{a: sqargs("ping", "--help")},
		{a: sqargs("ping"), errBecause: "no active data source"},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(strings.Join(tc.a, "_"), func(t *testing.T) {
			t.Parallel()

			rc, out, errOut := newTestRunCtx(testlg.New(t))
			err := cli.ExecuteWith(rc, tc.a)

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
		[]sqlz.Kind{sqlz.KindText, sqlz.KindBytes},
	)

	fBytes := proj.ReadFile(fixt.GopherPath)
	data := []interface{}{fixt.GopherFilename, fBytes}

	require.Equal(t, int64(1), th.CreateTable(true, src, tblDef, data))
}

// TestOutputRaw verifies that the raw output format works.
// We're particularly concerned that bytes output is correct.
func TestOutputRaw(t *testing.T) {
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
				[]sqlz.Kind{sqlz.KindText, sqlz.KindBytes},
			)

			th, src, _, _ := testh.NewWith(t, handle)

			// Create the table and insert data
			insertRow := []interface{}{fixt.GopherFilename, wantBytes}
			require.Equal(t, int64(1), th.CreateTable(true, src, tblDef, insertRow))

			// 1. Query and check that libsq is returning bytes correctly.
			query := fmt.Sprintf("SELECT col_bytes FROM %s WHERE col_name = '%s'",
				tblDef.Name, fixt.GopherFilename)
			sink, err := th.QuerySQL(src, query)
			require.NoError(t, err)

			require.Equal(t, 1, len(sink.Recs))
			require.Equal(t, sqlz.KindBytes, sink.RecMeta[0].Kind())
			dbBytes := *(sink.Recs[0][0].(*[]byte))
			require.Equal(t, fixt.GopherSize, len(dbBytes))
			require.Equal(t, wantBytes, dbBytes)

			// 1. Now that we've verified libsq, we'll test cli. First
			// using using --output=/path/to/file
			tmpDir, err := ioutil.TempDir("", "")
			require.NoError(t, err)
			outputPath := filepath.Join(tmpDir, "gopher.gif")
			t.Cleanup(func() {
				os.RemoveAll(outputPath)
			})

			ru := newRun(t).add(*src).hush()
			err = ru.exec("sql", "--raw", "--output="+outputPath, query)
			require.NoError(t, err)

			outputBytes, err := ioutil.ReadFile(outputPath)
			require.NoError(t, err)
			require.Equal(t, fixt.GopherSize, len(outputBytes))
			_, err = gif.Decode(bytes.NewReader(outputBytes))
			require.NoError(t, err)

			// 2. Now test that stdout also gets the same data
			ru = newRun(t).add(*src)
			err = ru.exec("sql", "--raw", query)
			require.NoError(t, err)
			require.Equal(t, wantBytes, ru.out.Bytes())
		})
	}
}
