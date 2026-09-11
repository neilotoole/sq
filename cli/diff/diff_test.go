package diff_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// TestDiff_Data_ReadOnly is a regression guard for the diff --data path
// opening sources read-only. The table-data comparison runs SLQ via a
// run.QueryContext; if its AccessMode is left at the ModeReadWrite
// default, DuckDB sources are opened read-write and take a write lock.
//
// The test makes the DuckDB file read-only on disk (0444), so a
// read-write open fails (DuckDB can't take a write lock on it) while a
// read-only open succeeds. diff is wholly read-only, so it must succeed.
func TestDiff_Data_ReadOnly(t *testing.T) {
	tu.SkipReadOnlyFileUnenforceable(t)
	th := testh.New(t)
	src := th.Source(sakila.Duck)
	path := strings.TrimPrefix(src.Location, "duckdb://")

	// th.Source returns a per-test copy, so chmod-ing it is safe.
	require.NoError(t, os.Chmod(path, 0o444))
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) }) // let TempDir cleanup remove it

	// Diff a table against itself: no differences (clean exit), but the
	// data path still opens the source and runs the row queries. With a
	// read-write open this fails on the 0444 file; read-only succeeds.
	tr := testrun.New(th.Context, t, nil).Hush().Add(*src)
	require.NoError(t, tr.Exec("diff", "--data",
		src.Handle+".actor", src.Handle+".actor"),
		"sq diff --data must open the source read-only (no write lock)")
}

func TestSchemaDiff(t *testing.T) {
	th := testh.New(t)

	tr := testrun.New(th.Context, t, nil)
	_ = tr.Reset().Exec("config", "set", "debug.progress.force", "true")

	tr = tr.Add(
		source.Source{
			Handle:   "@test_a",
			Type:     drivertype.SQLite,
			Location: "sqlite3://" + proj.Abs("cli/diff/testdata/sakila_a.db"),
		},
		source.Source{
			Handle:   "@test_b",
			Type:     drivertype.SQLite,
			Location: "sqlite3://" + proj.Abs("cli/diff/testdata/sakila_b.db"),
		},
	)

	err := tr.Reset().Exec("diff", "@test_a", "@test_b", "--schema")

	require.Error(t, err)
	require.Equal(t, 1, errz.ExitCode(err), "should be exit code 1 on differences")
	fmt.Fprintln(os.Stdout, tr.Out.String())
}

// TestDiff_unsupportedFormat is a regression guard for the format validation
// that moved out of OptDiffDataFormat's (globally-shared) validFn into
// getDiffRecordWriter. diff renders data only for text-based formats; xlsx,
// raw, and the inspect-only mermaid-erd format must still be rejected with a
// clear error.
func TestDiff_unsupportedFormat(t *testing.T) {
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil).Add(
		source.Source{
			Handle:   "@test_a",
			Type:     drivertype.SQLite,
			Location: "sqlite3://" + proj.Abs("cli/diff/testdata/sakila_a.db"),
		},
		source.Source{
			Handle:   "@test_b",
			Type:     drivertype.SQLite,
			Location: "sqlite3://" + proj.Abs("cli/diff/testdata/sakila_b.db"),
		},
	)

	for _, fm := range []string{"xlsx", "raw", "mermaid-erd"} {
		t.Run(fm, func(t *testing.T) {
			err := tr.Reset().Exec("diff", "@test_a", "@test_b", "--data", "-f", fm)
			require.Error(t, err)
			require.Contains(t, err.Error(), "diff does not support output format")
		})
	}
}
