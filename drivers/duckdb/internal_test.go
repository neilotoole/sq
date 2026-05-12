package duckdb

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh/tu"
)

func TestMungeLocation(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	cwd = filepath.ToSlash(cwd)

	root, err := filepath.Abs("/")
	require.NoError(t, err)
	root = filepath.ToSlash(root)

	cwdWant := Prefix + cwd + "/foo.duckdb"

	t.Log("cwdWant:", cwdWant)
	t.Log("root:", root)

	testCases := []struct {
		in        string
		want      string
		onlyForOS string
		wantErr   bool
	}{
		{
			in:      "",
			wantErr: true,
		},
		{
			in:   ":memory:",
			want: Prefix + ":memory:",
		},
		{
			in:   Prefix + ":memory:",
			want: Prefix + ":memory:",
		},
		{
			in:   "duckdb:///path/to/foo.duckdb",
			want: Prefix + root + "path/to/foo.duckdb",
		},
		{
			in:   "duckdb://foo.duckdb",
			want: cwdWant,
		},
		{
			in:   "duckdb:foo.duckdb",
			want: cwdWant,
		},
		{
			in:   "duckdb:/foo.duckdb",
			want: Prefix + root + "foo.duckdb",
		},
		{
			in:   "foo.duckdb",
			want: cwdWant,
		},
		{
			in:   "./foo.duckdb",
			want: cwdWant,
		},
		{
			in:   "/path/to/foo.duckdb",
			want: Prefix + root + "path/to/foo.duckdb",
		},
		{
			in:   `C:/Users/neil/work/sq/drivers/duckdb/testdata/sakila.duckdb`,
			want: `duckdb://C:/Users/neil/work/sq/drivers/duckdb/testdata/sakila.duckdb`,
			// The current impl of MungeLocation relies upon OS-specific functions
			// in pkg filepath. Thus, we skip this test on non-Windows OSes.
			// MungeLocation could probably be rewritten to be OS-independent?
			onlyForOS: "windows",
		},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			if tc.onlyForOS != "" && tc.onlyForOS != runtime.GOOS {
				t.Skipf("Skipping because this test is only for OS {%s}, but have {%s}",
					tc.onlyForOS, runtime.GOOS)
				return
			}

			got, err := MungeLocation(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
