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

func TestResolver_TildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	// Stage a file inside the user's home dir.
	f, err := os.CreateTemp(home, ".sq_secret_test_*")
	require.NoError(t, err)
	_, err = f.WriteString("tilde-value\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	t.Cleanup(func() { _ = os.Remove(f.Name()) })

	// Reference it as ~/<basename>.
	rel := filepath.Base(f.Name())
	got, err := file.New().Resolve(context.Background(), "~/"+rel)
	require.NoError(t, err)
	require.Equal(t, "tilde-value", got)
}

func TestResolver_BareTildeRefersToHome(t *testing.T) {
	// "~" alone resolves to $HOME, which is a directory — reading it
	// should fail with a non-ErrNotFound error (it's an IsDirectory
	// error in Go's stdlib).
	_, err := file.New().Resolve(context.Background(), "~")
	require.Error(t, err)
	require.NotErrorIs(t, err, secret.ErrNotFound)
}

func TestResolver_RejectsRelativePath(t *testing.T) {
	tests := []string{
		"relative.txt",
		"./relative.txt",
		"sub/dir/file",
	}
	for _, p := range tests {
		t.Run(p, func(t *testing.T) {
			_, err := file.New().Resolve(context.Background(), p)
			require.Error(t, err)
			require.Contains(t, err.Error(), "must be absolute")
		})
	}
}

func TestResolver_RejectsTildeUser(t *testing.T) {
	_, err := file.New().Resolve(context.Background(), "~root/secret")
	require.Error(t, err)
	require.Contains(t, err.Error(), "tilde expansion")
}

func TestResolver_RejectsEmptyPath(t *testing.T) {
	_, err := file.New().Resolve(context.Background(), "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty path")
}

func TestResolver_EmptyAuthorityURIForm(t *testing.T) {
	// ${file:///path} is the RFC 8089 file:// URI with empty authority.
	// The parser hands us "///path" (it splits on the first colon after
	// the scheme). This is sugar for "/path" and should resolve the same
	// file.
	p := write(t, "uri-value")
	got, err := file.New().Resolve(context.Background(), "//"+p)
	require.NoError(t, err)
	require.Equal(t, "uri-value", got)
}

func TestResolver_RejectsRemoteURIForm(t *testing.T) {
	// file://host/path has a non-empty authority — that's a remote
	// reference we don't support. Two slashes alone (no authority,
	// no path-leading slash) is also ambiguous, reject the same way.
	tests := []string{
		"//host/etc/passwd", // file://host/etc/passwd
		"//etc/passwd",      // ambiguous: looks like host=etc, path=/passwd
	}
	for _, p := range tests {
		t.Run(p, func(t *testing.T) {
			_, err := file.New().Resolve(context.Background(), p)
			require.Error(t, err)
			require.Contains(t, err.Error(), "remote file URIs are not supported")
		})
	}
}
