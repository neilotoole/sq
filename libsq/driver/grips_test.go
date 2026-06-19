package driver_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/core/secret/env"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

type captureResolver struct {
	value string
	calls []string
}

func (c *captureResolver) Resolve(_ context.Context, path string) (string, error) {
	c.calls = append(c.calls, path)
	return c.value, nil
}

func TestGrips_ResolveSourceSecrets(t *testing.T) {
	reg := secret.NewRegistry()
	reg.Register("keyring", &captureResolver{value: "hunter2"})
	ctx := secret.NewContext(context.Background(), reg)

	src := &source.Source{
		Handle:   "@sakila",
		Location: "postgres://alice:${keyring:my_db_pw}@db/sakila",
	}

	resolved, err := driver.ResolveSourceSecrets(ctx, src)
	require.NoError(t, err)
	require.NotSame(t, src, resolved, "must return a clone")
	require.Equal(t,
		"postgres://alice:hunter2@db/sakila",
		resolved.Location)
	require.Equal(t,
		"postgres://alice:${keyring:my_db_pw}@db/sakila",
		src.Location, "original src must be untouched")
}

func TestGrips_ResolveSourceSecrets_NoPlaceholder(t *testing.T) {
	reg := secret.NewRegistry()
	ctx := secret.NewContext(context.Background(), reg)
	src := &source.Source{
		Handle:   "@sakila",
		Location: "postgres://alice:hunter2@db/sakila",
	}
	resolved, err := driver.ResolveSourceSecrets(ctx, src)
	require.NoError(t, err)
	require.Same(t, src, resolved, "no placeholder => return input unchanged")
}

// TestGrips_ResolveSourceSecrets_NoRefs_Unescape verifies that the $$
// escape is honored even when the location contains no ${scheme:path}
// refs: the driver must receive the literal form. This is what makes
// the v0.54.0 config upgrade's escaping of legacy locations (which
// never contain intentional placeholders) connect byte-identically.
// No secret.Registry is bound to the context: unescaping must not
// require one, since a ref-free location resolves nothing.
func TestGrips_ResolveSourceSecrets_NoRefs_Unescape(t *testing.T) {
	tests := []struct {
		name string
		loc  string
		want string
	}{
		{
			name: "escaped dollar in password",
			loc:  "postgres://alice:p$$ss@db/sakila",
			want: "postgres://alice:p$ss@db/sakila",
		},
		{
			name: "escaped well-formed placeholder",
			loc:  "postgres://alice:$${env:HOME}@db/sakila",
			want: "postgres://alice:${env:HOME}@db/sakila",
		},
		{
			name: "escaped malformed placeholder",
			loc:  "postgres://alice:p$${ss}w@db/sakila",
			want: "postgres://alice:p${ss}w@db/sakila",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := &source.Source{Handle: "@sakila", Location: tc.loc}
			resolved, err := driver.ResolveSourceSecrets(context.Background(), src)
			require.NoError(t, err)
			require.NotSame(t, src, resolved, "must return a clone when location changes")
			require.Equal(t, tc.want, resolved.Location)
			require.Equal(t, tc.loc, src.Location, "original src must be untouched")
		})
	}
}

// TestGrips_ResolveSourceSecrets_AgreesWithExpand verifies that
// connect-time resolution and Registry.Expand produce identical bytes
// for the same template (gh #787). Historically the two diverged on the
// zero-refs path: Expand unescaped "$$" while ResolveSourceSecrets
// returned the location verbatim, so a no-placeholder password like
// "pa$$wd" connected as "pa$$wd" but displayed and exported as "pa$wd",
// silently corrupting restored backups.
func TestGrips_ResolveSourceSecrets_AgreesWithExpand(t *testing.T) {
	reg := secret.NewRegistry()
	reg.Register("keyring", &captureResolver{value: "hunter2"})
	ctx := secret.NewContext(context.Background(), reg)

	tests := []struct {
		name string
		loc  string
	}{
		{
			name: "zero refs with dollar escape",
			loc:  "postgres://alice:pa$$wd@db/sakila",
		},
		{
			name: "zero refs no dollar",
			loc:  "postgres://alice:pw@db/sakila",
		},
		{
			name: "refs with dollar escape outside placeholder",
			loc:  "postgres://ali$$ce:${keyring:my_db_pw}@db/sakila",
		},
		{
			name: "refs only",
			loc:  "postgres://alice:${keyring:my_db_pw}@db/sakila",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expanded, err := reg.Expand(ctx, tc.loc)
			require.NoError(t, err)

			src := &source.Source{
				Handle:   "@sakila",
				Type:     drivertype.Pg,
				Location: tc.loc,
			}
			resolved, err := driver.ResolveSourceSecrets(ctx, src)
			require.NoError(t, err)
			require.Equal(t, expanded, resolved.Location,
				"connect-time bytes must equal Expand output")
		})
	}
}

