package cli_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz/lockfile"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/core/secret/keyring"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
)

// failingConfigStore wraps a config.Store and forces Save to return an
// error, used by migrate-rollback tests. Load / Location / Lockfile /
// Exists delegate to the underlying store.
type failingConfigStore struct {
	underlying config.Store
}

func (s *failingConfigStore) Save(_ context.Context, _ *config.Config) error {
	return errz.New("simulated save failure")
}

func (s *failingConfigStore) Load(ctx context.Context) (*config.Config, error) {
	return s.underlying.Load(ctx)
}
func (s *failingConfigStore) Exists() bool     { return s.underlying.Exists() }
func (s *failingConfigStore) Location() string { return s.underlying.Location() }
func (s *failingConfigStore) Lockfile() (lockfile.Lockfile, error) {
	return s.underlying.Lockfile()
}

// migrateRollbackRow / migrateRollbackEnvelope and migrateJSONRow /
// migrateJSONEnvelope are JSON unmarshal targets for migrate-result
// tests. Promoted to top-level types (rather than inline anonymous
// structs) so the revive nested-structs lint stays happy.
type migrateRollbackRow struct {
	Handle string `json:"handle"`
	Status string `json:"status"`
	Error  string `json:"error"`
}
type migrateRollbackEnvelope struct {
	Rows   []migrateRollbackRow `json:"rows"`
	DryRun bool                 `json:"dry_run"`
}

type migrateJSONRow struct {
	Handle string `json:"handle"`
	Status string `json:"status"`
}
type migrateJSONEnvelope struct {
	Rows   []migrateJSONRow `json:"rows"`
	DryRun bool             `json:"dry_run"`
}

type pruneJSONRow struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}
type pruneJSONEnvelope struct {
	Rows   []pruneJSONRow `json:"rows"`
	DryRun bool           `json:"dry_run"`
}

func TestCmdConfigKeyringCreate_ExplicitValue(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	err := tr.Exec("config", "keyring", "create", "my_db_pw", "hunter2")
	require.NoError(t, err)

	got, err := gokeyring.Get("sq", "my_db_pw")
	require.NoError(t, err)
	require.Equal(t, "hunter2", got)
}

func TestCmdConfigKeyringCreate_PromptedFromStdin(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	// Pipe a password through stdin.
	tr.PipeStdin("hunter2\n")

	err := tr.Exec("config", "keyring", "create", "my_db_pw", "-p")
	require.NoError(t, err)

	got, err := gokeyring.Get("sq", "my_db_pw")
	require.NoError(t, err)
	require.Equal(t, "hunter2", got)
}

func TestCmdConfigKeyringCreate_RequiresValueOrFlag(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	// No VALUE arg, no -p flag.
	err := tr.Exec("config", "keyring", "create", "my_db_pw")
	require.Error(t, err)
}

// TestCmdConfigKeyringCreate_RejectsExisting: create errors when an
// entry already exists at the target path, and points the user at the
// update command.
func TestCmdConfigKeyringCreate_RejectsExisting(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "@dup/password", "old"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	err := tr.Exec("config", "keyring", "create", "@dup/password", "new")
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
	require.Contains(t, err.Error(), "update")

	// The existing value must be untouched.
	v, err := gokeyring.Get("sq", "@dup/password")
	require.NoError(t, err)
	require.Equal(t, "old", v)
}

func TestCmdConfigKeyringUpdate_ExplicitValue(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "@upd/password", "old"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	err := tr.Exec("config", "keyring", "update", "@upd/password", "new")
	require.NoError(t, err)

	got, err := gokeyring.Get("sq", "@upd/password")
	require.NoError(t, err)
	require.Equal(t, "new", got)
}

// TestCmdConfigKeyringUpdate_RejectsMissing: update errors when the
// target path has no entry, and points the user at the create command.
func TestCmdConfigKeyringUpdate_RejectsMissing(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	err := tr.Exec("config", "keyring", "update", "@absent/password", "new")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no keyring entry")
	require.Contains(t, err.Error(), "create")
}

// TestCmdConfigKeyringUpdate_Completion verifies that update's PATH
// argument shell-completes from existing keyring refs.
func TestCmdConfigKeyringUpdate_Completion(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil).Add(
		source.Source{
			Handle:   "@a_upd",
			Type:     drivertype.Pg,
			Location: "postgres://alice:${keyring:j2k7m3pxtz}@db/sakila",
		},
		source.Source{
			Handle:   "@b_upd",
			Type:     drivertype.Pg,
			Location: "postgres://alice:${keyring:abc456defg}@db/sakila",
		},
		// env: must NOT appear in keyring-update completions.
		source.Source{
			Handle:   "@c_upd",
			Type:     drivertype.Pg,
			Location: "postgres://alice:${env:DB_PW}@db/sakila",
		},
	)

	got := testComplete(t, tr, "config", "keyring", "update", "")
	require.Equal(t, []string{"abc456defg", "j2k7m3pxtz"}, got.values)
	require.Contains(t, got.directives, cobra.ShellCompDirectiveNoFileComp)

	// Prefix narrows.
	got = testComplete(t, tr, "config", "keyring", "update", "j2")
	require.Equal(t, []string{"j2k7m3pxtz"}, got.values)
}

func TestCmdConfigKeyringGet_WithoutRevealPrintsMetadataOnly(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "my_db_pw", "hunter2"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "keyring", "get", "my_db_pw"))
	require.NotContains(t, tr.Out.String(), "hunter2")
	require.Contains(t, tr.Out.String(), "my_db_pw")
}

func TestCmdConfigKeyringGet_WithRevealPrintsValue(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "my_db_pw", "hunter2"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "keyring", "get", "my_db_pw", "--reveal"))
	require.Contains(t, tr.Out.String(), "hunter2")
}

func TestCmdConfigKeyringGet_MissingErrors(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	err := tr.Exec("config", "keyring", "get", "@nope/x")
	require.Error(t, err)
}

func TestCmdConfigKeyringRm(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "my_db_pw", "hunter2"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "keyring", "rm", "my_db_pw"))

	_, err := gokeyring.Get("sq", "my_db_pw")
	require.ErrorIs(t, err, gokeyring.ErrNotFound)
}

func TestCmdConfigKeyringRm_MissingIsNotError(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "keyring", "rm", "@nope/x"))
}

