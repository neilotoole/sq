package templatez_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/templatez"
	"github.com/neilotoole/sq/testh/tu"
)

func TestTemplate(t *testing.T) {
	data := map[string]string{"Name": "wubble"}

	testCases := []struct {
		tpl     string
		data    any
		want    string
		wantErr bool
	}{
		// "upper" is a sprig func. Verify that it loads.
		{"{{.Name | upper}}", data, "WUBBLE", false},
		{"{{not_a_func .Name}}_", data, "", true},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.tpl), func(t *testing.T) {
			got, gotErr := templatez.ExecuteTemplate(t.Name(), tc.tpl, tc.data)
			t.Logf("\nTPL:   %s\nGOT:   %s\nERR:   %v", tc.tpl, got, gotErr)
			if tc.wantErr {
				require.Error(t, gotErr)
				// Also test ValidTemplate while we're at it.
				gotErr = templatez.ValidTemplate(t.Name(), tc.tpl)
				require.Error(t, gotErr)
				return
			}
			require.NoError(t, gotErr)
			gotErr = templatez.ValidTemplate(t.Name(), tc.tpl)
			require.NoError(t, gotErr)

			require.Equal(t, tc.want, got)
		})
	}
}
