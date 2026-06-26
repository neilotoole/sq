package diff

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source/metadata"
)

func Test_renderTableMeta2YAML(t *testing.T) {
	t.Run("with_check_trigger_viewdef", func(t *testing.T) {
		tm := &metadata.Table{
			Name:        "my_view",
			TableType:   "view",
			ViewDefinition: "SELECT id, name FROM actor WHERE active = 1",
			CheckConstraints: []*metadata.CheckConstraint{
				{Name: "chk_name_len", Table: "my_view", Clause: "length(name) > 0"},
			},
			Triggers: []*metadata.Trigger{
				{Name: "trg_after_insert", Table: "my_view", Timing: "AFTER"},
			},
		}

		got, err := renderTableMeta2YAML(false, tm)
		require.NoError(t, err)

		// The three new fields must appear in the output.
		require.Contains(t, got, "length(name) > 0", "check constraint clause must appear")
		require.Contains(t, got, "trg_after_insert", "trigger name must appear")
		require.Contains(t, got, "AFTER", "trigger timing must appear")
		require.Contains(t, got, "SELECT id, name FROM actor", "view definition must appear")
	})

	t.Run("without_check_trigger_viewdef", func(t *testing.T) {
		tm := &metadata.Table{
			Name:      "actor",
			TableType: "table",
			Columns: []*metadata.Column{
				{Name: "actor_id"},
			},
		}

		got, err := renderTableMeta2YAML(false, tm)
		require.NoError(t, err)

		// The three new keys must NOT appear when the fields are empty (omitempty).
		require.NotContains(t, got, "check_constraints", "check_constraints key must be absent")
		require.NotContains(t, got, "triggers", "triggers key must be absent")
		require.NotContains(t, got, "view_definition", "view_definition key must be absent")
	})
}

func Test_adjustHunkOffset(t *testing.T) {
	testCases := []struct {
		in      string
		offset  int
		want    string
		wantErr bool
	}{
		{in: "@@ -44,7 +44,7 @@", offset: 10, want: "@@ -54,7 +54,7 @@", wantErr: false},
		{in: "@@ -1 +0,7 @@", offset: 10, want: "@@ -11 +10,7 @@", wantErr: false},
		{in: "@@ -1,2 +1 @@", offset: 10, want: "@@ -11,2 +11 @@", wantErr: false},
		{in: "@@ -44 +44 @@", offset: 10, want: "@@ -54 +54 @@", wantErr: false},
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := adjustHunkOffset(tc.in, tc.offset)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			}
		})
	}
}