func TestCmdConfigKeyringRm_Completion(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil).Add(
		source.Source{
			Handle:   "@a",
			Type:     drivertype.Pg,
			Location: "postgres://alice:${keyring:j2k7m3pxtz}@db/sakila",
		},
		source.Source{
			Handle:   "@b",
			Type:     drivertype.Pg,
			Location: "postgres://alice:${keyring:abc456defg}@db/sakila",
		},
		// env: ref must NOT appear in keyring-rm completions.
		source.Source{
			Handle:   "@c",
			Type:     drivertype.Pg,
			Location: "postgres://alice:${env:DB_PW}@db/sakila",
		},
		// Source without a password (placeholder-free) must be ignored.
		source.Source{
			Handle:   "@d",
			Type:     drivertype.Pg,
			Location: "postgres://alice:hunter2@db/sakila",
		},
	)

	got := testComplete(t, tr, "config", "keyring", "rm", "")
	require.Equal(t, []string{"abc456defg", "j2k7m3pxtz"}, got.values)
	require.Contains(t, got.directives, cobra.ShellCompDirectiveNoFileComp)

	// Prefix narrows to the matching subset.
	got = testComplete(t, tr, "config", "keyring", "rm", "j2")
	require.Equal(t, []string{"j2k7m3pxtz"}, got.values)
}

func TestCmdConfigKeyringGet_Completion(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil).Add(
		source.Source{
			Handle:   "@a_g",
			Type:     drivertype.Pg,
			Location: "postgres://alice:${keyring:j2k7m3pxtz}@db/sakila",
		},
		source.Source{
			Handle:   "@b_g",
			Type:     drivertype.Pg,
			Location: "postgres://alice:${keyring:abc456defg}@db/sakila",
		},
		// env: ref must NOT appear in keyring-get completions.
		source.Source{
			Handle:   "@c_g",
			Type:     drivertype.Pg,
			Location: "postgres://alice:${env:DB_PW}@db/sakila",
		},
	)

	got := testComplete(t, tr, "config", "keyring", "get", "")
	require.Equal(t, []string{"abc456defg", "j2k7m3pxtz"}, got.values)
	require.Contains(t, got.directives, cobra.ShellCompDirectiveNoFileComp)

	// Prefix narrows to the matching subset.
	got = testComplete(t, tr, "config", "keyring", "get", "abc")
	require.Equal(t, []string{"abc456defg"}, got.values)
}

func TestCmdConfigKeyringMigrate_PerCase(t *testing.T) {
	tests := []struct {
		name string
		// inLocation is the source's Location before migrate runs.
		inLocation string
		// wantKeyring is the value the keyring entry should hold after
		// a successful migration (i.e. the full DSN verbatim). Empty
		// when the source should be skipped.
		wantKeyring string
		// wantSkipReason is a substring expected on stdout when the
		// source is skipped. Empty when the source should be migrated.
		wantSkipReason string
	}{
		{
			name:        "url with password",
			inLocation:  "postgres://alice:hunter2@db/sakila",
			wantKeyring: "postgres://alice:hunter2@db/sakila",
		},
		{
			name:           "url without password",
			inLocation:     "postgres://alice@db/sakila",
			wantSkipReason: "no password",
		},
		{
			name:           "non-url",
			inLocation:     "/data/file.xlsx",
			wantSkipReason: "not a URL",
		},
		{
			name: "malformed placeholder is surfaced, not silently migrated",
			// Unclosed ${ — ExtractRefs returns an error. Migrate must
			// NOT stamp the malformed Location into the keyring.
			inLocation:     "postgres://alice:${env:UNCLOSED@db/sakila",
			wantSkipReason: "malformed placeholder",
		},
		{
			name:           "already templated",
			inLocation:     "postgres://alice:${keyring:@h/password}@db/sakila",
			wantSkipReason: "already",
		},
		{
			name:        "url-encoded password preserved verbatim",
			inLocation:  "postgres://alice:p%40ss%3Aword@db/sakila",
			wantKeyring: "postgres://alice:p%40ss%3Aword@db/sakila",
		},
		{
			// The stored Location is a placeholder template: '$$' means a
			// literal '$' (e.g. written by the v0.54.0 config upgrade).
			// The keyring slot holds a literal value that Registry.Expand
			// splices raw at connect time, so migrate must unescape, or
			// the driver would receive the still-escaped bytes.
			name:        "escaped dollar unescaped before keyring store",
			inLocation:  "postgres://alice:p$$wd@db/sakila",
			wantKeyring: "postgres://alice:p$wd@db/sakila",
		},
		{
			// An escaped placeholder ('$${env:HOME}' means the literal
			// text '${env:HOME}') is skipped, not migrated: the braces
			// make url.Parse reject the userinfo. The source keeps
			// working via the connect-path unescape, so skipping is
			// safe; migrating would have to unescape (see previous
			// case).
			name:           "escaped placeholder skipped as non-URL",
			inLocation:     "postgres://alice:$${env:HOME}@db/sakila",
			wantSkipReason: "not a URL",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gokeyring.MockInit()
			th := testh.New(t)
			tr := testrun.New(th.Context, t, nil)
			require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
				Handle:   "@h",
				Type:     "postgres",
				Location: tc.inLocation,
			}))

			require.NoError(t, tr.Exec("config", "keyring", "migrate", "--all", "--yes"))

			src, err := tr.Run.Config.Collection.Get("@h")
			require.NoError(t, err)

			if tc.wantSkipReason != "" {
				// Skipped: Location unchanged from input; no keyring entry written.
				require.Equal(t, tc.inLocation, src.Location)
				require.Contains(t, tr.Out.String(), tc.wantSkipReason)
				return
			}

			// Success: Location is a bare ${keyring:<crockford-id>}; keyring at that
			// id holds the entire input DSN verbatim (no URL-decoding, no surgery).
			id := extractKeyringID(t, src.Location)
			got, err := gokeyring.Get("sq", id)
			require.NoError(t, err)
			require.Equal(t, tc.wantKeyring, got)
		})
	}
}

func TestCmdConfigKeyringMigrate_DryRun(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@h_dr",
		Type:     "postgres",
		Location: "postgres://alice:hunter2@db/sakila",
	}))

	require.NoError(t, tr.Exec("config", "keyring", "migrate", "--all", "--dry-run"))

	// Source unchanged.
	src, _ := tr.Run.Config.Collection.Get("@h_dr")
	require.Equal(t, "postgres://alice:hunter2@db/sakila", src.Location)
	// Dry-run mints no IDs and writes nothing to the keyring; the planned
	// output uses the literal "<new-id>" stand-in.
	require.Contains(t, tr.Out.String(), "@h_dr")
	require.Contains(t, tr.Out.String(), "${keyring:<new-id>}")
}

