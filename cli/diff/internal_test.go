package diff

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