// TestGrips_ResolveSourceSecrets_Idempotent verifies that resolving an
// already-resolved source is a no-op. Resolution converts template
// bytes to literal bytes; reinterpreting literal bytes as a template
// (a second '$$' unescape, or re-resolution of '${...}'-shaped text
// inside a resolved secret value) silently corrupts credentials. This
// class of bug occurred three times during review (ping --expand,
// inspect --expand, config export --expand), so the resolved clone now
// carries a marker making double-resolution structurally harmless.
func TestGrips_ResolveSourceSecrets_Idempotent(t *testing.T) {
	t.Run("zero refs escaped literal", func(t *testing.T) {
		src := &source.Source{
			Handle:   "@sakila",
			Location: "postgres://alice:p$$$$wd@db/sakila",
		}
		r1, err := driver.ResolveSourceSecrets(context.Background(), src)
		require.NoError(t, err)
		require.Equal(t, "postgres://alice:p$$wd@db/sakila", r1.Location)

		r2, err := driver.ResolveSourceSecrets(context.Background(), r1)
		require.NoError(t, err)
		require.Same(t, r1, r2, "second resolution must be a no-op")
		require.Equal(t, "postgres://alice:p$$wd@db/sakila", r2.Location)
	})

	t.Run("resolved secret value containing dollars", func(t *testing.T) {
		reg := secret.NewRegistry()
		reg.Register("keyring", &captureResolver{value: "p$$wd"})
		ctx := secret.NewContext(context.Background(), reg)

		src := &source.Source{
			Handle:   "@sakila",
			Location: "postgres://alice:${keyring:my_db_pw}@db/sakila",
		}
		r1, err := driver.ResolveSourceSecrets(ctx, src)
		require.NoError(t, err)
		require.Equal(t, "postgres://alice:p$$wd@db/sakila", r1.Location)

		// Without the marker, this second pass would halve '$$' to '$'.
		r2, err := driver.ResolveSourceSecrets(ctx, r1)
		require.NoError(t, err)
		require.Same(t, r1, r2, "second resolution must be a no-op")
		require.Equal(t, "postgres://alice:p$$wd@db/sakila", r2.Location)
	})
}

// TestGrips_ResolveSourceSecrets_MungeFileDB verifies that the
// driver-specific location munging that "sq add" applies to literal
// file-DB locations is also applied to a location resolved from
// placeholders (gh #798). A stored "${env:DB_PATH}" with
// DB_PATH=/data/sakila.db must resolve to "sqlite3:///data/sakila.db";
// the bare path would be rejected by the sqlite3 driver at connect
// time ("invalid sqlite3 location: missing \"sqlite3://\" prefix").
// The stored template must remain untouched: only the resolved clone
// carries the munged form.
func TestGrips_ResolveSourceSecrets_MungeFileDB(t *testing.T) {
	relPath, err := filepath.Abs("data/sakila.db")
	require.NoError(t, err)

	// Volume-qualify the absolute test paths so the expectations hold on
	// Windows too: there, filepath.Abs("/data/sakila.db") resolves the
	// rooted-but-driveless path against the current drive, yielding e.g.
	// "C:\data\sakila.db", which munges to "sqlite3://C:/data/sakila.db".
	// On POSIX, Abs is the identity here and the canonical slash form is
	// "sqlite3:///data/sakila.db".
	absSakilaDB, err := filepath.Abs("/data/sakila.db")
	require.NoError(t, err)
	absSakilaDBSlash := filepath.ToSlash(absSakilaDB)
	absSakilaDuck, err := filepath.Abs("/data/sakila.duckdb")
	require.NoError(t, err)

	testCases := []struct {
		name    string
		typ     drivertype.Type
		envVal  string
		want    string
		wantErr bool
	}{
		{
			name:   "sqlite bare absolute path",
			typ:    drivertype.SQLite,
			envVal: absSakilaDB,
			want:   "sqlite3://" + absSakilaDBSlash,
		},
		{
			name:   "sqlite bare relative path",
			typ:    drivertype.SQLite,
			envVal: "data/sakila.db",
			want:   "sqlite3://" + filepath.ToSlash(relPath),
		},
		{
			name:   "sqlite already munged",
			typ:    drivertype.SQLite,
			envVal: "sqlite3://" + absSakilaDBSlash,
			want:   "sqlite3://" + absSakilaDBSlash,
		},
		{
			name:   "sqlite munged with query suffix",
			typ:    drivertype.SQLite,
			envVal: "sqlite3://" + absSakilaDBSlash + "?mode=ro",
			want:   "sqlite3://" + absSakilaDBSlash + "?mode=ro",
		},
		{
			name:   "duckdb bare absolute path",
			typ:    drivertype.DuckDB,
			envVal: absSakilaDuck,
			want:   "duckdb://" + filepath.ToSlash(absSakilaDuck),
		},
		{
			name:   "duckdb memory",
			typ:    drivertype.DuckDB,
			envVal: ":memory:",
			want:   "duckdb://:memory:",
		},
		{
			name:    "sqlite empty resolved value",
			typ:     drivertype.SQLite,
			envVal:  "",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SQ_TEST_GH798_DB_PATH", tc.envVal)
			reg := secret.NewRegistry()
			reg.Register("env", env.NewResolver())
			ctx := secret.NewContext(context.Background(), reg)

			src := &source.Source{
				Handle:   "@gh798",
				Type:     tc.typ,
				Location: "${env:SQ_TEST_GH798_DB_PATH}",
			}

			resolved, err := driver.ResolveSourceSecrets(ctx, src)
			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "@gh798")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, resolved.Location)
			require.Equal(t, "${env:SQ_TEST_GH798_DB_PATH}", src.Location,
				"stored template must remain untouched")
		})
	}
}