func TestCmdConfigKeyringLs(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	// Seed sources with various placeholder shapes.
	tr.Add(
		source.Source{
			Handle:   "@sakila_ls",
			Type:     drivertype.Pg,
			Location: "postgres://alice:${keyring:@sakila_ls/password}@db/sakila",
		},
		source.Source{
			Handle:   "@prod_pg_ls",
			Type:     drivertype.Pg,
			Location: "${keyring:@prod_pg_ls/dsn}",
		},
		// Non-keyring placeholders MUST NOT appear in keyring-ls output.
		source.Source{
			Handle:   "@env_ls",
			Type:     drivertype.Pg,
			Location: "postgres://alice:${env:DB_PW}@db/sakila",
		},
		source.Source{
			Handle:   "@file_ls",
			Type:     drivertype.Pg,
			Location: "postgres://alice:${file:/etc/sq/secret}@db/sakila",
		},
		// Plain inline source — should NOT appear in ls output.
		source.Source{
			Handle:   "@plain_ls",
			Type:     drivertype.Pg,
			Location: "postgres://alice:hunter2@db/sakila",
		},
	)

	require.NoError(t, tr.Exec("config", "keyring", "ls"))
	out := tr.Out.String()
	require.Contains(t, out, "@sakila_ls/password")
	require.Contains(t, out, "@prod_pg_ls/dsn")
	// Confirm env/file refs are filtered out, and plain source is absent.
	require.NotContains(t, out, "DB_PW")
	require.NotContains(t, out, "/etc/sq/secret")
	require.NotContains(t, out, "@plain_ls")
	require.NotContains(t, out, "@env_ls")
	require.NotContains(t, out, "@file_ls")
}

// TestCmdConfigKeyringLs_EmptyConfig — no sources means no output and
// no error. Distinguishes "empty list" from "broken command".
func TestCmdConfigKeyringLs_EmptyConfig(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "keyring", "ls"))
	require.Empty(t, tr.Out.String())
}

// TestCmdConfigKeyringLs_HandleAndDriverColumns verifies that each row
// pairs the keyring path with its source's handle and driver type, and
// that non-keyring refs in the collection don't produce rows.
func TestCmdConfigKeyringLs_HandleAndDriverColumns(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	tr.Add(
		source.Source{
			Handle:   "@cols_pg",
			Type:     drivertype.Pg,
			Location: "${keyring:abc1234567}",
		},
		// Non-keyring source: must be filtered out entirely.
		source.Source{
			Handle:   "@cols_my",
			Type:     drivertype.MySQL,
			Location: "${env:MY_DSN}",
		},
	)
	require.NoError(t, tr.Exec("config", "keyring", "ls", "--no-header"))
	out := tr.Out.String()

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.Len(t, lines, 1, "only the keyring source should produce a row")
	require.Contains(t, lines[0], "abc1234567")
	require.Contains(t, lines[0], "@cols_pg")
	require.Contains(t, lines[0], "postgres")
	require.NotContains(t, out, "@cols_my")
}

// TestCmdConfigKeyringLs_SharedRefShowsMultipleRows verifies the
// load-bearing Form B property: when two sources reference the same
// keyring ID, the listing makes the sharing visible by emitting one
// row per (path, source) pair rather than deduplicating.
func TestCmdConfigKeyringLs_SharedRefShowsMultipleRows(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	const sharedID = "r5x2cd9k7w"
	tr.Add(
		source.Source{
			Handle:   "@primary_sh",
			Type:     drivertype.Pg,
			Location: "${keyring:" + sharedID + "}",
		},
		source.Source{
			Handle:   "@replica_sh",
			Type:     drivertype.Pg,
			Location: "${keyring:" + sharedID + "}",
		},
	)

	require.NoError(t, tr.Exec("config", "keyring", "ls", "--no-header"))
	lines := strings.Split(strings.TrimRight(tr.Out.String(), "\n"), "\n")
	require.Len(t, lines, 2, "shared ref should produce one row per source")

	// Both rows carry the same path; handles distinguish them. Sort
	// order is by path then handle, so @primary_sh < @replica_sh.
	for _, ln := range lines {
		require.Contains(t, ln, sharedID)
	}
	require.Contains(t, lines[0], "@primary_sh")
	require.Contains(t, lines[1], "@replica_sh")
}

// TestCmdConfigKeyringLs_CompositionFiltersNonKeyring verifies that
// a single source with mixed placeholder schemes produces exactly one
// row per ${keyring:...} placeholder, with env/file placeholders
// silently filtered out.
func TestCmdConfigKeyringLs_CompositionFiltersNonKeyring(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	tr.Add(source.Source{
		Handle:   "@compo",
		Type:     drivertype.Pg,
		Location: "postgres://${env:DB_USER}:${keyring:abc1234567}@${env:DB_HOST}/sakila",
	})

	require.NoError(t, tr.Exec("config", "keyring", "ls", "--no-header"))
	lines := strings.Split(strings.TrimRight(tr.Out.String(), "\n"), "\n")
	require.Len(t, lines, 1, "only the keyring placeholder should produce a row")
	require.Contains(t, lines[0], "abc1234567")
	require.Contains(t, lines[0], "@compo")
	require.Contains(t, lines[0], "postgres")
}

// TestCmdConfigKeyringLs_HeaderRow verifies that the default header
// row prints PATH/HANDLE/DRIVER, and that --no-header suppresses it.
func TestCmdConfigKeyringLs_HeaderRow(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil).Add(
		source.Source{
			Handle:   "@hdr",
			Type:     drivertype.Pg,
			Location: "${keyring:hdr_id}",
		},
	)

	// Default: header is on.
	require.NoError(t, tr.Exec("config", "keyring", "ls"))
	out := tr.Out.String()
	require.Contains(t, out, "PATH")
	require.Contains(t, out, "HANDLE")
	require.Contains(t, out, "DRIVER")
	require.Contains(t, out, "STATUS")
	require.Contains(t, out, "hdr_id")

	// --no-header (and the -H short form) suppresses it.
	tr = testrun.New(th.Context, t, tr)
	require.NoError(t, tr.Exec("config", "keyring", "ls", "--no-header"))
	out = tr.Out.String()
	require.NotContains(t, out, "PATH")
	require.NotContains(t, out, "HANDLE")
	require.NotContains(t, out, "DRIVER")
	require.Contains(t, out, "hdr_id")
}

