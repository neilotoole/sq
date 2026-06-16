package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// TestBug520_LsShowsPassword tests https://github.com/neilotoole/sq/issues/520.
func TestBug520_LsShowsPassword(t *testing.T) {
	t.Parallel()

	const password = "p_ssW0rd"

	testCases := []struct {
		name string
		loc  string
	}{
		{
			name: "sqlserver: no database param",
			loc:  "sqlserver://sa:p_ssW0rd@localhost",
		},
		{
			name: "sqlserver: with database param",
			loc:  "sqlserver://sa:p_ssW0rd@localhost?database=test",
		},
		{
			name: "sqlserver: no database param, with other param",
			loc:  "sqlserver://sa:p_ssW0rd@localhost?encrypt=true",
		},
		{
			name: "postgres: for regression/sanity check",
			loc:  "postgres://sakila:p_ssW0rd@localhost/sakila",
		},
		{
			name: "mysql: for regression/sanity check",
			loc:  "mysql://sakila:p_ssW0rd@localhost/sakila",
		},
		{
			name: "oracle: user/pass@host/service",
			loc:  "oracle://sakila:p_ssW0rd@localhost:1521/ORCL",
		},
		// NOTE: SQLite is not included because it uses file paths, not URLs with passwords.
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr := testrun.New(t.Context(), t, nil)

			require.NoError(t, tr.Exec("add", tc.loc, "--skip-verify"))
			require.NotContains(t, tr.OutString(), password, "should not print password")

			require.NoError(t, tr.Reset().Exec("ls"))
			require.NotContains(t, tr.OutString(), password, "should not print password")

			require.NoError(t, tr.Reset().Exec("ls", "-v", "--no-redact"))
			require.Contains(t, tr.OutString(), password, "should print password with --no-redact")

			require.NoError(t, tr.Reset().Exec("ls", "-v", "--reveal"))
			require.Contains(t, tr.OutString(), password, "should print password with --reveal")
		})
	}
}

// TestRevealFlagConfigPrecedence tests
// https://github.com/neilotoole/sq/issues/785: an explicitly set
// --reveal or --no-redact flag overrides config secrets.reveal in both
// directions. In particular, --reveal=false (or --no-redact=false) must
// force redaction even when config has secrets.reveal=true.
func TestRevealFlagConfigPrecedence(t *testing.T) {
	t.Parallel()

	const (
		password = "p_ssW0rd"
		loc      = "postgres://sakila:" + password + "@localhost/sakila"
	)

	testCases := []struct {
		name         string
		configReveal string // empty means option not set in config
		args         []string
		wantReveal   bool
	}{
		{
			name:       "no_flag_default_config",
			wantReveal: false,
		},
		{
			name:         "no_flag_config_true",
			configReveal: "true",
			wantReveal:   true,
		},
		{
			name:       "reveal_default_config",
			args:       []string{"--reveal"},
			wantReveal: true,
		},
		{
			name:         "reveal_config_false",
			configReveal: "false",
			args:         []string{"--reveal"},
			wantReveal:   true,
		},
		{
			name:         "no_redact_config_false",
			configReveal: "false",
			args:         []string{"--no-redact"},
			wantReveal:   true,
		},
		{
			name:         "reveal_false_config_true",
			configReveal: "true",
			args:         []string{"--reveal=false"},
			wantReveal:   false,
		},
		{
			name:         "no_redact_false_config_true",
			configReveal: "true",
			args:         []string{"--no-redact=false"},
			wantReveal:   false,
		},
		{
			// Both flags explicitly set with conflicting values: true wins,
			// preserving union composition (a script that bakes in
			// --no-redact composed with a user-added --reveal=false, or
			// vice versa).
			name:         "conflicting_flags_true_wins",
			configReveal: "false",
			args:         []string{"--reveal=false", "--no-redact"},
			wantReveal:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr := testrun.New(t.Context(), t, nil)
			require.NoError(t, tr.Exec("add", loc, "--skip-verify"))

			if tc.configReveal != "" {
				require.NoError(t, tr.Reset().Exec(
					"config", "set", "secrets.reveal", tc.configReveal))
			}

			args := append([]string{"ls", "-v"}, tc.args...)
			require.NoError(t, tr.Reset().Exec(args...))
			if tc.wantReveal {
				require.Contains(t, tr.OutString(), password,
					"password should be revealed")
			} else {
				require.NotContains(t, tr.OutString(), password,
					"password should be redacted")
			}
		})
	}
}

