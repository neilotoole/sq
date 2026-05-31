package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/tu"
)

func TestLastHandlePart(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{"@handle", "handle"},
		{"@prod/db", "db"},
		{"@prod/sub/db", "db"},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			got := cli.LastHandlePart(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestCmdMv_InvalidArgDiagnostics verifies that sq mv surfaces a
// specific reason when an argument is malformed, rather than the
// generic "invalid args" message. The hyphen case here mirrors the
// real user report ("sq mv @sakila/sakila @sakila/local/pg-keyring1").
func TestCmdMv_InvalidArgDiagnostics(t *testing.T) {
	tests := []struct {
		name string
		// args to sq mv (OLD, NEW).
		oldArg, newArg string
		// substrings expected on the error message.
		wants []string
	}{
		{
			name:   "hyphen in NEW handle",
			oldArg: "@sakila", newArg: "@sakila/local/pg-keyring1",
			wants: []string{"second arg", `illegal character "-"`, "letters, digits, and underscore"},
		},
		{
			name:   "missing leading @ on NEW (treated as group, hyphen flagged)",
			oldArg: "@sakila", newArg: "prod-x",
			wants: []string{"second arg", `illegal character "-"`},
		},
		{
			name:   "leading digit in OLD handle",
			oldArg: "@1bad", newArg: "@sakila",
			wants: []string{"first arg", "must start with a letter"},
		},
		{
			name:   "OLD has trailing slash",
			oldArg: "@sakila/", newArg: "@sakila/x",
			wants: []string{"first arg", "empty segment"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			th := testh.New(t)
			tr := testrun.New(th.Context, t, nil)
			// Seed a source so the mv command has something to operate on.
			require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
				Handle:   "@sakila",
				Type:     drivertype.Pg,
				Location: "postgres://alice:pw@db/sakila",
			}))

			err := tr.Exec("mv", tc.oldArg, tc.newArg)
			require.Error(t, err)
			for _, want := range tc.wants {
				require.Contains(t, err.Error(), want,
					"diagnostic for %q -> %q should contain %q",
					tc.oldArg, tc.newArg, want)
			}
		})
	}
}