// TestCmdConfigKeyringLs_JSON exercises --json output on ls.
func TestCmdConfigKeyringLs_JSON(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil).Add(
		source.Source{
			Handle:   "@a_js",
			Type:     drivertype.Pg,
			Location: "${keyring:abc1}",
		},
		source.Source{
			Handle:   "@b_js",
			Type:     drivertype.MySQL,
			Location: "${keyring:abc2}",
		},
		// env: must be filtered out of the JSON array too.
		source.Source{
			Handle:   "@env_js",
			Type:     drivertype.Pg,
			Location: "${env:DSN}",
		},
	)
	require.NoError(t, tr.Exec("config", "keyring", "ls", "--json"))

	var got []struct {
		Path   string `json:"path"`
		Handle string `json:"handle"`
		Driver string `json:"driver"`
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &got))
	require.Len(t, got, 2, "env source must not produce a row")
	require.Equal(t, "abc1", got[0].Path)
	require.Equal(t, "@a_js", got[0].Handle)
	require.Equal(t, "postgres", got[0].Driver)
	// Paths are referenced by sources but not written to the mock keyring.
	require.Equal(t, output.KeyringStatusMissing, got[0].Status)
	require.Equal(t, "abc2", got[1].Path)
	require.Equal(t, "@b_js", got[1].Handle)
	require.Equal(t, "mysql", got[1].Driver)
	require.Equal(t, output.KeyringStatusMissing, got[1].Status)
}

// TestCmdConfigKeyringLs_JSON_Empty: empty collection emits a JSON
// empty array, not a no-output run. Lets consumers safely `jq .` it.
func TestCmdConfigKeyringLs_JSON_Empty(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "keyring", "ls", "--json"))
	var got []any
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &got))
	require.Empty(t, got)
}

// TestCmdConfigKeyringGet_JSON_WithoutReveal: metadata-only — the
// "value" key must be absent from the JSON object.
func TestCmdConfigKeyringGet_JSON_WithoutReveal(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "abc_get_js", "hunter2"))
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "keyring", "get", "abc_get_js", "--json"))

	var got map[string]any
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &got))
	require.Equal(t, "abc_get_js", got["path"])
	require.Equal(t, true, got["exists"])
	_, hasValue := got["value"]
	require.False(t, hasValue, "value must be absent without --reveal")
}

// TestCmdConfigKeyringGet_JSON_WithReveal: --reveal puts the secret
// value into the JSON object's "value" key.
func TestCmdConfigKeyringGet_JSON_WithReveal(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "abc_get_js2", "hunter2"))
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "keyring", "get", "abc_get_js2", "--reveal", "--json"))

	var got map[string]any
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &got))
	require.Equal(t, "abc_get_js2", got["path"])
	require.Equal(t, true, got["exists"])
	require.Equal(t, "hunter2", got["value"])
}

// TestCmdConfigKeyringGet_JSON_WithReveal_EmptyValue: an empty stored
// secret must still emit the "value" key (as "") with --reveal, so a
// consumer can distinguish "empty secret, revealed" from "redacted".
func TestCmdConfigKeyringGet_JSON_WithReveal_EmptyValue(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "abc_get_js_empty", ""))
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "keyring", "get", "abc_get_js_empty", "--reveal", "--json"))

	var got map[string]any
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &got))
	require.Equal(t, "abc_get_js_empty", got["path"])
	require.Equal(t, true, got["exists"])
	v, hasValue := got["value"]
	require.True(t, hasValue, "value must be present with --reveal, even when empty")
	require.Equal(t, "", v)
}

// TestCmdConfigKeyringGet_JSON_WithoutReveal_EmptyValue: without
// --reveal, an empty stored secret emits no "value" key, same as any
// other redacted secret.
func TestCmdConfigKeyringGet_JSON_WithoutReveal_EmptyValue(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "abc_get_js_empty2", ""))
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "keyring", "get", "abc_get_js_empty2", "--json"))

	var got map[string]any
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &got))
	require.Equal(t, "abc_get_js_empty2", got["path"])
	require.Equal(t, true, got["exists"])
	_, hasValue := got["value"]
	require.False(t, hasValue, "value must be absent without --reveal")
}

// TestCmdConfigKeyringCreate_JSON: explicit create with --json emits a
// confirmation object with "created": true.
func TestCmdConfigKeyringCreate_JSON(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "keyring", "create", "abc_create_js", "v", "--json"))

	var got map[string]any
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &got))
	require.Equal(t, "abc_create_js", got["path"])
	require.Equal(t, true, got["created"])

	v, err := gokeyring.Get("sq", "abc_create_js")
	require.NoError(t, err)
	require.Equal(t, "v", v)
}

// TestCmdConfigKeyringUpdate_JSON: explicit update with --json emits a
// confirmation object with "updated": true.
func TestCmdConfigKeyringUpdate_JSON(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "abc_update_js", "old"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "keyring", "update", "abc_update_js", "new", "--json"))

	var got map[string]any
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &got))
	require.Equal(t, "abc_update_js", got["path"])
	require.Equal(t, true, got["updated"])

	v, err := gokeyring.Get("sq", "abc_update_js")
	require.NoError(t, err)
	require.Equal(t, "new", v)
}

// TestCmdConfigKeyringRm_JSON: rm emits a deletion confirmation.
func TestCmdConfigKeyringRm_JSON(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "abc_rm_js", "v"))
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "keyring", "rm", "abc_rm_js", "--json"))

	var got map[string]any
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &got))
	require.Equal(t, "abc_rm_js", got["path"])
	require.Equal(t, true, got["deleted"])
}

// TestCmdConfigKeyringMigrate_JSON_DryRun: dry-run JSON envelope reports
// dry_run=true and one row per source with planned/skip status.
func TestCmdConfigKeyringMigrate_JSON_DryRun(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@m_pw",
		Type:     "postgres",
		Location: "postgres://alice:hunter2@db/sakila",
	}))
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@m_skip",
		Type:     "postgres",
		Location: "postgres://alice@db/sakila", // no password -> skip
	}))

	require.NoError(t, tr.Exec("config", "keyring", "migrate", "--all", "--dry-run", "--json"))

	type planRow struct {
		Handle string `json:"handle"`
		Status string `json:"status"`
		Reason string `json:"reason,omitempty"`
	}
	type planEnvelope struct {
		DryRun bool      `json:"dry_run"`
		Rows   []planRow `json:"rows"`
	}
	var got planEnvelope
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &got))
	require.True(t, got.DryRun)
	require.Len(t, got.Rows, 2)

	byHandle := map[string]string{}
	reasons := map[string]string{}
	for _, r := range got.Rows {
		byHandle[r.Handle] = r.Status
		reasons[r.Handle] = r.Reason
	}
	require.Equal(t, "planned", byHandle["@m_pw"])
	require.Equal(t, "skip", byHandle["@m_skip"])
	require.Contains(t, reasons["@m_skip"], "password")
}

