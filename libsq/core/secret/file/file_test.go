package file_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/core/secret/file"
)

func write(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "secret")
	require.NoError(t, os.WriteFile(p, []byte(contents), 0o600))
	return p
}

func TestResolver_PlainContents(t *testing.T) {
	p := write(t, "hunter2")
	got, err := file.New().Resolve(context.Background(), p)
	require.NoError(t, err)
	require.Equal(t, "hunter2", got)
}

func TestResolver_TrailingLF_Trimmed(t *testing.T) {
	p := write(t, "hunter2\n")
	got, err := file.New().Resolve(context.Background(), p)
	require.NoError(t, err)
	require.Equal(t, "hunter2", got)
}

func TestResolver_TrailingCRLF_Trimmed(t *testing.T) {
	p := write(t, "hunter2\r\n")
	got, err := file.New().Resolve(context.Background(), p)
	require.NoError(t, err)
	require.Equal(t, "hunter2", got)
}

func TestResolver_OnlySingleTrailingNewlineTrimmed(t *testing.T) {
	// Multiple trailing newlines: only the last one is trimmed.
	p := write(t, "hunter2\n\n")
	got, err := file.New().Resolve(context.Background(), p)
	require.NoError(t, err)
	require.Equal(t, "hunter2\n", got)
}

func TestResolver_InternalNewlinesPreserved(t *testing.T) {
	p := write(t, "line1\nline2")
	got, err := file.New().Resolve(context.Background(), p)
	require.NoError(t, err)
	require.Equal(t, "line1\nline2", got)
}

func TestResolver_MissingFile(t *testing.T) {
	_, err := file.New().Resolve(context.Background(), filepath.Join(t.TempDir(), "no-such-file"))
	require.ErrorIs(t, err, secret.ErrNotFound)
}

func TestResolver_EmptyFile(t *testing.T) {
	p := write(t, "")
	got, err := file.New().Resolve(context.Background(), p)
	require.NoError(t, err)
	require.Equal(t, "", got)
}
