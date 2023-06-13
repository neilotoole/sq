package cobraz

import (
	"testing"

	"github.com/neilotoole/sq/testh/tutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestExtractDirectives(t *testing.T) {
	testCases := []struct {
		in          cobra.ShellCompDirective
		want        []cobra.ShellCompDirective
		wantStrings []string
	}{
		{
			cobra.ShellCompDirectiveError,
			[]cobra.ShellCompDirective{cobra.ShellCompDirectiveError},
			[]string{ShellCompDirectiveErrorText},
		},
		{
			cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoSpace,
			[]cobra.ShellCompDirective{cobra.ShellCompDirectiveError, cobra.ShellCompDirectiveNoSpace},
			[]string{ShellCompDirectiveErrorText, ShellCompDirectiveNoSpaceText},
		},
		{
			cobra.ShellCompDirectiveDefault,
			[]cobra.ShellCompDirective{cobra.ShellCompDirectiveDefault},
			[]string{ShellCompDirectiveDefaultText},
		},
	}

	for i, tc := range testCases {
		t.Run(tutil.Name(i, tc.in), func(t *testing.T) {
			gotDirectives := ExtractDirectives(tc.in)
			require.Equal(t, tc.want, gotDirectives)
			gotStrings := MarshalDirective(tc.in)
			require.Equal(t, tc.wantStrings, gotStrings)
		})
	}
}
