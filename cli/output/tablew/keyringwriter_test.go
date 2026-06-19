package tablew_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/tablew"
)

// newTestPrinting returns a Printing config suitable for text-rendering
// unit tests: header on by default and color disabled (so assertions
// don't have to strip ANSI escapes).
func newTestPrinting() *output.Printing {
	pr := output.NewPrinting()
	pr.EnableColor(false)
	return pr
}

// TestKeyringWriter_Migrate_RowFormatting pins the text-mode output
// shape for each KeyringMigrateRow status. Without this test, a
// regression that reorders Error/Handle/NewLocation in any of the four
// case branches (or drops the status word) would silently break the
// user-facing migration log.
func TestKeyringWriter_Migrate_RowFormatting(t *testing.T) {
	tests := []struct {
		name     string
		row      output.KeyringMigrateRow
		contains []string
	}{
		{
			name: "planned",
			row: output.KeyringMigrateRow{
				Handle: "@plan_src",
				Status: output.KeyringMigrateStatusPlanned,
			},
			contains: []string{"@plan_src", "->", "${keyring:<new-id>}"},
		},
		{
			name: "skip",
			row: output.KeyringMigrateRow{
				Handle: "@skip_src",
				Status: output.KeyringMigrateStatusSkip,
				Reason: "no password component",
			},
			contains: []string{"@skip_src", "skip", "no password component"},
		},
		{
			name: "migrated",
			row: output.KeyringMigrateRow{
				Handle:      "@done_src",
				Status:      output.KeyringMigrateStatusMigrated,
				NewLocation: "${keyring:abc1234567}",
			},
			contains: []string{"@done_src", "done", "${keyring:abc1234567}"},
		},
		{
			name: "failed",
			row: output.KeyringMigrateRow{
				Handle: "@fail_src",
				Status: output.KeyringMigrateStatusFailed,
				Error:  "save config (rolled back): boom",
			},
			contains: []string{"@fail_src", "FAIL", "save config (rolled back): boom"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := tablew.NewKeyringWriter(&buf, newTestPrinting())
			require.NoError(t, w.Migrate([]output.KeyringMigrateRow{tc.row}, false))
			out := buf.String()
			for _, s := range tc.contains {
				require.Contains(t, out, s, "rendered output must contain %q; got: %s", s, out)
			}
		})
	}
}

// TestKeyringWriter_List_HeaderToggle verifies that List honors
// pr.ShowHeader: header row prints when true (with STATUS/PATH/HANDLE/DRIVER
// labels) and is omitted when false.
func TestKeyringWriter_List_HeaderToggle(t *testing.T) {
	refs := []output.KeyringRef{
		{Path: "abc123", Handle: "@h", Driver: "postgres"},
	}

	t.Run("header on", func(t *testing.T) {
		var buf bytes.Buffer
		pr := newTestPrinting()
		pr.ShowHeader = true
		w := tablew.NewKeyringWriter(&buf, pr)
		require.NoError(t, w.List(refs))
		out := buf.String()
		require.Contains(t, out, "STATUS")
		require.Contains(t, out, "PATH")
		require.Contains(t, out, "HANDLE")
		require.Contains(t, out, "DRIVER")
		require.Contains(t, out, "abc123")
	})

	t.Run("header off", func(t *testing.T) {
		var buf bytes.Buffer
		pr := newTestPrinting()
		pr.ShowHeader = false
		w := tablew.NewKeyringWriter(&buf, pr)
		require.NoError(t, w.List(refs))
		out := buf.String()
		require.NotContains(t, out, "STATUS")
		require.NotContains(t, out, "PATH")
		require.NotContains(t, out, "HANDLE")
		require.NotContains(t, out, "DRIVER")
		require.Contains(t, out, "abc123")
	})
}

// TestKeyringWriter_ListStatusColumn verifies that List renders the STATUS
// column with the correct values for referenced, orphan, and missing rows.
func TestKeyringWriter_ListStatusColumn(t *testing.T) {
	buf := &bytes.Buffer{}
	w := tablew.NewKeyringWriter(buf, newTestPrinting())

	err := w.List([]output.KeyringRef{
		{Status: output.KeyringStatusReferenced, Path: "j2k7m3pxtz", Handle: "@prod", Driver: "postgres"},
		{Status: output.KeyringStatusOrphan, Path: "m4n8k2pxtz"},
		{Status: output.KeyringStatusMissing, Path: "@stale/pw", Handle: "@stale", Driver: "mysql"},
	})
	require.NoError(t, err)

	out := buf.String()
	require.Contains(t, out, "STATUS")
	require.Contains(t, out, "referenced")
	require.Contains(t, out, "orphan")
	require.Contains(t, out, "missing")
	require.Contains(t, out, "m4n8k2pxtz")
}
