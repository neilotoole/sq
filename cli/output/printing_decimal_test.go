package output_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
)

func TestPrinting_DecimalAsNumber(t *testing.T) {
	pr := output.NewPrinting()
	// Default is string mode (quoted), i.e. not as-number.
	require.False(t, pr.DecimalAsNumber)

	pr.DecimalAsNumber = true
	require.True(t, pr.Clone().DecimalAsNumber, "Clone must preserve DecimalAsNumber")
}
