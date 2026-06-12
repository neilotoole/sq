package driver

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// pgShape is the postgres-equivalent shape used in walker tests.
// It is a literal mirror of what drivers/postgres/postgres.go returns
// from LocationShape(). The duplication keeps walker tests pure (no
// driver-package dependency); driver tests in cli/ verify the mirror.
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

// TestWalk_schemeMismatchOmitsLoc pins the credential-leak guard:
// when the scheme doesn't match, the returned error must NOT echo
// loc, because loc can carry inline credentials and the error may
// be logged.
func TestWalk_schemeMismatchOmitsLoc(t *testing.T) {
	loc := "mysql://alice:hunter2@localhost"
	_, err := Walk(pgShape, loc)
	require.Error(t, err)
	require.EqualError(t, err, "scheme not matched")
	require.NotContains(t, err.Error(), "alice")
	require.NotContains(t, err.Error(), "hunter2")
	require.NotContains(t, err.Error(), "mysql")
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
	Schemes: []string{"rqlite"},
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
		{"rqlite_bare_host_port_q_tls", rqliteShape, "rqlite://localhost:4001?tls=true"},
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

// TestWalk_gh743BareHostIPv6 is the IPv6 variant of the #743 case.
// In addition to the segment-positioning assertions in
// TestWalk_gh743BareHost it pins the parsed Hostname and Port, so a
// future change that breaks bracket parsing in parseAuthority is
// caught at the walker layer.
func TestWalk_gh743BareHostIPv6(t *testing.T) {
	got, err := Walk(pgShape, "postgres://[::1]:5432?")
	require.NoError(t, err)
	require.Equal(t, SegConnParams, got.Current)
	require.NotContains(t, got.Done, SegCredentials)
	require.Contains(t, got.Done, SegAuthority)
	require.Equal(t, "::1", got.Hostname)
	require.Equal(t, 5432, got.Port)
	require.True(t, got.PortSet)
}

// TestWalk_gh792AtBeyondAuthority covers issue #792: an '@' in the
// path or query string of credential-less input must not be treated
// as the userinfo terminator. Per RFC 3986, userinfo can only occur
// before the first '/' or '?', so the walker bounds its '@' search to
// that authority portion. A tail with no '/' or '?' yet (mid-typing)
// keeps the existing behavior: an '@' there genuinely terminates
// userinfo.
func TestWalk_gh792AtBeyondAuthority(t *testing.T) {
	t.Run("at_in_query_value_trailing", func(t *testing.T) {
		// The reported repro: completing after "...application_name=me@".
		got, err := Walk(pgShape, "postgres://localhost/db?application_name=me@")
		require.NoError(t, err)
		require.Equal(t, SegConnParams, got.Current)
		require.NotContains(t, got.Done, SegCredentials)
		require.False(t, got.HasCreds)
		require.Empty(t, got.User)
		require.Equal(t, "localhost", got.Hostname)
		require.Equal(t, "db", got.PathName)
		require.Equal(t, "application_name", got.ParamLastKey)
		require.True(t, got.ParamAtValue)
		require.Equal(t, "me@", got.Params.Get("application_name"))
	})

	t.Run("at_in_query_value_then_next_key", func(t *testing.T) {
		got, err := Walk(pgShape, "postgres://localhost/db?application_name=me@example.com&ssl")
		require.NoError(t, err)
		require.Equal(t, SegConnParams, got.Current)
		require.False(t, got.HasCreds)
		require.Equal(t, "localhost", got.Hostname)
		require.Equal(t, "me@example.com", got.Params.Get("application_name"))
		require.Equal(t, "ssl", got.ParamLastKey)
		require.False(t, got.ParamAtValue)
	})

	t.Run("at_in_path_segment", func(t *testing.T) {
		got, err := Walk(pgShape, "postgres://localhost/cust@")
		require.NoError(t, err)
		require.Equal(t, SegPathName, got.Current)
		require.NotContains(t, got.Done, SegCredentials)
		require.False(t, got.HasCreds)
		require.Equal(t, "localhost", got.Hostname)
		require.Equal(t, "cust@", got.PathName)
	})

	t.Run("at_in_path_then_query", func(t *testing.T) {
		got, err := Walk(pgShape, "postgres://localhost/cust@db?")
		require.NoError(t, err)
		require.Equal(t, SegConnParams, got.Current)
		require.False(t, got.HasCreds)
		require.Contains(t, got.Done, SegPathName)
		require.Equal(t, "cust@db", got.PathName)
	})

	t.Run("at_in_query_sqlserver", func(t *testing.T) {
		got, err := Walk(sqlserverShape, "sqlserver://localhost?database=me@x")
		require.NoError(t, err)
		require.Equal(t, SegConnParams, got.Current)
		require.False(t, got.HasCreds)
		require.Equal(t, "localhost", got.Hostname)
		require.Equal(t, "database", got.ParamLastKey)
		require.True(t, got.ParamAtValue)
	})

	t.Run("creds_with_at_in_query_value", func(t *testing.T) {
		// Genuine credentials must keep working when the query also
		// contains an '@'.
		got, err := Walk(pgShape, "postgres://alice:hunter2@localhost/db?application_name=me@")
		require.NoError(t, err)
		require.True(t, got.HasCreds)
		require.Equal(t, "alice", got.User)
		require.Equal(t, "hunter2", got.Pass)
		require.Equal(t, "localhost", got.Hostname)
		require.Equal(t, "db", got.PathName)
		require.Equal(t, SegConnParams, got.Current)
		require.True(t, got.ParamAtValue)
		require.Equal(t, "me@", got.Params.Get("application_name"))
	})

	t.Run("at_terminates_userinfo_when_no_boundary_yet", func(t *testing.T) {
		// Mid-typing: no '/' or '?' yet, so the '@' is the userinfo
		// terminator (username without password). Unchanged behavior.
		got, err := Walk(pgShape, "postgres://bob@")
		require.NoError(t, err)
		require.Equal(t, []SegmentKind{SegCredentials}, got.Done)
		require.True(t, got.HasCreds)
		require.Equal(t, "bob", got.User)
		require.False(t, got.PassSet)
	})
}

func TestWalk_rqliteTLSParam(t *testing.T) {
	got, err := Walk(rqliteShape, "rqlite://alice@h:8443?level=strong&tls=true")
	require.NoError(t, err)
	require.Equal(t, "rqlite", got.Scheme)
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

// TestLocationShape_Validate covers the structural mistakes that a
// hand-rolled driver literal could realistically introduce. The
// happy path is covered per-driver in
// cli/complete_location_shapes_test.go.
func TestLocationShape_Validate(t *testing.T) {
	cases := []struct {
		name    string
		shape   LocationShape
		wantMsg string
	}{
		{
			name:    "empty type",
			shape:   LocationShape{Schemes: []string{"x"}, Segments: []Segment{{Kind: SegPathFile}}},
			wantMsg: "Type is empty",
		},
		{
			name:    "no schemes",
			shape:   LocationShape{Type: drivertype.Pg, Segments: []Segment{{Kind: SegPathFile}}},
			wantMsg: "Schemes is empty",
		},
		{
			name: "empty scheme entry",
			shape: LocationShape{
				Type:     drivertype.Pg,
				Schemes:  []string{"x", ""},
				Segments: []Segment{{Kind: SegPathFile}},
			},
			wantMsg: "Schemes[1] is empty",
		},
		{
			name:    "no segments",
			shape:   LocationShape{Type: drivertype.Pg, Schemes: []string{"x"}},
			wantMsg: "Segments is empty",
		},
		{
			name: "zero kind",
			shape: LocationShape{
				Type:     drivertype.Pg,
				Schemes:  []string{"x"},
				Segments: []Segment{{}},
			},
			wantMsg: "Segments[0].Kind is unset",
		},
		{
			name: "duplicate kind",
			shape: LocationShape{
				Type:    drivertype.Pg,
				Schemes: []string{"x"},
				Segments: []Segment{
					{Kind: SegAuthority},
					{Kind: SegAuthority},
				},
			},
			wantMsg: "duplicates kind",
		},
		{
			name: "leading key on wrong kind",
			shape: LocationShape{
				Type:    drivertype.Pg,
				Schemes: []string{"x"},
				Segments: []Segment{
					{Kind: SegAuthority, LeadingKey: "database"},
				},
			},
			wantMsg: "LeadingKey on kind",
		},
		{
			name: "placeholder on wrong kind",
			shape: LocationShape{
				Type:    drivertype.Pg,
				Schemes: []string{"x"},
				Segments: []Segment{
					{Kind: SegAuthority, Placeholder: "db"},
				},
			},
			wantMsg: "Placeholder on kind",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.shape.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantMsg)
		})
	}
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
	for b.Loop() {
		for _, in := range inputs {
			_, _ = Walk(in.shape, in.loc)
		}
	}
}