// TestCmdConfigKeyringMigrate_JSON_Apply: applied JSON envelope reports
// dry_run=false and one row per non-skipped source with migrated status
// (skipped sources omitted from the apply phase).
func TestCmdConfigKeyringMigrate_JSON_Apply(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@m_pw_apply",
		Type:     "postgres",
		Location: "postgres://alice:hunter2@db/sakila",
	}))

	require.NoError(t, tr.Exec("config", "keyring", "migrate", "--all", "--json"))

	type applyRow struct {
		Handle      string `json:"handle"`
		Status      string `json:"status"`
		NewLocation string `json:"new_location,omitempty"`
	}
	type applyEnvelope struct {
		DryRun bool       `json:"dry_run"`
		Rows   []applyRow `json:"rows"`
	}
	var got applyEnvelope
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &got))
	require.False(t, got.DryRun)
	require.Len(t, got.Rows, 1)
	require.Equal(t, "@m_pw_apply", got.Rows[0].Handle)
	require.Equal(t, "migrated", got.Rows[0].Status)
	require.Contains(t, got.Rows[0].NewLocation, "${keyring:")
}

// TestCmdConfigKeyringMigrate_RollbackOnSaveFailure exercises the
// failure path in applyMigratePlans: keyring write succeeds, then
// ConfigStore.Save fails. The expected response is:
//
//   - The in-memory Source.Location is restored to the pre-migrate
//     value (no half-mutated config persists).
//   - The "failed" row carries Status="failed" and Error contains
//     "rolled back" so JSON consumers can detect the path.
//   - The command surfaces a non-nil error.
//
// The rollback also calls kr.Delete on the orphan keyring entry, but
// the test doesn't assert that directly because the minted ID isn't
// observable from outside (orphan listing is pending — see #715).
func TestCmdConfigKeyringMigrate_RollbackOnSaveFailure(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	const origLoc = "postgres://alice:hunter2@db/sakila"
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@rb_src",
		Type:     "postgres",
		Location: origLoc,
	}))
	// Swap in a failing store AFTER initial setup so the source-add
	// Save() above still succeeds. Now any further Save errors.
	tr.Run.ConfigStore = &failingConfigStore{underlying: tr.Run.ConfigStore}

	err := tr.Exec("config", "keyring", "migrate", "--all", "--yes", "--json")
	require.Error(t, err)
	require.Contains(t, err.Error(), "one or more sources failed")

	// The in-memory source must reflect the rollback: Location is the
	// original DSN, not a ${keyring:...} placeholder.
	src, getErr := tr.Run.Config.Collection.Get("@rb_src")
	require.NoError(t, getErr)
	require.Equal(t, origLoc, src.Location,
		"rollback must restore the original Location")

	// JSON envelope carries the failed row with a "rolled back" hint.
	var env migrateRollbackEnvelope
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &env))
	require.False(t, env.DryRun)
	require.Len(t, env.Rows, 1)
	require.Equal(t, "@rb_src", env.Rows[0].Handle)
	require.Equal(t, "failed", env.Rows[0].Status)
	require.Contains(t, env.Rows[0].Error, "rolled back")
}

// TestCmdConfigKeyringMigrate_JSON_SkipsPrompt verifies the documented
// behavior in cmd_config_keyring_migrate.go: in --json mode the
// y/N confirmation prompt is bypassed entirely, because JSON consumers
// are non-interactive and --yes is implied.
//
// The test loads "n\n" into stdin (which would answer "no" if the
// prompt fired), then runs `migrate --all --json` WITHOUT --yes. If
// the prompt is honored, the "n" cancels migration and no entry is
// written. The assertion below catches that regression.
func TestCmdConfigKeyringMigrate_JSON_SkipsPrompt(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@js_prompt",
		Type:     "postgres",
		Location: "postgres://alice:hunter2@db/sakila",
	}))
	// Pipe "n\n" so the y/N prompt — if reached — would answer "no".
	tr.PipeStdin("n\n")

	require.NoError(t, tr.Exec("config", "keyring", "migrate", "--all", "--json"))

	var env migrateJSONEnvelope
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &env))
	require.False(t, env.DryRun)
	require.Len(t, env.Rows, 1)
	// If the prompt had run, "n" would have aborted with no rows.
	// "migrated" status proves the prompt was bypassed.
	require.Equal(t, "migrated", env.Rows[0].Status,
		"--json must bypass the y/N prompt; got status %q", env.Rows[0].Status)

	// Sanity: source's Location is now a placeholder.
	src, _ := tr.Run.Config.Collection.Get("@js_prompt")
	require.Contains(t, src.Location, "${keyring:")
}

// TestCmdConfigKeyringMigrate_ConfigFormatJSON_SkipsPrompt verifies
// that JSON output selected via the config "format" option behaves
// exactly like the --json flag: no preview envelope, no y/N prompt,
// a single valid JSON document on stdout. Regression test for #790,
// where outputFormatIsJSON checked only the --json flag while writer
// selection used the resolved format, so `sq config set format json`
// produced two concatenated JSON envelopes with a blocking prompt in
// between.
func TestCmdConfigKeyringMigrate_ConfigFormatJSON_SkipsPrompt(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	// Persist format=json to config, then start a fresh run that loads
	// it, mirroring `sq config set format json` followed by a separate
	// `sq config keyring migrate` invocation.
	require.NoError(t, tr.Exec("config", "set", "format", "json"))
	tr = testrun.New(th.Context, t, tr)

	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@cfg_json",
		Type:     "postgres",
		Location: "postgres://alice:hunter2@db/sakila",
	}))
	// Pipe "n\n" so the y/N prompt (if reached) would answer "no".
	tr.PipeStdin("n\n")

	// No --json flag and no --yes: JSON-ness must come from config.
	require.NoError(t, tr.Exec("config", "keyring", "migrate", "--all"))

	// Unmarshaling the entire stdout buffer fails if the output is two
	// concatenated documents or contains prompt text, so this asserts
	// "exactly one valid JSON document" as well as the envelope shape.
	var env migrateJSONEnvelope
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &env),
		"stdout must be a single valid JSON document; got: %s", tr.Out.String())
	require.False(t, env.DryRun)
	require.Len(t, env.Rows, 1)
	require.Equal(t, "migrated", env.Rows[0].Status,
		"config format=json must bypass the y/N prompt; got status %q", env.Rows[0].Status)

	// Sanity: source's Location is now a placeholder.
	src, err := tr.Run.Config.Collection.Get("@cfg_json")
	require.NoError(t, err)
	require.Contains(t, src.Location, "${keyring:")
}

