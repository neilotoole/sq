package duckdb_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/duckdb"
)

func TestApplyReadOnlyToLocation(t *testing.T) {
	testCases := []struct {
		name        string
		in          string
		wantOut     string
		wantChanged bool
	}{
		{
			name:        "no_query",
			in:          "duckdb:///path/to/f.duckdb",
			wantOut:     "duckdb:///path/to/f.duckdb?access_mode=READ_ONLY",
			wantChanged: true,
		},
		{
			name:        "other_param",
			in:          "duckdb:///path/to/f.duckdb?threads=4",
			wantOut:     "duckdb:///path/to/f.duckdb?threads=4&access_mode=READ_ONLY",
			wantChanged: true,
		},
		{
			name:        "user_read_write_wins",
			in:          "duckdb:///path/to/f.duckdb?access_mode=READ_WRITE",
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
			name:        "memory_skipped",
			in:          "duckdb://:memory:",
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
			gotOut, gotChanged := duckdb.ApplyReadOnlyToLocation(tc.in)
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
