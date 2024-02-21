package diff_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/stretchr/testify/require"
)

func TestSchemaDiff(t *testing.T) {
	th := testh.New(t)

	tr := testrun.New(th.Context, t, nil)

	tr.Add(
		source.Source{
			Handle:   "@test_a",
			Type:     drivertype.SQLite,
			Location: "sqlite3:///Users/neilotoole/work/sq/sq/cli/diff/testdata/sakila_a.db",
		},
		source.Source{
			Handle:   "@test_b",
			Type:     drivertype.SQLite,
			Location: "sqlite3:///Users/neilotoole/work/sq/sq/cli/diff/testdata/sakila_b.db",
		},
	)

	err := tr.Reset().Exec("diff", "@test_a", "@test_b", "--schema")

	require.NoError(t, err)
	fmt.Fprintln(os.Stdout, tr.Out.String())
}
