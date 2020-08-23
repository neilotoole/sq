package sqlmodel

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrimTrailingDelims(t *testing.T) {
	delims := []string{";", "go", "Go", "gO", "GO"}

	testCases := map[string]string{
		"":                                "",
		"\t":                              "",
		"   ":                             "",
		";":                               "",
		"GO":                              "",
		"go;":                             "",
		"select * from food":              "select * from food",
		"select * from food;":             "select * from food",
		"select * from food ;":            "select * from food",
		"select * from food ; ":           "select * from food",
		"select * from food;go":           "select * from food",
		"select * from food;GO":           "select * from food",
		"select * from food;Go":           "select * from food",
		"select * from food;gO":           "select * from food",
		"select * from food; go":          "select * from food",
		"select * from food ; go ;;;go  ": "select * from food",
		"select * from food2go":           "select * from food2go",
		"select * from food2go;go":        "select * from food2go",
		"select * from food2go go":        "select * from food2go",
		"select * from food2go go go go":  "select * from food2go",
	}

	for input, want := range testCases {
		got := trimTrailingDelims(input, delims...)
		require.Equal(t, want, got)
	}
}
