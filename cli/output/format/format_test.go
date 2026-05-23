package format_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output/format"
)

// TestMermaidERD_validButUnenumerated pins the deliberate asymmetry:
// "mermaid-erd" is a valid (parseable) format, but it's intentionally absent
// from All() because it's an inspect-only metadata format with no record
// writer. Adding it to All() would advertise it for query commands (shell
// completion, format parity), where it can't produce records — so this guards
// against an accidental "fix" that adds it back.
func TestMermaidERD_validButUnenumerated(t *testing.T) {
	var f format.Format
	require.NoError(t, f.UnmarshalText([]byte("mermaid-erd")))
	require.Equal(t, format.MermaidERD, f)

	require.False(t, slices.Contains(format.All(), format.MermaidERD),
		"MermaidERD must stay out of All(): it's inspect-only with no record writer")
}
