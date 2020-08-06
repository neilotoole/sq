package postgres

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/driver"
)

var GetTableColumnNames = getTableColumnNames

func TestPlaceholders(t *testing.T) {
	testCases := map[int]string{
		0: "",
		1: "$1",
		2: "$1" + driver.Comma + "$2",
		3: "$1" + driver.Comma + "$2" + driver.Comma + "$3",
	}

	for n, want := range testCases {
		got := placeholders(n)
		require.Equal(t, want, got)
	}
}

func TestReplacePlaceholders(t *testing.T) {
	testCases := map[string]string{
		"":                "",
		"hello":           "hello",
		"?":               "$1",
		"??":              "$1$2",
		" ? ":             " $1 ",
		"(?, ?)":          "($1, $2)",
		"(?, ?, ?)":       "($1, $2, $3)",
		" (?  , ? , ?)  ": " ($1  , $2 , $3)  ",
	}

	for input, want := range testCases {
		got := replacePlaceholders(input)
		require.Equal(t, want, got)
	}
}
