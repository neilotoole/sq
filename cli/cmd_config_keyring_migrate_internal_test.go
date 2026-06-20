package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestConfigKeyringMigrate_RequiresConfigLock guards that migrate is marked to
// take the config lock, since it mutates sq.yml. Dropping the marker would
// silently make migrate non-atomic against a concurrent sq process.
func TestConfigKeyringMigrate_RequiresConfigLock(t *testing.T) {
	require.True(t, cmdRequiresConfigLock(newConfigKeyringMigrateCmd()))
}

// TestPromptYesNo covers the strict y/n prompt: only y/yes and n/no (plus the
// empty-line [y/N] default) are accepted; any other input is an error, as is
// EOF with no answer. The prompt does not retry.
func TestPromptYesNo(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    bool
		wantErr bool
	}{
		{name: "y", input: "y\n", want: true},
		{name: "yes", input: "yes\n", want: true},
		{name: "uppercase Y", input: "Y\n", want: true},
		{name: "uppercase YES", input: "YES\n", want: true},
		{name: "whitespace trimmed", input: "  yes  \n", want: true},
		{name: "y no newline (data+EOF)", input: "y", want: true},
		{name: "n", input: "n\n", want: false},
		{name: "no", input: "no\n", want: false},
		{name: "empty line is the [y/N] default no", input: "\n", want: false},
		{name: "unrecognized word errors", input: "what?\n", wantErr: true},
		{name: "partial 'ye' errors", input: "ye\n", wantErr: true},
		{name: "yep errors", input: "yep\n", wantErr: true},
		{name: "numeric errors", input: "1\n", wantErr: true},
		{name: "empty input (EOF) errors", input: "", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			got, err := promptYesNo(strings.NewReader(tc.input), &out, "Proceed?")
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestPromptYesNo_DoesNotRetry verifies the prompt is issued exactly once: an
// unrecognized answer errors immediately rather than re-prompting.
func TestPromptYesNo_DoesNotRetry(t *testing.T) {
	var out bytes.Buffer
	// A valid "y" follows the garbage, but it must never be read.
	_, err := promptYesNo(strings.NewReader("maybe\ny\n"), &out, "Proceed?")
	require.Error(t, err)
	require.Equal(t, 1, strings.Count(out.String(), "Proceed?"), "prompt must be issued once")
}