// TestCmdConfigKeyringMigrate_PromptAbort verifies that answering "n" at the
// y/N confirmation leaves the source untouched and returns no error (abort is
// not a failure).
func TestCmdConfigKeyringMigrate_PromptAbort(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	const origLoc = "postgres://alice:hunter2@db/sakila"
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@pa_src",
		Type:     "postgres",
		Location: origLoc,
	}))

	// Answer "no" at the prompt.
	tr.PipeStdin("n\n")
	require.NoError(t, tr.Exec("config", "keyring", "migrate", "--all"))

	// Abort is not an error; the source Location must be unchanged.
	src, err := tr.Run.Config.Collection.Get("@pa_src")
	require.NoError(t, err)
	require.Equal(t, origLoc, src.Location, "abort must not modify the Location")

	// No keyring entry written: Location still lacks a ${keyring:...} placeholder.
	require.NotContains(t, src.Location, "${keyring:")

	// The plan/prompt text must appear so the user can see what would run.
	require.Contains(t, tr.Out.String(), "@pa_src")
}

// TestCmdConfigKeyringMigrate_PromptProceedEmptyAborts verifies that pressing
// Enter without typing confirms the [y/N] default of "no", aborting the
// migration without error.
func TestCmdConfigKeyringMigrate_PromptProceedEmptyAborts(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	const origLoc = "postgres://alice:hunter2@db/sakila"
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@pe_src",
		Type:     "postgres",
		Location: origLoc,
	}))

	// Empty input (just Enter) should default to "no".
	tr.PipeStdin("\n")
	require.NoError(t, tr.Exec("config", "keyring", "migrate", "--all"))

	src, err := tr.Run.Config.Collection.Get("@pe_src")
	require.NoError(t, err)
	require.Equal(t, origLoc, src.Location, "empty Enter must abort and leave Location unchanged")
	require.NotContains(t, src.Location, "${keyring:")
}

// TestCmdConfigKeyringMigrate_PromptProceed verifies that answering "y" at the
// confirmation prompt executes the migration.
func TestCmdConfigKeyringMigrate_PromptProceed(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	const origLoc = "postgres://alice:hunter2@db/sakila"
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@pp_src",
		Type:     "postgres",
		Location: origLoc,
	}))

	// Answer "yes" at the prompt.
	tr.PipeStdin("y\n")
	require.NoError(t, tr.Exec("config", "keyring", "migrate", "--all"))

	src, err := tr.Run.Config.Collection.Get("@pp_src")
	require.NoError(t, err)

	// Location must now be a bare ${keyring:<id>} placeholder.
	id := extractKeyringID(t, src.Location)

	// The keyring entry must hold the original DSN verbatim.
	got, err := gokeyring.Get("sq", id)
	require.NoError(t, err)
	require.Equal(t, origLoc, got)
}

// TestCmdConfigKeyringMigrate_SingleHandle verifies that passing a single
// @HANDLE migrates only that source, leaving other eligible sources unchanged.
func TestCmdConfigKeyringMigrate_SingleHandle(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	const loc1 = "postgres://alice:hunter2@db/sakila"
	const loc2 = "postgres://bob:secret99@db2/northwind"
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@h1",
		Type:     "postgres",
		Location: loc1,
	}))
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@h2",
		Type:     "postgres",
		Location: loc2,
	}))

	require.NoError(t, tr.Exec("config", "keyring", "migrate", "@h1", "--yes"))

	// @h1 must be migrated.
	src1, err := tr.Run.Config.Collection.Get("@h1")
	require.NoError(t, err)
	id := extractKeyringID(t, src1.Location)
	got, err := gokeyring.Get("sq", id)
	require.NoError(t, err)
	require.Equal(t, loc1, got)

	// @h2 must be untouched.
	src2, err := tr.Run.Config.Collection.Get("@h2")
	require.NoError(t, err)
	require.Equal(t, loc2, src2.Location, "@h2 must not be migrated when only @h1 was specified")
}

// TestCmdConfigKeyringMigrate_RequiresHandleOrAll verifies that running
// migrate with no handle and no --all flag returns an error.
func TestCmdConfigKeyringMigrate_RequiresHandleOrAll(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	err := tr.Exec("config", "keyring", "migrate")
	require.Error(t, err)
	require.Contains(t, err.Error(), "specify @HANDLE or --all")
}

// TestCmdConfigKeyringMigrate_UnknownHandle verifies that migrating a handle
// that does not exist in the collection returns an error.
func TestCmdConfigKeyringMigrate_UnknownHandle(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	err := tr.Exec("config", "keyring", "migrate", "@nonexistent", "--yes")
	require.Error(t, err)
}

// TestCmdConfigKeyringMigrate_MixedCollection verifies that --all --yes
// migrates only eligible sources and skips the rest with an informative
// reason. Three sources: one eligible, one without a password, one non-URL.
func TestCmdConfigKeyringMigrate_MixedCollection(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	const eligibleLoc = "postgres://alice:hunter2@db/sakila"
	const noPassLoc = "postgres://alice@db/sakila"
	const fileLoc = "/data/file.xlsx"

	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@mc_eligible",
		Type:     "postgres",
		Location: eligibleLoc,
	}))
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@mc_nopass",
		Type:     "postgres",
		Location: noPassLoc,
	}))
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@mc_file",
		Type:     "xlsx",
		Location: fileLoc,
	}))

	require.NoError(t, tr.Exec("config", "keyring", "migrate", "--all", "--yes"))

	// The eligible source must be migrated.
	srcEligible, err := tr.Run.Config.Collection.Get("@mc_eligible")
	require.NoError(t, err)
	id := extractKeyringID(t, srcEligible.Location)
	got, err := gokeyring.Get("sq", id)
	require.NoError(t, err)
	require.Equal(t, eligibleLoc, got)

	// The no-password source must be unchanged.
	srcNoPass, err := tr.Run.Config.Collection.Get("@mc_nopass")
	require.NoError(t, err)
	require.Equal(t, noPassLoc, srcNoPass.Location)

	// The file source must be unchanged.
	srcFile, err := tr.Run.Config.Collection.Get("@mc_file")
	require.NoError(t, err)
	require.Equal(t, fileLoc, srcFile.Location)

	// Skip reasons must appear in output.
	out := tr.Out.String()
	require.Contains(t, out, "no password")
	require.Contains(t, out, "not a URL")
}

