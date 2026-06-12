package v0_54_0_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/config/yamlstore"
	v0_34_0 "github.com/neilotoole/sq/cli/config/yamlstore/upgrades/v0.34.0" //nolint:revive
	v0_54_0 "github.com/neilotoole/sq/cli/config/yamlstore/upgrades/v0.54.0" //nolint:revive
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/tu"
)

// TestUpgrade verifies the redact → secrets.reveal rename and polarity
// flip across both the top-level options block and per-source options.
func TestUpgrade(t *testing.T) {
	log := lgt.New(t)
	ctx := lg.NewContext(context.Background(), log)

	const (
		prevVers = "v0.53.0"
		nextVers = "v0.54.0"
	)

	testh.SetBuildVersion(t, nextVers)

	cfgDir := tu.DirCopy(t, "testdata")
	t.Setenv(config.EnvarConfig, cfgDir)

	cfgFilePath := filepath.Join(cfgDir, "sq.yml")

	gotPrevVers, err := yamlstore.LoadVersionFromFile(cfgFilePath)
	require.NoError(t, err)
	require.Equal(t, prevVers, gotPrevVers)

	upgrades := yamlstore.UpgradeRegistry{
		v0_34_0.Version: v0_34_0.Upgrade,
		v0_54_0.Version: v0_54_0.Upgrade,
	}

	optsReg := &options.Registry{}
	cli.RegisterDefaultOpts(optsReg)

	cfg, cfgStore, err := yamlstore.Load(ctx, nil, optsReg, upgrades)
	require.NoError(t, err)

	require.Equal(t, cfgDir, cfgStore.Location())
	require.Equal(t, nextVers, cfg.Version)

	// Top-level: redact: false → secrets.reveal: true.
	require.True(t, secret.OptSecretsReveal.Get(cfg.Options),
		"top-level redact:false should migrate to secrets.reveal:true")
	require.NotContains(t, cfg.Options, "redact",
		"the legacy 'redact' key should be removed from top-level options")

	// Per-source: redact: true (default) is dropped (no key written),
	// since secrets.reveal defaults to false which is the same semantic.
	csvSrc := cfg.Collection.Sources()[1]
	require.Equal(t, true, csvSrc.Options[driver.OptIngestHeader.Key()],
		"unrelated source options should survive the upgrade")
	require.NotContains(t, csvSrc.Options, "redact",
		"per-source 'redact' should be dropped on upgrade")
	require.NotContains(t, csvSrc.Options, "secrets.reveal",
		"per-source redact:true (default) should drop, not write secrets.reveal:false")

	// Per-source: redact: false → secrets.reveal: true on the source.
	xlsxSrc := cfg.Collection.Sources()[2]
	require.Equal(t, true, xlsxSrc.Options[secret.OptSecretsReveal.Key()],
		"per-source redact:false should migrate to secrets.reveal:true")
	require.NotContains(t, xlsxSrc.Options, "redact",
		"per-source 'redact' should be removed on upgrade")

	// Before any upgrade step runs, a verbatim backup of the
	// pre-upgrade config must be written alongside sq.yml, named for
	// the version the config was upgraded from. The name must not end
	// in ".sq.yml", or ext config loading would pick it up.
	backupRaw, err := os.ReadFile(filepath.Join(cfgDir, "sq.v0.53.0.bak.yml"))
	require.NoError(t, err, "pre-upgrade backup file must exist")
	origCfgRaw, err := os.ReadFile(filepath.Join("testdata", "sq.yml"))
	require.NoError(t, err)
	require.Equal(t, string(origCfgRaw), string(backupRaw),
		"backup must be byte-identical to the pre-upgrade config")
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(filepath.Join(cfgDir, "sq.v0.53.0.bak.yml"))
		require.NoError(t, err)
		require.Equal(t, ioz.RWPerms, fi.Mode().Perm(),
			"backup may contain credentials; must be user-only readable")
	}

	wantCfgRaw, err := os.ReadFile(filepath.Join("testdata", "want.sq.yml"))
	require.NoError(t, err)

	gotCfgRaw, err := os.ReadFile(filepath.Join(cfgDir, "sq.yml"))
	require.NoError(t, err)

	t.Logf("Output written to: %s", filepath.Join(cfgDir, "sq.yml"))

	wantLines := strings.Split(strings.TrimSpace(string(wantCfgRaw)), "\n")
	gotLines := strings.Split(strings.TrimSpace(string(gotCfgRaw)), "\n")
	require.Equal(t, wantLines, gotLines)
}

