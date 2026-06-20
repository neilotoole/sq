package yamlw_test

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/kind"
)

// TestRecordWriter_Bytes verifies that the YAML record writer encodes a []byte
// value as a base64 string (matching the JSON output) rather than goccy's
// default YAML sequence of byte ints. See #851.
func TestRecordWriter_Bytes(t *testing.T) {
	hi := []byte("hi")
	wantHi := base64.StdEncoding.EncodeToString(hi) // "aGk="

	// These bytes base64-encode to the all-digit group "1234", which must stay
	// quoted in YAML so it round-trips as a string rather than the int 1234.
	numericB64 := []byte{0xd7, 0x6d, 0xf8}
	wantNumeric := base64.StdEncoding.EncodeToString(numericB64) // "1234"

	t.Run("bytes_kind", func(t *testing.T) {
		pr := output.NewPrinting()
		pr.EnableColor(true)
		got := renderColorYAML(t, pr, kind.Bytes, hi)
		require.Contains(t, got, pr.Bytes.Sprint(wantHi))
	})

	t.Run("unknown_kind", func(t *testing.T) {
		pr := output.NewPrinting()
		pr.EnableColor(true)
		got := renderColorYAML(t, pr, kind.Unknown, hi)
		require.Contains(t, got, pr.Bytes.Sprint(wantHi))
	})

	t.Run("numeric_base64_is_quoted", func(t *testing.T) {
		pr := output.NewPrinting()
		pr.EnableColor(false)
		got := renderColorYAML(t, pr, kind.Bytes, numericB64)
		require.Equal(t, "- c: \""+wantNumeric+"\"\n", got)
	})

	t.Run("monochrome", func(t *testing.T) {
		pr := output.NewPrinting()
		pr.EnableColor(false)
		got := renderColorYAML(t, pr, kind.Bytes, hi)
		require.Equal(t, "- c: "+wantHi+"\n", got)
		require.NotContains(t, got, "\x1b")
	})

	// A typed-nil []byte is a NULL value and must render as null, matching the
	// JSON encoder, rather than as the base64 of an empty slice (""). It does
	// not satisfy the writer's val == nil check, so it reaches the []byte path.
	t.Run("nil_renders_null", func(t *testing.T) {
		pr := output.NewPrinting()
		pr.EnableColor(false)
		got := renderColorYAML(t, pr, kind.Bytes, []byte(nil))
		require.Equal(t, "- c: null\n", got)
	})

	// An empty but non-nil []byte is distinct from NULL: it round-trips as an
	// empty string, matching the JSON encoder.
	t.Run("empty_renders_quoted_empty", func(t *testing.T) {
		pr := output.NewPrinting()
		pr.EnableColor(false)
		got := renderColorYAML(t, pr, kind.Bytes, []byte{})
		require.Equal(t, "- c: \"\"\n", got)
	})

	// A typed-nil []byte in a kind.Unknown column takes the byValue path but
	// must still short-circuit to null before color resolution.
	t.Run("unknown_nil_renders_null", func(t *testing.T) {
		pr := output.NewPrinting()
		pr.EnableColor(false)
		got := renderColorYAML(t, pr, kind.Unknown, []byte(nil))
		require.Equal(t, "- c: null\n", got)
	})
}
