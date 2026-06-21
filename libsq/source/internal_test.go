package source

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh/tu"
)

// TestValidSource covers validSource directly, including the branches
// (nil source, invalid handle) that are shadowed by earlier guards when
// reached via the public VerifyIntegrity path.
func TestValidSource(t *testing.T) {
	require.Error(t, validSource(nil))

	// Invalid handle.
	require.Error(t, validSource(&Source{
		Handle:   "bad-handle",
		Type:     drivertype.SQLite,
		Location: "/tmp/x.db",
	}))

	// Empty location.
	require.Error(t, validSource(&Source{
		Handle: "@x",
		Type:   drivertype.SQLite,
	}))

	// Unknown driver type.
	require.Error(t, validSource(&Source{
		Handle:   "@x",
		Location: "/tmp/x.db",
	}))

	// Valid.
	require.NoError(t, validSource(&Source{
		Handle:   "@x",
		Type:     drivertype.SQLite,
		Location: "/tmp/x.db",
	}))
}

// TestSuggestNameForScheme_edges covers the per-scheme edge branches of
// suggestNameForScheme that the public placeholder tests don't reach.
func TestSuggestNameForScheme_edges(t *testing.T) {
	testCases := []struct {
		scheme string
		body   string
		want   string
		ok     bool
	}{
		{scheme: "env", body: "", want: "", ok: false},
		{scheme: "file", body: ".", want: "", ok: false},
		{scheme: "file", body: "/", want: "", ok: false},
		{scheme: "file", body: "", want: "", ok: false},
		{scheme: "file", body: "/path/.dsn", want: "", ok: false},
		{scheme: "op", body: "//vault", want: "", ok: false},
		{scheme: "op", body: "//vault/", want: "", ok: false},
		{scheme: "vault", body: "secret/", want: "", ok: false},
		{scheme: "unknown", body: "whatever", want: "", ok: false},
	}

	for _, tc := range testCases {
		t.Run(tc.scheme+"_"+tc.body, func(t *testing.T) {
			got, ok := suggestNameForScheme(tc.scheme, tc.body)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestSuggestNameFromPlaceholder_composition covers the non-bare
// composition branch (placeholder embedded in surrounding text).
func TestSuggestNameFromPlaceholder_composition(t *testing.T) {
	// Surrounding literal text means it's not a bare placeholder.
	_, ok := suggestNameFromPlaceholder("prefix${env:FOO}suffix")
	require.False(t, ok)

	// Multiple refs -> not handled.
	_, ok = suggestNameFromPlaceholder("${env:A}${env:B}")
	require.False(t, ok)

	// No refs -> not handled.
	_, ok = suggestNameFromPlaceholder("plain-location")
	require.False(t, ok)
}

func TestGroupsFilterOnlyDirectChildren(t *testing.T) {
	testCases := []struct {
		parent string
		groups []string
		want   []string
	}{
		{
			parent: "/",
			groups: []string{"/", "prod", "prod/customer", "staging"},
			want:   []string{"prod", "staging"},
		},
		{
			parent: "prod",
			groups: []string{"/", "prod", "prod/customer", "prod/backup", "staging"},
			want:   []string{"prod/customer", "prod/backup"},
		},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.want), func(t *testing.T) {
			got := groupsFilterOnlyDirectChildren(tc.parent, tc.groups)
			require.EqualValues(t, tc.want, got)
		})
	}
}