// TestUpgrade_EscapesLocationDollars verifies that the upgrade escapes
// '$' as '$$' in any source location that would not survive v0.54.0
// placeholder-template interpretation byte-identically: locations
// containing a well-formed ${scheme:path} ref (would be silently
// substituted at connect), malformed placeholder syntax (would fail
// every connect), or a literal "$$" (would be unescaped at connect).
// Locations whose only dollars are lone '$' characters already pass
// through verbatim and must be left untouched. See
// https://github.com/neilotoole/sq/issues/782.
func TestUpgrade_EscapesLocationDollars(t *testing.T) {
	log := lgt.New(t)
	ctx := lg.NewContext(context.Background(), log)

	tests := []struct {
		name string
		loc  string
		want string
	}{
		{
			name: "no dollar untouched",
			loc:  "postgres://sakila:p_ssW0rd@localhost/sakila",
			want: "postgres://sakila:p_ssW0rd@localhost/sakila",
		},
		{
			name: "lone dollar untouched",
			loc:  "postgres://sakila:p$ssW0rd@localhost/sakila",
			want: "postgres://sakila:p$ssW0rd@localhost/sakila",
		},
		{
			name: "malformed placeholder escaped",
			loc:  "postgres://sakila:p${ss}W0rd@localhost/sakila",
			want: "postgres://sakila:p$${ss}W0rd@localhost/sakila",
		},
		{
			name: "well-formed placeholder escaped",
			loc:  "postgres://sakila:${env:HOME}@localhost/sakila",
			want: "postgres://sakila:$${env:HOME}@localhost/sakila",
		},
		{
			name: "literal double dollar escaped",
			loc:  "postgres://sakila:p$$wd@localhost/sakila",
			want: "postgres://sakila:p$$$$wd@localhost/sakila",
		},
		{
			name: "file path with lone dollar untouched",
			loc:  "/Users/neilotoole/sq/file$.csv",
			want: "/Users/neilotoole/sq/file$.csv",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(`config.version: v0.53.0
collection:
  sources:
  - handle: "@src"
    driver: postgres
    location: ` + `"` + tc.loc + `"` + `
`)

			out, err := v0_54_0.Upgrade(ctx, in)
			require.NoError(t, err)

			var m map[string]any
			require.NoError(t, ioz.UnmarshallYAML(out, &m))
			src := m["collection"].(map[string]any)["sources"].([]any)[0].(map[string]any)
			got, ok := src["location"].(string)
			require.True(t, ok)
			require.Equal(t, tc.want, got)

			// The connect-path invariant: the stored location must parse
			// cleanly with zero refs, and unescape to the original bytes,
			// so the driver receives exactly what worked in v0.53.0.
			refs, err := secret.ExtractRefs(got)
			require.NoError(t, err)
			require.Empty(t, refs)
			require.Equal(t, tc.loc, secret.Unescape(got))
		})
	}
}

// TestUpgrade_CorruptLocation verifies the upgrade's handling of
// degenerate source locations.
//
// A missing location aborts the upgrade before anything is written:
// such a config never loaded (validSource rejects empty locations), so
// erroring can't break a working config, and it matches the
// corrupt-shape precedent elsewhere in this upgrade.
//
// A non-string location is skipped, NOT an error: goccy-yaml coerces
// scalar values (int/float/bool) into string fields, so 'location:
// 123' loaded fine on v0.53.0 as "123" and erroring here would brick
// a previously working config. Skipping is provably safe for escaping
// purposes: a YAML scalar that parses as non-string can't contain '$'
// (any '$'-bearing value parses as a string).
func TestUpgrade_CorruptLocation(t *testing.T) {
	log := lgt.New(t)
	ctx := lg.NewContext(context.Background(), log)

	t.Run("missing location errors", func(t *testing.T) {
		in := []byte(`config.version: v0.53.0
collection:
  sources:
  - handle: "@src"
    driver: postgres
`)
		_, err := v0_54_0.Upgrade(ctx, in)
		require.Error(t, err)
		require.Contains(t, err.Error(), "corrupt config")
		require.Contains(t, err.Error(), "location")
	})

	t.Run("non-string location skipped", func(t *testing.T) {
		in := []byte(`config.version: v0.53.0
collection:
  sources:
  - handle: "@src"
    driver: postgres
    location: 123
`)
		out, err := v0_54_0.Upgrade(ctx, in)
		require.NoError(t, err,
			"non-string location must not abort: it loaded fine on v0.53.0 (goccy coerces scalars)")

		var m map[string]any
		require.NoError(t, ioz.UnmarshallYAML(out, &m))
		require.Equal(t, v0_54_0.Version, m["config.version"])
		src := m["collection"].(map[string]any)["sources"].([]any)[0].(map[string]any)
		require.EqualValues(t, 123, src["location"], "non-string location value must be untouched")
	})
}

