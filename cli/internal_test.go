package cli

import (
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/testh/tutil"

	"github.com/stretchr/testify/require"
)

func Test_preprocessFlagArgVars(t *testing.T) {
	testCases := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{
			name: "empty",
			in:   []string{},
			want: []string{},
		},
		{
			name: "no flags",
			in:   []string{".actor"},
			want: []string{".actor"},
		},
		{
			name: "non-arg flag",
			in:   []string{"--json", ".actor"},
			want: []string{"--json", ".actor"},
		},
		{
			name: "non-arg flag with value",
			in:   []string{"--json", "true", ".actor"},
			want: []string{"--json", "true", ".actor"},
		},
		{
			name: "single arg flag",
			in:   []string{"--arg", "name", "TOM", ".actor"},
			want: []string{"--arg", "name:TOM", ".actor"},
		},
		{
			name:    "invalid arg name",
			in:      []string{"--arg", "na me", "TOM", ".actor"},
			wantErr: true,
		},
		{
			name:    "invalid arg name (with colon)",
			in:      []string{"--arg", "na:me", "TOM", ".actor"},
			wantErr: true,
		},
		{
			name: "colon in value",
			in:   []string{"--arg", "name", "T:OM", ".actor"},
			want: []string{"--arg", "name:T:OM", ".actor"},
		},
		{
			name: "single arg flag with whitespace",
			in:   []string{"--arg", "name", "TOM DOWD", ".actor"},
			want: []string{"--arg", "name:TOM DOWD", ".actor"},
		},
		{
			name: "two arg flags",
			in:   []string{"--arg", "name", "TOM", "--arg", "eyes", "blue", ".actor"},
			want: []string{"--arg", "name:TOM", "--arg", "eyes:blue", ".actor"},
		},
		{
			name: "two arg flags with interspersed flag",
			in:   []string{"--arg", "name", "TOM", "--json", "true", "--arg", "eyes", "blue", ".actor"},
			want: []string{"--arg", "name:TOM", "--json", "true", "--arg", "eyes:blue", ".actor"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, gotErr := preprocessFlagArgVars(tc.in)
			if tc.wantErr {
				t.Log(gotErr.Error())
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.EqualValues(t, tc.want, got)
		})
	}
}

func Test_lastHandlePart(t *testing.T) {
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
			got := lastHandlePart(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestRegisterDefaultOpts(t *testing.T) {
	log := slogt.New(t)
	reg := &options.Registry{}

	log.Debug("options.Registry (before)", "reg", reg)
	RegisterDefaultOpts(reg)

	log.Debug("options.Registry (after)", "reg", reg)

	keys := reg.Keys()
	require.Len(t, keys, 19)
}
