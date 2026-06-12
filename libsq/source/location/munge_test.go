package location_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/location"
)

func TestMungeForDriver_EmptyPath(t *testing.T) {
	testCases := []struct {
		name string
		typ  drivertype.Type
		loc  string
	}{
		{name: "sqlite3 prefix only", typ: drivertype.SQLite, loc: "sqlite3://"},
		{name: "sqlite3 bare scheme only", typ: drivertype.SQLite, loc: "sqlite3:"},
		{name: "sqlite3 empty path with query", typ: drivertype.SQLite, loc: "sqlite3://?mode=ro"},
		{name: "duckdb prefix only", typ: drivertype.DuckDB, loc: "duckdb://"},
		{name: "duckdb empty path with query", typ: drivertype.DuckDB, loc: "duckdb://?access_mode=READ_ONLY"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := location.MungeForDriver(tc.typ, tc.loc)
			require.Error(t, err)
			require.NotContains(t, err.Error(), tc.loc,
				"error must not echo the location")
		})
	}

	// The :memory: sentinel is not an empty path.
	got, err := location.MungeForDriver(drivertype.DuckDB, "duckdb://:memory:")
	require.NoError(t, err)
	require.Equal(t, "duckdb://:memory:", got)
}
