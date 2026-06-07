package driver

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// pgShape is the postgres-equivalent shape used in walker tests.
// It mirrors what drivers/postgres/postgres.go will return in PR B.
var pgShape = LocationShape{
	Type:    drivertype.Pg,
	Schemes: []string{"postgres"},
	Segments: []Segment{
		{Kind: SegCredentials, Optional: true},
		{Kind: SegAuthority},
		{Kind: SegPathName, Optional: true, Placeholder: "db"},
		{Kind: SegConnParams, Optional: true},
	},
}

func TestWalk_schemeMatch(t *testing.T) {
	got, err := Walk(pgShape, "postgres://")
	require.NoError(t, err)
	require.Equal(t, "postgres", got.Scheme)
	require.Equal(t, "postgres://", got.Loc)
}

func TestWalk_schemeMismatch(t *testing.T) {
	_, err := Walk(pgShape, "mysql://localhost")
	require.Error(t, err)
}

func TestWalk_credsPartialUser(t *testing.T) {
	got, err := Walk(pgShape, "postgres://alice")
	require.NoError(t, err)
	require.Equal(t, SegCredentials, got.Current)
	require.Empty(t, got.Done)
	require.Equal(t, "alice", got.User)
	require.False(t, got.PassSet)
	require.False(t, got.HasCreds)
}

func TestWalk_credsPartialPass(t *testing.T) {
	got, err := Walk(pgShape, "postgres://alice:")
	require.NoError(t, err)
	require.Equal(t, SegCredentials, got.Current)
	require.Equal(t, "alice", got.User)
	require.True(t, got.PassSet)
	require.Equal(t, "", got.Pass)
}

func TestWalk_credsFullUser(t *testing.T) {
	got, err := Walk(pgShape, "postgres://alice@")
	require.NoError(t, err)
	require.Equal(t, []SegmentKind{SegCredentials}, got.Done)
	require.True(t, got.HasCreds)
	require.Equal(t, "alice", got.User)
}

func TestWalk_credsFullUserPass(t *testing.T) {
	got, err := Walk(pgShape, "postgres://alice:hunter2@")
	require.NoError(t, err)
	require.Equal(t, []SegmentKind{SegCredentials}, got.Done)
	require.True(t, got.HasCreds)
	require.Equal(t, "alice", got.User)
	require.Equal(t, "hunter2", got.Pass)
	require.True(t, got.PassSet)
}

func TestWalk_authPartialHost(t *testing.T) {
	got, err := Walk(pgShape, "postgres://alice@local")
	require.NoError(t, err)
	require.Equal(t, []SegmentKind{SegCredentials}, got.Done)
	require.Equal(t, SegAuthority, got.Current)
	require.Equal(t, "local", got.Hostname)
	require.False(t, got.PortSet)
}

func TestWalk_authHostPort(t *testing.T) {
	got, err := Walk(pgShape, "postgres://alice@localhost:5432")
	require.NoError(t, err)
	require.Equal(t, SegAuthority, got.Current)
	require.Equal(t, "localhost", got.Hostname)
	require.Equal(t, 5432, got.Port)
	require.True(t, got.PortSet)
}

func TestWalk_authBareHost(t *testing.T) {
	// The #743 ambiguous case: no '@', no '/' or '?'. Walker treats
	// as partial credentials, NOT authority.
	got, err := Walk(pgShape, "postgres://localhost")
	require.NoError(t, err)
	require.Equal(t, SegCredentials, got.Current)
	require.Equal(t, "localhost", got.User)
}

func TestWalk_authIPv6(t *testing.T) {
	got, err := Walk(pgShape, "postgres://alice@[::1]:5432")
	require.NoError(t, err)
	require.Equal(t, SegAuthority, got.Current)
	require.Equal(t, "::1", got.Hostname)
	require.Equal(t, 5432, got.Port)
}

