package urlz_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/urlz"
	"github.com/neilotoole/sq/testh/tutil"
)

func TestQueryParamKeys(t *testing.T) {
	testCases := []struct {
		q       string
		want    []string
		wantErr bool
	}{
		{"a=1", []string{"a"}, false},
		{"a=1&b=2", []string{"a", "b"}, false},
		{"b=1&a=2", []string{"b", "a"}, false},
		{"a=1&b=", []string{"a", "b"}, false},
		{"a=1&b=;", []string{"a"}, true},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, tc.q), func(t *testing.T) {
			got, gotErr := urlz.QueryParamKeys(tc.q)
			if tc.wantErr {
				require.Error(t, gotErr)
			} else {
				require.NoError(t, gotErr)
			}
			require.Equal(t, tc.want, got)
		})
	}
}

func TestStripQuery(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{"https://sq.io", "https://sq.io"},
		{"https://sq.io/path", "https://sq.io/path"},
		{"https://sq.io/path#frag", "https://sq.io/path#frag"},
		{"https://sq.io?a=b", "https://sq.io"},
		{"https://sq.io/path?a=b", "https://sq.io/path"},
		{"https://sq.io/path?a=b&c=d#frag", "https://sq.io/path#frag"},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, tc), func(t *testing.T) {
			u, err := url.Parse(tc.in)
			require.NoError(t, err)
			got := urlz.StripQuery(*u)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestStripUser(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{"https://sq.io", "https://sq.io"},
		{"https://alice:123@sq.io/path", "https://sq.io/path"},
		{"https://alice@sq.io/path", "https://sq.io/path"},
		{"https://alice:@sq.io/path", "https://sq.io/path"},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, tc), func(t *testing.T) {
			u, err := url.Parse(tc.in)
			require.NoError(t, err)
			got := urlz.StripUser(*u)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestStripScheme(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{"https://sq.io", "sq.io"},
		{"https://alice:123@sq.io/path", "alice:123@sq.io/path"},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, tc), func(t *testing.T) {
			u, err := url.Parse(tc.in)
			require.NoError(t, err)
			got := urlz.StripScheme(*u)
			require.Equal(t, tc.want, got)
		})
	}
}

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
		tc := tc
		t.Run(tutil.Name(i, tc), func(t *testing.T) {
			u, err := url.Parse(tc.in)
			require.NoError(t, err)
			got := urlz.StripSchemeAndUser(*u)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestRenameQueryParamKey(t *testing.T) {
	testCases := []struct {
		q      string
		oldKey string
		newKey string
		want   string
	}{
		{"", "a", "b", ""},
		{"a=1", "a", "b", "b=1"},
		{"a", "a", "b", "b"},
		{"aa", "a", "b", "aa"},
		{"a=", "a", "b", "b="},
		{"a=1&a=2", "a", "b", "b=1&b=2"},
		{"a=1&c=2", "a", "b", "b=1&c=2"},
		{"a=1&c=2&a=3&a=4", "a", "b", "b=1&c=2&b=3&b=4"},
		{"a=a&c=2&a=b&a=c", "a", "b", "b=a&c=2&b=b&b=c"},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, tc.q, tc.oldKey, tc.newKey), func(t *testing.T) {
			got := urlz.RenameQueryParamKey(tc.q, tc.oldKey, tc.newKey)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestURLStripQuery(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{"https://sq.io", "https://sq.io"},
		{"https://sq.io/path", "https://sq.io/path"},
		{"https://sq.io/path#frag", "https://sq.io/path#frag"},
		{"https://sq.io?a=b", "https://sq.io"},
		{"https://sq.io/path?a=b", "https://sq.io/path"},
		{"https://sq.io/path?a=b&c=d#frag", "https://sq.io/path#frag"},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, tc), func(t *testing.T) {
			u, err := url.Parse(tc.in)
			require.NoError(t, err)
			got := urlz.StripQuery(*u)
			require.Equal(t, tc.want, got)
		})
	}
}
