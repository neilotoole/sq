package rqlite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/testh/tu"
)

// Exported for external_test consumers in drivers/rqlite/*_test.go.
var (
	KindFromDBTypeName = kindFromDBTypeName
	RTypeNullTime      = rtypeNullTime
)

func TestPlaceholders(t *testing.T) {
	testCases := []struct {
		numCols int
		numRows int
		want    string
	}{
		{numCols: 0, numRows: 0, want: ""},
		{numCols: 1, numRows: 1, want: "(?)"},
		{numCols: 2, numRows: 1, want: "(?, ?)"},
		{numCols: 1, numRows: 2, want: "(?), (?)"},
		{numCols: 2, numRows: 2, want: "(?, ?), (?, ?)"},
	}

	for _, tc := range testCases {
		got := placeholders(tc.numCols, tc.numRows)
		require.Equal(t, tc.want, got)
	}
}

func TestDsnFromLocation(t *testing.T) {
	testCases := []struct {
		loc     string
		want    string
		wantErr bool
	}{
		{loc: "", wantErr: true},
		{loc: "sqlite3://foo.db", wantErr: true},
		{loc: "http://host:4001", wantErr: true},
		{loc: Prefix + "host:4001", want: "http://host:4001"},
		{loc: Prefix + "user:pass@host:4001", want: "http://user:pass@host:4001"},
		{loc: Prefix + "host:4001?level=strong", want: "http://host:4001?level=strong"},
		{loc: PrefixSecure + "host:4001", want: "https://host:4001"},
		{loc: PrefixSecure + "user:pass@host:4001?level=none", want: "https://user:pass@host:4001?level=none"},
	}

	for _, tc := range testCases {
		t.Run(tu.Name(tc.loc), func(t *testing.T) {
			got, err := dsnFromLocation(tc.loc)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestDBTypeForKind(t *testing.T) {
	testCases := map[kind.Kind]string{
		kind.Text:     "TEXT",
		kind.Int:      "INTEGER",
		kind.Float:    "REAL",
		kind.Bytes:    "BLOB",
		kind.Decimal:  "NUMERIC",
		kind.Bool:     "BOOLEAN",
		kind.Datetime: "DATETIME",
		kind.Date:     "DATE",
		kind.Time:     "TIME",
		kind.Unknown:  "TEXT",
		kind.Null:     "TEXT",
	}

	for knd, want := range testCases {
		t.Run(knd.String(), func(t *testing.T) {
			require.Equal(t, want, DBTypeForKind(knd))
		})
	}
}

func TestKindFromDBTypeName(t *testing.T) {
	ctx := context.Background()
	// The kind mapping mirrors SQLite affinity rules. These cases cover
	// the common direct matches, the parameterized-suffix stripping, and
	// the fallback affinity branches.
	testCases := map[string]kind.Kind{
		"INTEGER":      kind.Int,
		"BIGINT":       kind.Int,
		"TEXT":         kind.Text,
		"VARCHAR(45)":  kind.Text,
		"BLOB":         kind.Bytes,
		"DATETIME":     kind.Datetime,
		"TIMESTAMP":    kind.Datetime,
		"DATE":         kind.Date,
		"TIME":         kind.Time,
		"BOOLEAN":      kind.Bool,
		"NUMERIC":      kind.Decimal,
		"DECIMAL":      kind.Decimal,
		"REAL":         kind.Float,
		"FLOAT":        kind.Float,
		"INT2":         kind.Int,
		"MEDIUMINT":    kind.Int,
		"NCHAR":        kind.Text,
		"DOUBLE":       kind.Float,
		"someInteger":  kind.Int,  // affinity rule: contains "INT"
		"someText":     kind.Text, // affinity rule: contains "TEXT"
		"longCLOB":     kind.Text, // affinity rule: contains "CLOB"
		"weirdBLOBish": kind.Bytes,
	}
	for dbType, want := range testCases {
		t.Run(dbType, func(t *testing.T) {
			require.Equal(t, want, kindFromDBTypeName(ctx, "col", dbType, nil))
		})
	}
}