// TestUpgrade_NoRedactKey_IsIdempotent verifies that running Upgrade
// against a config that has no "redact" key (whether already-migrated
// or never had one) leaves the user-visible options untouched. Only
// the config.version stamp changes.
func TestUpgrade_NoRedactKey_IsIdempotent(t *testing.T) {
	log := lgt.New(t)
	ctx := lg.NewContext(context.Background(), log)

	in := []byte(`config.version: v0.53.0
options:
  format: json
  secrets.reveal: true
collection:
  active.source: "@prod/pg"
  sources:
  - handle: "@prod/pg"
    driver: postgres
    location: postgres://sakila:p_ssW0rd@localhost/sakila
`)

	out, err := v0_54_0.Upgrade(ctx, in)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, ioz.UnmarshallYAML(out, &m))

	require.Equal(t, v0_54_0.Version, m["config.version"],
		"config.version is stamped")

	opts, ok := m["options"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, opts["secrets.reveal"],
		"existing secrets.reveal:true must survive untouched")
	require.NotContains(t, opts, "redact",
		"no spurious 'redact' key should appear")
}

// TestUpgrade_NonBoolRedact_DropsAndWarns verifies that a corrupt
// (non-bool) value for "redact" is dropped without erroring out.
// The user-visible effect is that secrets.reveal falls back to its
// default; the runtime logs a warning at upgrade time so a
// post-upgrade redaction-behavior change can be traced.
func TestUpgrade_NonBoolRedact_DropsAndWarns(t *testing.T) {
	log := lgt.New(t)
	ctx := lg.NewContext(context.Background(), log)

	in := []byte(`config.version: v0.53.0
options:
  format: json
  redact: "true"
collection:
  active.source: "@prod/pg"
  sources:
  - handle: "@prod/pg"
    driver: postgres
    location: postgres://alice:p_ssW0rd@localhost/sakila
    options:
      redact: 42
`)

	out, err := v0_54_0.Upgrade(ctx, in)
	require.NoError(t, err, "upgrade should not error on a non-bool redact value")

	var m map[string]any
	require.NoError(t, ioz.UnmarshallYAML(out, &m))

	opts := m["options"].(map[string]any)
	require.NotContains(t, opts, "redact",
		"top-level non-bool 'redact' must be dropped")
	require.NotContains(t, opts, "secrets.reveal",
		"non-bool 'redact' must fall back to default, not write secrets.reveal")

	srcOpts := m["collection"].(map[string]any)["sources"].([]any)[0].(map[string]any)["options"].(map[string]any)
	require.NotContains(t, srcOpts, "redact",
		"per-source non-bool 'redact' must be dropped")
	require.NotContains(t, srcOpts, "secrets.reveal")
}

// TestUpgrade_CorruptOptionsShape verifies that a present-but-wrong-
// shape "options" entry surfaces as an error rather than silently
// stamping config.version and skipping the redact translation.
func TestUpgrade_CorruptOptionsShape(t *testing.T) {
	log := lgt.New(t)
	ctx := lg.NewContext(context.Background(), log)

	in := []byte(`config.version: v0.53.0
options: "not a map"
`)

	_, err := v0_54_0.Upgrade(ctx, in)
	require.Error(t, err)
	require.Contains(t, err.Error(), "corrupt config")
}
