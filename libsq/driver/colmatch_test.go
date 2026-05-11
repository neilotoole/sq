package driver_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/driver"
)

func TestResolveTableColumnsFold(t *testing.T) {
	actual := []string{"actor_id", "first_name", "last_name", "MixedCase"}

	testCases := []struct {
		name    string
		src     []string
		want    []string
		wantErr string
	}{
		{
			name: "exact_lowercase",
			src:  []string{"actor_id", "first_name"},
			want: []string{"actor_id", "first_name"},
		},
		{
			name: "uppercase_from_oracle",
			src:  []string{"ACTOR_ID", "FIRST_NAME", "LAST_NAME"},
			want: []string{"actor_id", "first_name", "last_name"},
		},
		{
			name: "preserves_input_order",
			src:  []string{"LAST_NAME", "ACTOR_ID"},
			want: []string{"last_name", "actor_id"},
		},
		{
			name: "mixed_case_canonical_returned",
			src:  []string{"mixedcase"},
			want: []string{"MixedCase"},
		},
		{
			name: "empty_input_yields_empty",
			src:  []string{},
			want: []string{},
		},
		{
			name:    "missing_column_errors",
			src:     []string{"ACTOR_ID", "BOGUS_COL"},
			wantErr: `column "BOGUS_COL" does not exist`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := driver.ResolveTableColumnsFold(actual, tc.src)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
