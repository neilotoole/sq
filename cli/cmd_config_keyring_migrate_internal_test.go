package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPromptYesNo covers the y/n prompt: recognized answers, the [y/N]
// empty-line default, re-prompting on unrecognized input, and a non-nil
// error when the input stream ends without a valid answer.
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
		{name: "y no newline (data+EOF)", input: "y", want: true},
		{name: "n", input: "n\n", want: false},
		{name: "no", input: "no\n", want: false},
		{name: "empty line is the [y/N] default no", input: "\n", want: false},
		{name: "reprompt then yes", input: "what?\ny\n", want: true},
		{name: "reprompt then no", input: "huh\nn\n", want: false},
		{name: "invalid then EOF errors", input: "garbage\n", wantErr: true},
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

// TestPromptYesNo_RepromptsOnInvalid verifies the prompt is re-issued for
// each unrecognized response before a valid one is accepted.
func TestPromptYesNo_RepromptsOnInvalid(t *testing.T) {
	var out bytes.Buffer
	got, err := promptYesNo(strings.NewReader("maybe\nsure\ny\n"), &out, "Proceed?")
	require.NoError(t, err)
	require.True(t, got)
	// Three reads (maybe, sure, y) means the prompt was printed three times.
	require.Equal(t, 3, strings.Count(out.String(), "Proceed?"))
}
