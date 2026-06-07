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
