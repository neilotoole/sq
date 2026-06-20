package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output/format"
	"github.com/neilotoole/sq/libsq/core/options"
)

// TestGetOutputConfig_ForcedColorNonFileStdout verifies that getOutputConfig
// does not panic when FORCE_COLOR forces color on but stdout is not an *os.File
// (e.g. a bytes.Buffer in tests, or a pipe). Before the fix, this path type-
// asserted stdout to *os.File for colorable.NewColorable.
func TestGetOutputConfig_ForcedColorNonFileStdout(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "1")
	t.Setenv("TERM", "")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	outCfg := getOutputConfig(nil, nil, nil, format.JSON, options.Options{}, stdout, stderr)
	require.NotNil(t, outCfg)
	require.Same(t, stdout, outCfg.out)
	require.False(t, outCfg.outPr.IsMonochrome())
}