// TestExpandRevealMatrix verifies the cross-product of --expand and
// --reveal across the three placeholder shapes (whole-DSN, composition,
// inline-plaintext) on `sq ls -v`. This is the canonical proof of the
// matrix described in gh-729.
func TestExpandRevealMatrix(t *testing.T) {
	// Not parallel: subtests use t.Setenv which requires a non-parallel
	// ancestor.
	const (
		keyringID    = "expand_matrix_kr"
		fullDSN      = "postgres://alice:hunter2@db/sakila"
		envVar       = "SQ_TEST_EXPAND_DBPW"
		envPass      = "envhunter"
		composeLoc   = "postgres://alice:${env:" + envVar + "}@db/sakila"
		inlineLoc    = "postgres://alice:hunter2@db/sakila"
		redactedPass = "xxxxx"
	)

	keyringLoc := "${keyring:" + keyringID + "}"

	type row struct {
		shape         string
		loc           string
		none          []string
		reveal        []string
		expand        []string
		revealExpd    []string
		noneNot       []string
		revealNot     []string
		expandNot     []string
		revealExpdNot []string
	}

	cases := []row{
		{
			// Shape A: the whole Location is a keyring placeholder. Without
			// --expand the placeholder is shown verbatim (it is not a URL so
			// there is no userinfo to redact). With --expand the keyring entry
			// is resolved then the password in the resolved DSN is redacted
			// (or shown with --reveal).
			shape:      "A_wholeDSN_keyring",
			loc:        keyringLoc,
			none:       []string{keyringLoc},
			reveal:     []string{keyringLoc},
			expand:     []string{"postgres://alice:" + redactedPass + "@db/sakila"},
			revealExpd: []string{fullDSN},
			noneNot:    []string{"hunter2"},
			revealNot:  []string{"hunter2"},
			expandNot:  []string{"hunter2"},
		},
		{
			// Shape B: the placeholder is embedded in the password field of a
			// URL. Without --expand the redactor masks the password position
			// (the placeholder text is consumed along with the credential).
			// With --reveal the raw location is shown verbatim. With
			// --expand the env var is resolved and then the password redacted
			// (or shown with --reveal).
			shape:      "B_composition_env",
			loc:        composeLoc,
			none:       []string{"postgres://alice:" + redactedPass + "@db/sakila"},
			reveal:     []string{composeLoc},
			expand:     []string{"postgres://alice:" + redactedPass + "@db/sakila"},
			revealExpd: []string{"postgres://alice:" + envPass + "@db/sakila"},
			noneNot:    []string{envPass},
			revealNot:  []string{envPass},
			expandNot:  []string{envPass},
		},
		{
			// Shape C: the password is inline plaintext with no placeholder.
			// Without --reveal the password is always redacted, regardless of
			// --expand (there is nothing to expand).
			shape:      "C_inline",
			loc:        inlineLoc,
			none:       []string{"postgres://alice:" + redactedPass + "@db/sakila"},
			reveal:     []string{inlineLoc},
			expand:     []string{"postgres://alice:" + redactedPass + "@db/sakila"},
			revealExpd: []string{inlineLoc},
			noneNot:    []string{"hunter2"},
			expandNot:  []string{"hunter2"},
		},
	}

	combos := []struct {
		name    string
		args    []string
		getWant func(r row) (want, wantNot []string)
	}{
		{"none", []string{"ls", "-v"}, func(r row) ([]string, []string) { return r.none, r.noneNot }},
		{"reveal", []string{"ls", "-v", "--reveal"}, func(r row) ([]string, []string) { return r.reveal, r.revealNot }},
		{"expand", []string{"ls", "-v", "--expand"}, func(r row) ([]string, []string) { return r.expand, r.expandNot }},
		{
			"reveal+expand",
			[]string{"ls", "-v", "--reveal", "--expand"},
			func(r row) ([]string, []string) { return r.revealExpd, r.revealExpdNot },
		},
	}

	for _, c := range cases {
		t.Run(c.shape, func(t *testing.T) {
			// Per-shape isolation: mock keyring and set env before each
			// shape block so cases do not leak into each other.
			gokeyring.MockInit()
			require.NoError(t, gokeyring.Set("sq", keyringID, fullDSN))
			t.Setenv(envVar, envPass)

			for _, combo := range combos {
				t.Run(combo.name, func(t *testing.T) {
					tr := testrun.New(t.Context(), t, nil)
					require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
						Handle:   "@matrix",
						Type:     drivertype.Pg,
						Location: c.loc,
					}))

					require.NoError(t, tr.Exec(combo.args...))
					got := tr.OutString()

					want, wantNot := combo.getWant(c)
					for _, sub := range want {
						require.Contains(t, got, sub,
							"%s/%s must contain %q\noutput: %s",
							c.shape, combo.name, sub, got)
					}
					for _, sub := range wantNot {
						require.NotContains(t, got, sub,
							"%s/%s must not contain %q\noutput: %s",
							c.shape, combo.name, sub, got)
					}
				})
			}
		})
	}
}

