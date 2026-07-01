package diff

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source/metadata"
)

func TestRenderSourceMeta2YAML_DBSemver(t *testing.T) {
	// Present when set.
	got, err := renderSourceMeta2YAML(&metadata.Source{DBVersion: "8.0.36-x", DBSemver: "v8.0.36"})
	require.NoError(t, err)
	require.Contains(t, got, "db_semver: v8.0.36")

	// Omitted when empty (omitempty).
	got2, err := renderSourceMeta2YAML(&metadata.Source{DBVersion: "8.0.36-x"})
	require.NoError(t, err)
	require.NotContains(t, got2, "db_semver")
}