// TestCmdConfigKeyringLs_Statuses verifies the three-state classification:
// referenced (in keyring + in config), orphan (in keyring, no config ref),
// and missing (in config, absent from keyring).
func TestCmdConfigKeyringLs_Statuses(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	// A referenced entry: source references it AND it exists in the keyring.
	require.NoError(t, keyring.NewStore().Set(th.Context, "ref1234567", "live-secret"))
	// An orphan: exists in the keyring, referenced by nothing.
	require.NoError(t, keyring.NewStore().Set(th.Context, "orphan23456", "stale-secret"))

	tr.Add(
		source.Source{
			Handle:   "@ref_pg",
			Type:     drivertype.Pg,
			Location: "${keyring:ref1234567}",
		},
		// A missing entry: source references it, but it is NOT in the keyring.
		source.Source{
			Handle:   "@missing_pg",
			Type:     drivertype.Pg,
			Location: "${keyring:gone7654321}",
		},
	)

	require.NoError(t, tr.Exec("config", "keyring", "ls"))
	out := tr.Out.String()
	require.Contains(t, out, "referenced")
	require.Contains(t, out, "orphan")
	require.Contains(t, out, "missing")
	require.Contains(t, out, "ref1234567")
	require.Contains(t, out, "orphan23456")
	require.Contains(t, out, "gone7654321")

	// Spec mandates sort order: referenced → orphan → missing.
	idxReferenced := strings.Index(out, "ref1234567")
	idxOrphan := strings.Index(out, "orphan23456")
	idxMissing := strings.Index(out, "gone7654321")
	require.Less(t, idxReferenced, idxOrphan, "referenced must appear before orphan")
	require.Less(t, idxOrphan, idxMissing, "orphan must appear before missing")
}

// TestCmdConfigKeyringRm_Completion_TolerantOfMalformedSource verifies
// that shell completion doesn't crash or short-circuit when one of the
// sources in the active collection has a malformed placeholder in its
// Location (which would make secret.ExtractRefs return an error). The
// completion function's `continue` on extract error is the load-bearing
// behavior here: completion is best-effort, not a config validator.
func TestCmdConfigKeyringRm_Completion_TolerantOfMalformedSource(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil).Add(
		source.Source{
			Handle:   "@good_completion",
			Type:     drivertype.Pg,
			Location: "postgres://alice:${keyring:goodid}@db/sakila",
		},
		// Unclosed "${" — ExtractRefs returns an error. The completion
		// function must skip this source rather than blow up or stop
		// producing candidates altogether.
		source.Source{
			Handle:   "@bad_completion",
			Type:     drivertype.Pg,
			Location: "postgres://alice:${env:UNCLOSED@db/sakila",
		},
	)

	got := testComplete(t, tr, "config", "keyring", "rm", "")
	require.Contains(t, got.values, "goodid",
		"malformed source must not block completion of healthy refs")
}

// TestCmdConfigKeyringPrune_MalformedSourceAbortsWithoutDeleting verifies
// that prune hard-fails when any source has a malformed placeholder in its
// Location, and that no keyring entries are deleted in that case. A source
// whose Location has both a valid ${keyring:...} ref and a malformed
// placeholder would have its valid ref silently dropped by ExtractRefs
// (all-or-nothing); prune would then misclassify the live entry as an
// orphan and delete it. Hard-failing before the delete loop prevents that
// data loss.
func TestCmdConfigKeyringPrune_MalformedSourceAbortsWithoutDeleting(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	kr := keyring.NewStore()
	// A live entry referenced by the malformed source (must NOT be deleted).
	require.NoError(t, kr.Set(th.Context, "keepme1234", "live"))
	// A genuine orphan (must also NOT be deleted, because prune must abort
	// before touching anything when the referenced set may be incomplete).
	require.NoError(t, kr.Set(th.Context, "orphan9999", "stale"))

	// Add a source whose Location contains both a valid ${keyring:...} ref
	// and an unclosed ${ that makes ExtractRefs return an error.
	tr.Add(source.Source{
		Handle:   "@bad_src",
		Type:     drivertype.Pg,
		Location: "postgres://u:${keyring:keepme1234}@host/db?x=${env:UNCLOSED",
	})

	err := tr.Exec("config", "keyring", "prune")
	require.Error(t, err, "prune must error when a source has a malformed Location")

	// Neither entry may have been deleted.
	v, resolveErr := kr.Resolve(th.Context, "keepme1234")
	require.NoError(t, resolveErr)
	require.Equal(t, "live", v, "referenced entry must survive")

	v, resolveErr = kr.Resolve(th.Context, "orphan9999")
	require.NoError(t, resolveErr)
	require.Equal(t, "stale", v, "orphan entry must survive when prune aborts early")
}

// TestCmdConfigKeyringLs_MalformedSourceErrors verifies that ls returns a
// non-nil error when any source has a malformed placeholder in its Location.
// An incomplete referenced set would misclassify live entries as orphans
// in the output, so hard-failing is correct.
func TestCmdConfigKeyringLs_MalformedSourceErrors(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	tr.Add(source.Source{
		Handle:   "@bad_ls",
		Type:     drivertype.Pg,
		Location: "postgres://u:${keyring:keepme1234}@host/db?x=${env:UNCLOSED",
	})

	err := tr.Exec("config", "keyring", "ls")
	require.Error(t, err, "ls must error when a source has a malformed Location")
}

func TestCmdConfigKeyringPrune_DeletesOrphans(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	kr := keyring.NewStore()
	require.NoError(t, kr.Set(th.Context, "keep1234567", "live"))   // referenced
	require.NoError(t, kr.Set(th.Context, "m4n8k2pxtz", "stale"))   // orphan (valid sq-minted opaque ID)
	require.NoError(t, kr.Set(th.Context, "named_orphan", "stale")) // orphan (named)

	tr.Add(source.Source{
		Handle:   "@keep_pg",
		Type:     drivertype.Pg,
		Location: "${keyring:keep1234567}",
	})

	require.NoError(t, tr.Exec("config", "keyring", "prune"))

	out := tr.Out.String()
	// Output must label both the opaque-ID and named-kind orphans.
	require.Contains(t, out, "(id)")
	require.Contains(t, out, "(named)")

	// Referenced entry survives.
	v, err := kr.Resolve(th.Context, "keep1234567")
	require.NoError(t, err)
	require.Equal(t, "live", v)

	// Both orphans (opaque-ID and named) are deleted.
	_, err = kr.Resolve(th.Context, "m4n8k2pxtz")
	require.ErrorIs(t, err, secret.ErrNotFound)
	_, err = kr.Resolve(th.Context, "named_orphan")
	require.ErrorIs(t, err, secret.ErrNotFound)
}

