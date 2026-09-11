package metadata_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source/metadata"
)

func TestSource_DBSemver_Marshal(t *testing.T) {
	// Present when set.
	b, err := json.Marshal(&metadata.Source{DBSemver: "v8.0.36"})
	require.NoError(t, err)
	require.Contains(t, string(b), `"db_semver":"v8.0.36"`)

	// Omitted when empty (omitempty).
	b2, err := json.Marshal(&metadata.Source{})
	require.NoError(t, err)
	require.NotContains(t, string(b2), "db_semver")
}

func TestSource_DBSemver_Clone(t *testing.T) {
	src := &metadata.Source{DBVersion: "8.0.36-x", DBSemver: "v8.0.36"}
	got := src.Clone()
	require.Equal(t, "v8.0.36", got.DBSemver)
}