// TestExpandLenient_PartialFailure verifies that when one source's
// resolver fails (missing keyring entry) during `sq ls -v --expand`,
// that source's row falls back to its verbatim placeholder while the
// other sources expand normally, and the command exits 0.
func TestExpandLenient_PartialFailure(t *testing.T) {
	// Not parallel: gokeyring.MockInit() mutates a process-global
	// keyring backend, and parallelizing this with another
	// MockInit-using test in the future would race.

	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "ok_id",
		"postgres://u:pw@h/db"))
	// "missing_id" intentionally absent.

	tr := testrun.New(t.Context(), t, nil)
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@ok",
		Type:     drivertype.Pg,
		Location: "${keyring:ok_id}",
	}))
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@missing",
		Type:     drivertype.Pg,
		Location: "${keyring:missing_id}",
	}))

	require.NoError(t, tr.Exec("ls", "-v", "--expand"),
		"per-source resolver failure must not abort the command")
	got := tr.OutString()
	require.Contains(t, got, "postgres://u:xxxxx@h/db",
		"@ok must expand (with redaction)")
	require.Contains(t, got, "${keyring:missing_id}",
		"@missing must show verbatim placeholder")
}

// TestExpand_PerCommandWiring is a smoke test for the --expand flag across
// the display commands. Expansion is applied centrally by the writer-layer
// expand decorators (see expand_writer.go); this test catches a future
// refactor that breaks the wiring for one of these commands, even though
// the expand unit tests would still pass.
//
// Shape A (whole-DSN keyring placeholder) is used because it produces the
// most visible difference: without --expand the verbatim placeholder string
// appears; with --expand the resolved DSN (with the password redacted) appears.
//
// Commands tested: sq src --json, sq ls --json, sq ping --json.
//
// sq add, sq mv, and sq inspect are not exercised here:
//   - sq add and sq mv have awkward bootstrap requirements when testing
//     with keyring placeholders (the added source must already exist in
//     the collection before the command echo can be asserted).
//   - sq inspect is covered by its own dedicated regression test:
//     TestInspect_LocationOverride_NoLeak in cmd_inspect_test.go.
func TestExpand_PerCommandWiring(t *testing.T) {
	// Not parallel: gokeyring.MockInit() mutates a process-global
	// keyring backend, and parallelizing this with another
	// MockInit-using test in the future would race.
	const (
		keyringID    = "wiring_smoke_kr"
		fullDSN      = "postgres://alice:hunter2@db.example.com/sakila"
		placeholder  = "${keyring:" + keyringID + "}"
		redactedPass = "xxxxx"
	)

	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", keyringID, fullDSN))

	makeSrc := func() *source.Source {
		return &source.Source{
			Handle:   "@h",
			Type:     drivertype.Pg,
			Location: placeholder,
		}
	}

	t.Run("sq_src_json", func(t *testing.T) {
		tr := testrun.New(t.Context(), t, nil)
		require.NoError(t, tr.Run.Config.Collection.Add(makeSrc()))
		_, err := tr.Run.Config.Collection.SetActive("@h", false)
		require.NoError(t, err)

		require.NoError(t, tr.Exec("src", "--json", "--expand"),
			"sq src --json --expand must succeed")
		out := tr.OutString()
		require.Contains(t, out, "postgres://alice:"+redactedPass+"@db.example.com/sakila",
			"sq src --json --expand must show the resolved, redacted DSN")
		require.NotContains(t, out, placeholder,
			"sq src --json --expand must not show the verbatim placeholder")
		require.NotContains(t, out, "hunter2",
			"sq src --json --expand must not leak the plaintext secret")
	})

	t.Run("sq_ls_json", func(t *testing.T) {
		tr := testrun.New(t.Context(), t, nil)
		require.NoError(t, tr.Run.Config.Collection.Add(makeSrc()))

		require.NoError(t, tr.Exec("ls", "--json", "--expand"),
			"sq ls --json --expand must succeed")
		out := tr.OutString()
		require.Contains(t, out, "postgres://alice:"+redactedPass+"@db.example.com/sakila",
			"sq ls --json --expand must show the resolved, redacted DSN")
		require.NotContains(t, out, placeholder,
			"sq ls --json --expand must not show the verbatim placeholder")
		require.NotContains(t, out, "hunter2",
			"sq ls --json --expand must not leak the plaintext secret")
	})

	t.Run("sq_ping_json", func(t *testing.T) {
		tr := testrun.New(t.Context(), t, nil)
		require.NoError(t, tr.Run.Config.Collection.Add(makeSrc()))

		// Ping will fail to connect (the postgres DSN is not reachable in CI),
		// but the JSON output is still written before the connection error. The
		// test does not require ping success; it requires that the Location
		// field in the JSON output is the resolved, redacted form.
		_ = tr.Exec("ping", "@h", "--json", "--expand")
		out := tr.OutString()
		require.Contains(t, out, "postgres://alice:"+redactedPass+"@db.example.com/sakila",
			"sq ping --json --expand must show the resolved, redacted DSN")
		require.NotContains(t, out, placeholder,
			"sq ping --json --expand must not show the verbatim placeholder")
		require.NotContains(t, out, "hunter2",
			"sq ping --json --expand must not leak the plaintext secret")
	})

	t.Run("sq_rm_json", func(t *testing.T) {
		// rm reloads config from the store, so the source must be
		// persisted (TestRun.Add saves), not just added in-memory.
		tr := testrun.New(t.Context(), t, nil).Add(*makeSrc())

		// rm's --json output for removed sources is verbose-gated.
		require.NoError(t, tr.Exec("rm", "@h", "--json", "-v", "--expand"),
			"sq rm --json -v --expand must succeed")
		out := tr.OutString()
		require.Contains(t, out, "postgres://alice:"+redactedPass+"@db.example.com/sakila",
			"sq rm --json --expand must show the resolved, redacted DSN")
		require.NotContains(t, out, placeholder,
			"sq rm --json --expand must not show the verbatim placeholder")
		require.NotContains(t, out, "hunter2",
			"sq rm --json --expand must not leak the plaintext secret")
	})

	t.Run("sq_mv_json", func(t *testing.T) {
		// mv reloads config from the store, so the source must be
		// persisted (TestRun.Add saves), not just added in-memory.
		tr := testrun.New(t.Context(), t, nil).Add(*makeSrc())

		require.NoError(t, tr.Exec("mv", "@h", "@h2", "--json", "--expand"),
			"sq mv --json --expand must succeed")
		out := tr.OutString()
		require.Contains(t, out, "postgres://alice:"+redactedPass+"@db.example.com/sakila",
			"sq mv --json --expand must show the resolved, redacted DSN for the moved source")
		require.NotContains(t, out, placeholder,
			"sq mv --json --expand must not show the verbatim placeholder")
		require.NotContains(t, out, "hunter2",
			"sq mv --json --expand must not leak the plaintext secret")
	})
}

