package cli

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh/tu"
)

func TestStripSchemeAndUser(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{"https://sq.io", "sq.io"},
		{"https://alice:123@sq.io/path", "sq.io/path"},
		{"https://alice@sq.io/path", "sq.io/path"},
		{"https://alice:@sq.io/path", "sq.io/path"},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc), func(t *testing.T) {
			u, err := url.Parse(tc.in)
			require.NoError(t, err)
			got := stripSchemeAndUser(*u)
			require.Equal(t, tc.want, got)
		})
	}
}
