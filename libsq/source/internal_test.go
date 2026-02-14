package source

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh/tu"
)

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