// TestExpand_NoOp_OnNonDisplayCommand verifies that --expand is
// accepted (silent no-op) on a command that does not print Location,
// so a global alias like `alias sq='sq --reveal --expand'` is safe.
func TestExpand_NoOp_OnNonDisplayCommand(t *testing.T) {
	t.Parallel()

	tr := testrun.New(t.Context(), t, nil)
	require.NoError(t, tr.Exec("config", "ls", "--expand"))
	// Mere absence of an error is the assertion; cobra would reject
	// an unknown flag with a non-nil error.
}

// TestRedactFlags_Union verifies that --reveal and --no-redact behave
// as a union: setting either flips redaction off, setting both is not
// treated as a conflict, and the default (neither set) still redacts.
// The "default" case is the baseline that makes the disclosure-case
// assertions meaningful — without it, a regression that globally
// breaks redaction would silently make every union case pass.
func TestRedactFlags_Union(t *testing.T) {
	t.Parallel()

	const (
		loc      = "postgres://sakila:p_ssW0rd@localhost/sakila"
		password = "p_ssW0rd"
	)

	cases := []struct {
		name    string
		args    []string
		expects bool // whether the password should appear in output
	}{
		{name: "default", args: []string{"ls", "-v"}, expects: false},
		{name: "reveal", args: []string{"ls", "-v", "--reveal"}, expects: true},
		{name: "no-redact", args: []string{"ls", "-v", "--no-redact"}, expects: true},
		{name: "both", args: []string{"ls", "-v", "--reveal", "--no-redact"}, expects: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr := testrun.New(t.Context(), t, nil)
			require.NoError(t, tr.Exec("add", loc, "--skip-verify"))
			require.NoError(t, tr.Reset().Exec(tc.args...))
			if tc.expects {
				require.Contains(t, tr.OutString(), password,
					"%s must flip redaction off", tc.name)
			} else {
				require.NotContains(t, tr.OutString(), password,
					"%s must redact by default", tc.name)
			}
		})
	}
}

