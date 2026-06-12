package duckdb_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/duckdb"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

func TestApplyReadOnlyToLocation(t *testing.T) {
	testCases := []struct {
		name        string
		in          string
		explicit    bool
		wantOut     string
		wantChanged bool
	}{
		{
			name:        "no_query_implicit",
			in:          "duckdb:///path/to/f.duckdb",
			wantOut:     "duckdb:///path/to/f.duckdb?access_mode=READ_ONLY",
			wantChanged: true,
		},
		{
			name:        "no_query_explicit",
			in:          "duckdb:///path/to/f.duckdb",
			explicit:    true,
			wantOut:     "duckdb:///path/to/f.duckdb?access_mode=READ_ONLY",
			wantChanged: true,
		},
		{
			name:        "other_param_implicit",
			in:          "duckdb:///path/to/f.duckdb?threads=4",
			wantOut:     "duckdb:///path/to/f.duckdb?threads=4&access_mode=READ_ONLY",
			wantChanged: true,
		},
		{
			name:        "other_param_explicit",
			in:          "duckdb:///path/to/f.duckdb?threads=4",
			explicit:    true,
			wantOut:     "duckdb:///path/to/f.duckdb?threads=4&access_mode=READ_ONLY",
			wantChanged: true,
		},
		{
			name:        "automatic_implicit_unchanged",
			in:          "duckdb:///path/to/f.duckdb?access_mode=AUTOMATIC",
			wantOut:     "duckdb:///path/to/f.duckdb?access_mode=AUTOMATIC",
			wantChanged: false,
		},
		{
			name:        "automatic_explicit_overridden",
			in:          "duckdb:///path/to/f.duckdb?access_mode=AUTOMATIC",
			explicit:    true,
			wantOut:     "duckdb:///path/to/f.duckdb?access_mode=READ_ONLY",
			wantChanged: true,
		},
		{
			name:        "automatic_lowercase_explicit_overridden",
			in:          "duckdb:///path/to/f.duckdb?access_mode=automatic",
			explicit:    true,
			wantOut:     "duckdb:///path/to/f.duckdb?access_mode=READ_ONLY",
			wantChanged: true,
		},
		{
			name:        "automatic_explicit_other_params_preserved",
			in:          "duckdb:///path/to/f.duckdb?threads=4&access_mode=AUTOMATIC",
			explicit:    true,
			wantOut:     "duckdb:///path/to/f.duckdb?access_mode=READ_ONLY&threads=4",
			wantChanged: true,
		},
		{
			name:        "user_read_write_wins_implicit",
			in:          "duckdb:///path/to/f.duckdb?access_mode=READ_WRITE",
			wantOut:     "duckdb:///path/to/f.duckdb?access_mode=READ_WRITE",
			wantChanged: false,
		},
		{
			name:        "user_read_write_wins_explicit",
			in:          "duckdb:///path/to/f.duckdb?access_mode=READ_WRITE",
			explicit:    true,
			wantOut:     "duckdb:///path/to/f.duckdb?access_mode=READ_WRITE",
			wantChanged: false,
		},
		{
			name:        "user_read_only_already_set",
			in:          "duckdb:///path/to/f.duckdb?access_mode=READ_ONLY",
			wantOut:     "duckdb:///path/to/f.duckdb?access_mode=READ_ONLY",
			wantChanged: false,
		},
		{
			name:        "user_read_only_lowercase_already_set_explicit",
			in:          "duckdb:///path/to/f.duckdb?access_mode=read_only",
			explicit:    true,
			wantOut:     "duckdb:///path/to/f.duckdb?access_mode=read_only",
			wantChanged: false,
		},
		{
			name:        "memory_skipped",
			in:          "duckdb://:memory:",
			wantOut:     "duckdb://:memory:",
			wantChanged: false,
		},
		{
			name:        "memory_skipped_explicit",
			in:          "duckdb://:memory:",
			explicit:    true,
			wantOut:     "duckdb://:memory:",
			wantChanged: false,
		},
		{
			name:        "memory_with_query_skipped",
			in:          "duckdb://:memory:?threads=4",
			wantOut:     "duckdb://:memory:?threads=4",
			wantChanged: false,
		},
		{
			name:        "empty_path_memory_skipped",
			in:          "duckdb://",
			wantOut:     "duckdb://",
			wantChanged: false,
		},
		{
			name:        "empty_path_memory_skipped_explicit",
			in:          "duckdb://",
			explicit:    true,
			wantOut:     "duckdb://",
			wantChanged: false,
		},
		{
			name:        "empty_path_memory_with_query_skipped",
			in:          "duckdb://?threads=4",
			wantOut:     "duckdb://?threads=4",
			wantChanged: false,
		},
		{
			name:        "non_duckdb_scheme_passthrough",
			in:          "sqlite3:///foo.db",
			wantOut:     "sqlite3:///foo.db",
			wantChanged: false,
		},
		{
			name:        "empty",
			in:          "",
			wantOut:     "",
			wantChanged: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotOut, gotChanged := duckdb.ApplyReadOnlyToLocation(tc.in, tc.explicit)
			require.Equal(t, tc.wantOut, gotOut)
			require.Equal(t, tc.wantChanged, gotChanged)
		})
	}
}

func TestExplicitAccessMode(t *testing.T) {
	testCases := []struct {
		name     string
		in       string
		wantMode string
		wantOK   bool
	}{
		{name: "no_query", in: "duckdb:///f.duckdb", wantOK: false},
		{name: "other_param", in: "duckdb:///f.duckdb?threads=4", wantOK: false},
		{name: "read_only", in: "duckdb:///f.duckdb?access_mode=READ_ONLY", wantMode: "READ_ONLY", wantOK: true},
		{name: "read_write", in: "duckdb:///f.duckdb?access_mode=READ_WRITE", wantMode: "READ_WRITE", wantOK: true},
		{name: "mixed_case", in: "duckdb:///f.duckdb?access_mode=read_write", wantMode: "read_write", wantOK: true},
		{name: "empty", in: "", wantOK: false},
		{name: "non_duckdb", in: "sqlite3:///f.db?access_mode=READ_ONLY", wantOK: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotMode, gotOK := duckdb.ExplicitAccessMode(tc.in)
			require.Equal(t, tc.wantOK, gotOK)
			require.Equal(t, tc.wantMode, gotMode)
		})
	}
}

// TestDoOpen_HonorsReadOnlyContext is a smoke check that doOpen consults
// the ctx hint and that the DuckDB engine accepts the rewritten DSN.
// It opens a nonexistent path READ_ONLY; the rewritten DSN should error
// cleanly because DuckDB READ_ONLY requires an existing file. We assert
// the file was NOT created (which would happen under READ_WRITE).
func TestDoOpen_HonorsReadOnlyContext(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "doesnotexist.duckdb")
	src := &source.Source{
		Handle:   "@test_ro",
		Type:     drivertype.DuckDB,
		Location: "duckdb://" + tmp,
	}

	ctx := driver.WithReadOnly(context.Background())

	prov := &duckdb.Provider{}
	drvr, err := prov.DriverFor(drivertype.DuckDB)
	require.NoError(t, err)

	_, openErr := drvr.Open(ctx, src)
	require.Error(t, openErr, "READ_ONLY open of nonexistent file must fail")
	require.NoFileExists(t, tmp,
		"DuckDB must not have created the file when opened READ_ONLY")
}