func TestCmdConfigKeyringPrune_DryRun(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	kr := keyring.NewStore()
	// m4n8k2pxtz is a valid sq-minted opaque ID (10-char Crockford).
	require.NoError(t, kr.Set(th.Context, "m4n8k2pxtz", "stale"))

	require.NoError(t, tr.Exec("config", "keyring", "prune", "--dry-run"))
	out := tr.Out.String()
	require.Contains(t, out, "m4n8k2pxtz")

	// Dry-run deletes nothing.
	v, err := kr.Resolve(th.Context, "m4n8k2pxtz")
	require.NoError(t, err)
	require.Equal(t, "stale", v)
}

// TestCmdConfigKeyringPrune_JSON verifies the JSON envelope emitted by
// "sq config keyring prune --json". It checks both the apply path
// (dry_run=false, status="deleted") and the dry-run path
// (dry_run=true, status="planned"), and that dry-run leaves entries intact.
func TestCmdConfigKeyringPrune_JSON(t *testing.T) {
	// --- Apply path ---
	t.Run("apply", func(t *testing.T) {
		gokeyring.MockInit()
		th := testh.New(t)
		tr := testrun.New(th.Context, t, nil)

		kr := keyring.NewStore()
		// m4n8k2pxtz is a valid sq-minted opaque ID (10-char Crockford).
		require.NoError(t, kr.Set(th.Context, "m4n8k2pxtz", "stale"))
		require.NoError(t, kr.Set(th.Context, "named_orphan", "stale"))

		require.NoError(t, tr.Exec("config", "keyring", "prune", "--json"))

		var env pruneJSONEnvelope
		require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &env))
		require.False(t, env.DryRun)
		require.Len(t, env.Rows, 2)

		byPath := map[string]pruneJSONRow{}
		for _, r := range env.Rows {
			byPath[r.Path] = r
		}
		require.Equal(t, output.KeyringKindID, byPath["m4n8k2pxtz"].Kind)
		require.Equal(t, output.KeyringPruneStatusDeleted, byPath["m4n8k2pxtz"].Status)
		require.Equal(t, output.KeyringKindNamed, byPath["named_orphan"].Kind)
		require.Equal(t, output.KeyringPruneStatusDeleted, byPath["named_orphan"].Status)

		// Both entries must be gone after apply.
		_, err := kr.Resolve(th.Context, "m4n8k2pxtz")
		require.ErrorIs(t, err, secret.ErrNotFound)
		_, err = kr.Resolve(th.Context, "named_orphan")
		require.ErrorIs(t, err, secret.ErrNotFound)
	})

	// --- Dry-run path ---
	t.Run("dry-run", func(t *testing.T) {
		gokeyring.MockInit()
		th := testh.New(t)
		tr := testrun.New(th.Context, t, nil)

		kr := keyring.NewStore()
		require.NoError(t, kr.Set(th.Context, "m4n8k2pxtz", "stale"))
		require.NoError(t, kr.Set(th.Context, "named_orphan", "stale"))

		require.NoError(t, tr.Exec("config", "keyring", "prune", "--dry-run", "--json"))

		var env pruneJSONEnvelope
		require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &env))
		require.True(t, env.DryRun)
		require.Len(t, env.Rows, 2)

		byPath := map[string]pruneJSONRow{}
		for _, r := range env.Rows {
			byPath[r.Path] = r
		}
		require.Equal(t, output.KeyringPruneStatusPlanned, byPath["m4n8k2pxtz"].Status)
		require.Equal(t, output.KeyringPruneStatusPlanned, byPath["named_orphan"].Status)

		// Dry-run must not delete anything.
		v, err := kr.Resolve(th.Context, "m4n8k2pxtz")
		require.NoError(t, err)
		require.Equal(t, "stale", v)
		v, err = kr.Resolve(th.Context, "named_orphan")
		require.NoError(t, err)
		require.Equal(t, "stale", v)
	})
}

// TestCmdConfigKeyringPrune_WriterError exercises the errz.Append branch
// where the output writer itself fails. The command must surface the writer
// error even when no individual deletion failed.
//
// The test pre-populates tr.Run.Writers with an erroring stub before calling
// Exec. preRun skips writer initialization when Writers is non-nil (see
// cli/run.go), so the stub is preserved through command execution. The prune
// command accesses only ru.Writers.Keyring, so nil values in the other
// Writers fields do not cause a panic.
//
// Note on Delete seam: forcing kr.Delete to fail is not cleanly testable via
// the zalando/go-keyring mock. MockInit installs an in-memory map backend;
// Delete on a present key always succeeds, and Store.Delete treats not-found
// as success explicitly. Adding a seam for a forced Delete failure would
// require production code changes purely for testing, so the partial-delete-
// failure path is not tested at the kr.Delete level.
func TestCmdConfigKeyringPrune_WriterError(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	kr := keyring.NewStore()
	require.NoError(t, kr.Set(th.Context, "orphan23456", "stale"))

	// Pre-populate Writers with a stub so preRun skips writer
	// initialization. Only Keyring needs to be set; the prune
	// command touches no other writer field.
	tr.Run.Writers = &output.Writers{Keyring: &failingKeyringWriter{}}

	err := tr.Exec("config", "keyring", "prune")
	require.Error(t, err)
	require.Contains(t, err.Error(), "simulated prune writer failure")
}

// failingKeyringWriter is a KeyringWriter stub whose Prune always errors,
// used by TestCmdConfigKeyringPrune_WriterError.
type failingKeyringWriter struct{}

func (w *failingKeyringWriter) List(_ []output.KeyringRef) error { return nil }
func (w *failingKeyringWriter) Get(_, _ string, _ bool) error    { return nil }
func (w *failingKeyringWriter) Created(_ string) error           { return nil }
func (w *failingKeyringWriter) Updated(_ string) error           { return nil }
func (w *failingKeyringWriter) Rm(_ string) error                { return nil }

func (w *failingKeyringWriter) Migrate(_ []output.KeyringMigrateRow, _ bool) error {
	return nil
}

func (w *failingKeyringWriter) Prune(_ []output.KeyringPruneRow, _ bool) error {
	return errz.New("simulated prune writer failure")
}