// TestExpandCentral_GroupCmd verifies that --expand is honored through
// the central writer-layer decorator (see expand_writer.go) by a
// command that has no expansion code of its own: sq group (gh780).
// Before centralization, only commands that explicitly called
// maybeExpandSource / maybeExpandCollection honored --expand; sq group
// accepted the flag and silently ignored it.
func TestExpandCentral_GroupCmd(t *testing.T) {
	// Not parallel: uses t.Setenv.
	const (
		envVar  = "SQ_TEST_GROUP_EXPAND_PW"
		envPass = "grouphunter"
	)
	t.Setenv(envVar, envPass)
	loc := "postgres://alice:${env:" + envVar + "}@db/sakila"

	testCases := []struct {
		name    string
		args    []string
		want    string
		wantNot string
	}{
		{
			name: "expand_reveal_shows_resolved",
			args: []string{"group", "--json", "--expand", "--reveal"},
			want: envPass,
		},
		{
			name:    "reveal_only_shows_placeholder",
			args:    []string{"group", "--json", "--reveal"},
			want:    "${env:" + envVar + "}",
			wantNot: envPass,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tr := testrun.New(t.Context(), t, nil)
			require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
				Handle:   "@group_expand",
				Type:     drivertype.Pg,
				Location: loc,
			}))

			require.NoError(t, tr.Exec(tc.args...))
			got := tr.OutString()
			require.Contains(t, got, tc.want)
			if tc.wantNot != "" {
				require.NotContains(t, got, tc.wantNot)
			}
		})
	}
}
