package drivertype_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source/drivertype"
)

func TestType_String(t *testing.T) {
	testCases := []struct {
		typ  drivertype.Type
		want string
	}{
		{drivertype.None, ""},
		{drivertype.SQLite, "sqlite3"},
		{drivertype.Pg, "postgres"},
		{drivertype.MSSQL, "sqlserver"},
		{drivertype.MySQL, "mysql"},
		{drivertype.ClickHouse, "clickhouse"},
		{drivertype.CSV, "csv"},
		{drivertype.TSV, "tsv"},
		{drivertype.JSON, "json"},
		{drivertype.JSONA, "jsona"},
		{drivertype.JSONL, "jsonl"},
		{drivertype.XLSX, "xlsx"},
	}

	for _, tc := range testCases {
		t.Run(tc.want, func(t *testing.T) {
			got := tc.typ.String()
			require.Equal(t, tc.want, got)
		})
	}
}

func TestType_Constants(t *testing.T) {
	// Verify that each constant has the expected underlying string value.
	// This ensures the constants don't accidentally change.
	require.Equal(t, drivertype.Type(""), drivertype.None)
	require.Equal(t, drivertype.Type("sqlite3"), drivertype.SQLite)
	require.Equal(t, drivertype.Type("postgres"), drivertype.Pg)
	require.Equal(t, drivertype.Type("sqlserver"), drivertype.MSSQL)
	require.Equal(t, drivertype.Type("mysql"), drivertype.MySQL)
	require.Equal(t, drivertype.Type("clickhouse"), drivertype.ClickHouse)
	require.Equal(t, drivertype.Type("csv"), drivertype.CSV)
	require.Equal(t, drivertype.Type("tsv"), drivertype.TSV)
	require.Equal(t, drivertype.Type("json"), drivertype.JSON)
	require.Equal(t, drivertype.Type("jsona"), drivertype.JSONA)
	require.Equal(t, drivertype.Type("jsonl"), drivertype.JSONL)
	require.Equal(t, drivertype.Type("xlsx"), drivertype.XLSX)
}

func TestType_Equality(t *testing.T) {
	// Verify that Type can be compared for equality.
	typ := drivertype.Pg
	require.True(t, typ == drivertype.Pg)
	require.False(t, typ == drivertype.MySQL)
	require.True(t, drivertype.None == drivertype.Type(""))
}

func TestType_ZeroValue(t *testing.T) {
	var typ drivertype.Type
	require.Equal(t, drivertype.None, typ)
	require.Equal(t, "", typ.String())
}
