package cli_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
)

func TestCmdConfigKeyringSet_ExplicitValue(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	err := tr.Exec("config", "keyring", "set", "@sakila/password", "hunter2")
	require.NoError(t, err)

	got, err := gokeyring.Get("sq", "@sakila/password")
	require.NoError(t, err)
	require.Equal(t, "hunter2", got)
}

func TestCmdConfigKeyringSet_PromptedFromStdin(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	// Pipe a password through stdin; matches the cmd_add test pattern.
	tmp, err := os.CreateTemp(t.TempDir(), "pw")
	require.NoError(t, err)
	_, err = tmp.WriteString("hunter2\n")
	require.NoError(t, err)
	_, err = tmp.Seek(0, 0)
	require.NoError(t, err)
	tr.Run.Stdin = tmp

	err = tr.Exec("config", "keyring", "set", "@sakila/password", "-p")
	require.NoError(t, err)

	got, err := gokeyring.Get("sq", "@sakila/password")
	require.NoError(t, err)
	require.Equal(t, "hunter2", got)
}

func TestCmdConfigKeyringSet_RequiresValueOrFlag(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	// No VALUE arg, no -p flag.
	err := tr.Exec("config", "keyring", "set", "@sakila/password")
	require.Error(t, err)
}

func TestCmdConfigKeyringGet_WithoutRevealPrintsMetadataOnly(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "@sakila/password", "hunter2"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "keyring", "get", "@sakila/password"))
	require.NotContains(t, tr.Out.String(), "hunter2")
	require.Contains(t, tr.Out.String(), "@sakila/password")
}

func TestCmdConfigKeyringGet_WithRevealPrintsValue(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "@sakila/password", "hunter2"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "keyring", "get", "@sakila/password", "--reveal"))
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
	require.NoError(t, gokeyring.Set("sq", "@sakila/password", "hunter2"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "keyring", "rm", "@sakila/password"))

	_, err := gokeyring.Get("sq", "@sakila/password")
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
	require.NoError(t, tr.Exec("config", "keyring", "ls"))
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

	require.NoError(t, tr.Exec("config", "keyring", "ls"))
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

	require.NoError(t, tr.Exec("config", "keyring", "ls"))
	lines := strings.Split(strings.TrimRight(tr.Out.String(), "\n"), "\n")
	require.Len(t, lines, 1, "only the keyring placeholder should produce a row")
	require.Contains(t, lines[0], "abc1234567")
	require.Contains(t, lines[0], "@compo")
	require.Contains(t, lines[0], "postgres")
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
	}
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &got))
	require.Len(t, got, 2, "env source must not produce a row")
	require.Equal(t, "abc1", got[0].Path)
	require.Equal(t, "@a_js", got[0].Handle)
	require.Equal(t, "postgres", got[0].Driver)
	require.Equal(t, "abc2", got[1].Path)
	require.Equal(t, "@b_js", got[1].Handle)
	require.Equal(t, "mysql", got[1].Driver)
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

// TestCmdConfigKeyringSet_JSON: explicit set with --json emits a
// confirmation object.
func TestCmdConfigKeyringSet_JSON(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "keyring", "set", "abc_set_js", "v", "--json"))

	var got map[string]any
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &got))
	require.Equal(t, "abc_set_js", got["path"])
	require.Equal(t, true, got["set"])

	// Side effect: keyring actually got written to.
	v, err := gokeyring.Get("sq", "abc_set_js")
	require.NoError(t, err)
	require.Equal(t, "v", v)
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
