package proj

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFindProjDir verifies that findProjDir locates the sq project root
// by walking up from a directory inside the repo. The test's own cwd
// (the testh/proj package dir) is inside the repo, so the walk must succeed.
func TestFindProjDir(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	got, ok := findProjDir(cwd)
	require.True(t, ok, "should locate sq project root from package cwd")
	require.True(t, isProjDir(got), "returned dir must contain sq's go.mod")
}

// TestFindProjDir_NotFound verifies that findProjDir returns ok=false when
// no ancestor of startDir is the sq project root. A temp dir is (on normal
// systems) outside the repo tree, so the walk reaches the filesystem root
// without a match.
func TestFindProjDir_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	if strings.HasPrefix(tmpDir, projDir) {
		t.Skip("TMPDIR is inside the sq checkout; test's outside-the-tree assumption is violated")
	}

	got, ok := findProjDir(tmpDir)
	require.False(t, ok, "temp dir outside repo must not resolve to a proj dir")
	require.Empty(t, got)
}

// TestProjDirIgnoresExternalEnv is the regression test for the worktree
// footgun: even when SQ_ROOT is set in the environment, resolution must
// derive from cwd and ignore the external value.
func TestProjDirIgnoresExternalEnv(t *testing.T) {
	t.Setenv(EnvRoot, filepath.Join(t.TempDir(), "not-the-real-root"))

	cwd, err := os.Getwd()
	require.NoError(t, err)

	got, ok := findProjDir(cwd)
	require.True(t, ok)
	require.True(t, isProjDir(got), "must derive from cwd, not the bogus SQ_ROOT env")
}