// TestGrips_ResolveSourceSecrets_MungeFileDB_NoChange verifies that
// connect-time munging is gated on resolution actually changing the
// location bytes. A literal file-DB location with no placeholders or
// escapes was already munged by "sq add" (or deliberately left
// non-canonical by the user); ResolveSourceSecrets must pass it
// through untouched, preserving pre-resolution behavior. Likewise, if
// expansion is a byte-level no-op (a secret value that is literally
// its own placeholder text), the still-placeholder-shaped string must
// not be reinterpreted as a file path in the cwd; it passes through
// for the driver to reject.
func TestGrips_ResolveSourceSecrets_MungeFileDB_NoChange(t *testing.T) {
	t.Run("literal non-canonical location", func(t *testing.T) {
		src := &source.Source{
			Handle:   "@gh798",
			Type:     drivertype.SQLite,
			Location: "/data/sakila.db",
		}
		resolved, err := driver.ResolveSourceSecrets(context.Background(), src)
		require.NoError(t, err)
		require.Same(t, src, resolved, "no change => return input unchanged")
		require.Equal(t, "/data/sakila.db", resolved.Location,
			"location must not be munged")
	})

	t.Run("expansion is byte-level no-op", func(t *testing.T) {
		const tmpl = "${env:SQ_TEST_GH798_DB_PATH}"
		t.Setenv("SQ_TEST_GH798_DB_PATH", tmpl)
		reg := secret.NewRegistry()
		reg.Register("env", env.NewResolver())
		ctx := secret.NewContext(context.Background(), reg)

		src := &source.Source{
			Handle:   "@gh798",
			Type:     drivertype.SQLite,
			Location: tmpl,
		}
		resolved, err := driver.ResolveSourceSecrets(ctx, src)
		require.NoError(t, err)
		require.Equal(t, tmpl, resolved.Location,
			"placeholder-shaped resolved value must not be munged into a file path")
		require.True(t, resolved.SecretsResolved)
	})
}

// TestGrips_ResolveSourceSecrets_MungeFileDB_NonFileDriver verifies
// that munging at the resolution boundary leaves non-file driver
// locations untouched.
func TestGrips_ResolveSourceSecrets_MungeFileDB_NonFileDriver(t *testing.T) {
	reg := secret.NewRegistry()
	reg.Register("keyring", &captureResolver{value: "hunter2"})
	ctx := secret.NewContext(context.Background(), reg)

	src := &source.Source{
		Handle:   "@sakila",
		Type:     drivertype.Pg,
		Location: "postgres://alice:${keyring:my_db_pw}@db/sakila",
	}

	resolved, err := driver.ResolveSourceSecrets(ctx, src)
	require.NoError(t, err)
	require.Equal(t, "postgres://alice:hunter2@db/sakila", resolved.Location)
}

func TestGrips_ResolveSourceSecrets_NoRegistry(t *testing.T) {
	src := &source.Source{
		Handle:   "@sakila",
		Location: "postgres://alice:${keyring:my_db_pw}@db/sakila",
	}
	// Placeholders present but no secret.Registry on context: must
	// return an explicit error rather than silently passing the
	// unresolved Location through to the driver, where it would
	// surface as a confusing DSN-parse or connection error.
	resolved, err := driver.ResolveSourceSecrets(context.Background(), src)
	require.Error(t, err)
	require.Nil(t, resolved)
	require.Contains(t, err.Error(), "@sakila")
	require.Contains(t, err.Error(), "no secret registry bound to context")
}

func TestGrips_ResolveSourceSecrets_NilSource(t *testing.T) {
	got, err := driver.ResolveSourceSecrets(context.Background(), nil)
	require.NoError(t, err)
	require.Nil(t, got)
}
