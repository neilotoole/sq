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
