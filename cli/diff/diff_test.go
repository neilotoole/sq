package diff_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
)

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