func TestWalk_pathNameEmpty(t *testing.T) {
	got, err := Walk(pgShape, "postgres://alice@localhost/")
	require.NoError(t, err)
	require.Contains(t, got.Done, SegAuthority)
	require.Equal(t, SegPathName, got.Current)
	require.Equal(t, "", got.PathName)
}

func TestWalk_pathNamePartial(t *testing.T) {
	got, err := Walk(pgShape, "postgres://alice@localhost/myd")
	require.NoError(t, err)
	require.Equal(t, SegPathName, got.Current)
	require.Equal(t, "myd", got.PathName)
}

func TestWalk_pathNameOptionalSkipped(t *testing.T) {
	// Authority done, then '?' -> path was skipped, in ConnParams.
	got, err := Walk(pgShape, "postgres://alice@localhost?")
	require.NoError(t, err)
	require.Contains(t, got.Done, SegAuthority)
	// SegPathName NOT in Done because user skipped it.
	require.NotContains(t, got.Done, SegPathName)
}

func TestWalk_pathNameTerminated(t *testing.T) {
	// Path followed by '?' exercises the terminator-hit branch:
	// SegPathName is added to Done, and the cursor advances past
	// the path so the next segment can consume the '?'.
	got, err := Walk(pgShape, "postgres://alice@localhost/mydb?")
	require.NoError(t, err)
	require.Contains(t, got.Done, SegPathName)
	require.Equal(t, "mydb", got.PathName)
}

// sqliteShape is the sqlite3-equivalent shape used in walker tests.
var sqliteShape = LocationShape{
	Type:    drivertype.SQLite,
	Schemes: []string{"sqlite3"},
	Segments: []Segment{
		{Kind: SegPathFile},
		{Kind: SegConnParams, Optional: true},
	},
}

// duckdbShape mirrors drivers/duckdb. PathFile Optional for stdin.
var duckdbShape = LocationShape{
	Type:    drivertype.DuckDB,
	Schemes: []string{"duckdb"},
	Segments: []Segment{
		{Kind: SegPathFile, Optional: true},
		{Kind: SegConnParams, Optional: true},
	},
}

func TestWalk_pathFilePartial(t *testing.T) {
	got, err := Walk(sqliteShape, "sqlite3://./foo")
	require.NoError(t, err)
	require.Equal(t, SegPathFile, got.Current)
	require.Equal(t, "./foo", got.PathFile)
}

func TestWalk_pathFileWithQuery(t *testing.T) {
	got, err := Walk(sqliteShape, "sqlite3://./foo.db?")
	require.NoError(t, err)
	require.Contains(t, got.Done, SegPathFile)
	require.Equal(t, "./foo.db", got.PathFile)
}

func TestWalk_pathFileEmptyStdin(t *testing.T) {
	got, err := Walk(duckdbShape, "duckdb://")
	require.NoError(t, err)
	require.Equal(t, SegPathFile, got.Current)
	require.Equal(t, "", got.PathFile)
}

func TestWalk_paramsEmpty(t *testing.T) {
	got, err := Walk(pgShape, "postgres://alice@h/db?")
	require.NoError(t, err)
	require.Equal(t, SegConnParams, got.Current)
	require.Equal(t, "", got.ParamLastKey)
	require.False(t, got.ParamAtValue)
}

func TestWalk_paramsKeyOnly(t *testing.T) {
	got, err := Walk(pgShape, "postgres://alice@h/db?sslm")
	require.NoError(t, err)
	require.Equal(t, SegConnParams, got.Current)
	require.Equal(t, "sslm", got.ParamLastKey)
	require.False(t, got.ParamAtValue)
}

func TestWalk_paramsAtValue(t *testing.T) {
	got, err := Walk(pgShape, "postgres://alice@h/db?sslmode=")
	require.NoError(t, err)
	require.Equal(t, SegConnParams, got.Current)
	require.Equal(t, "sslmode", got.ParamLastKey)
	require.True(t, got.ParamAtValue)
}

