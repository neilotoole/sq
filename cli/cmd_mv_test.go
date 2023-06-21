package cli_test

import (
	"testing"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/testh/tutil"
	"github.com/stretchr/testify/require"
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
		tc := tc
		t.Run(tutil.Name(i, tc.in), func(t *testing.T) {
			got := cli.LastHandlePart(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}
