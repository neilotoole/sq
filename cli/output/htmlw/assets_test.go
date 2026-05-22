package htmlw

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMermaidJS(t *testing.T) {
	js, err := mermaidJS()
	require.NoError(t, err)
	require.Greater(t, len(js), 500_000, "decompressed mermaid bundle should be sizable")
	require.Contains(t, string(js), "mermaid")
}