func TestWalk_paramsMultipleWithLastEmpty(t *testing.T) {
	got, err := Walk(pgShape, "postgres://alice@h/db?sslmode=require&app=")
	require.NoError(t, err)
	require.Equal(t, SegConnParams, got.Current)
	require.Equal(t, "app", got.ParamLastKey)
	require.True(t, got.ParamAtValue)
	require.Equal(t, "require", got.Params.Get("sslmode"))
}

var sqlserverShape = LocationShape{
	Type:    drivertype.MSSQL,
	Schemes: []string{"sqlserver"},
	Segments: []Segment{
		{Kind: SegCredentials, Optional: true},
		{Kind: SegAuthority},
		{Kind: SegPathName, Optional: true, Placeholder: "instance"},
		{Kind: SegConnParams, Optional: true, LeadingKey: "database"},
	},
}

var rqliteShape = LocationShape{
	Type:    drivertype.Rqlite,
	Schemes: []string{"rqlite", "rqlites"},
	Segments: []Segment{
		{Kind: SegCredentials, Optional: true},
		{Kind: SegAuthority},
		{Kind: SegConnParams, Optional: true},
	},
}

// TestWalk_gh743BareHost covers issue #743: bare-host URLs (no
// user@) with a trailing '?' must reach SegConnParams, not stall
// in SegCredentials.
func TestWalk_gh743BareHost(t *testing.T) {
	cases := []struct {
		name  string
		shape LocationShape
		loc   string
	}{
		{"pg_bare_host_port_q", pgShape, "postgres://localhost:5432?"},
		{"sqlserver_bare_host_port_q", sqlserverShape, "sqlserver://localhost:1433?"},
		{"rqlite_bare_host_port_q", rqliteShape, "rqlite://localhost:4001?"},
		{"rqlites_bare_host_port_q", rqliteShape, "rqlites://localhost:4001?"},
		{"pg_bare_host_only_q", pgShape, "postgres://localhost?"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Walk(tc.shape, tc.loc)
			require.NoError(t, err)
			require.Equal(t, SegConnParams, got.Current,
				"bug #743: should be in SegConnParams, not %v", got.Current)
			require.NotContains(t, got.Done, SegCredentials)
		})
	}
}

func TestWalk_rqliteAltScheme(t *testing.T) {
	got, err := Walk(rqliteShape, "rqlites://alice@h:8443?level=strong")
	require.NoError(t, err)
	require.Equal(t, "rqlites", got.Scheme)
	require.Equal(t, SegConnParams, got.Current)
}

func TestWalk_sqlserverDatabaseInQuery(t *testing.T) {
	got, err := Walk(sqlserverShape, "sqlserver://alice@h?database=mydb")
	require.NoError(t, err)
	require.Equal(t, SegConnParams, got.Current)
	require.Equal(t, "database", got.ParamLastKey)
	require.True(t, got.ParamAtValue)
}

func TestWalk_sqlserverInstanceAndDatabase(t *testing.T) {
	got, err := Walk(sqlserverShape, "sqlserver://alice@h/myinst?database=mydb")
	require.NoError(t, err)
	require.Contains(t, got.Done, SegPathName)
	require.Equal(t, "myinst", got.PathName)
	require.Equal(t, SegConnParams, got.Current)
}

func BenchmarkWalk(b *testing.B) {
	inputs := []struct {
		shape LocationShape
		loc   string
	}{
		{pgShape, "postgres://alice@db.example.com:5432/mydb?sslmode=require"},
		{rqliteShape, "rqlite://localhost:4001?disableClusterDiscovery=true"},
		{sqliteShape, "sqlite3:///path/to/sakila.db?cache=shared"},
		{pgShape, "postgres://"},
		{pgShape, "postgres://localhost:5432?"}, // the #743 case
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, in := range inputs {
			_, _ = Walk(in.shape, in.loc)
		}
	}
}
