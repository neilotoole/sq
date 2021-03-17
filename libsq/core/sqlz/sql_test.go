package sqlz_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/testh/tutil"
)

func TestExtractOuterFuncNameFromSQLExpr(t *testing.T) {
	testCases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "ST_AsBinary(loc)", want: "ST_AsBinary"},
		{in: " AsBinary( loc ) ", want: "AsBinary"},
		{in: "ST_AsBinary()", want: "ST_AsBinary"},
		{in: "ST_AsBinary(   )", want: "ST_AsBinary"},
		{in: "ST_AsBinary", wantErr: true},
		{in: "ST_AsBinary(", wantErr: true},
		{in: "ST_AsBinary ( ", wantErr: true},
		{in: " ST_AsBinary ( ) ", want: "ST_AsBinary"},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tutil.Name(tc.in), func(t *testing.T) {
			got, gotErr := sqlz.ExtractOuterFuncNameFromSQLExpr(tc.in)
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestIsSQLFuncExpr(t *testing.T) {
	testCases := []struct {
		in   string
		want bool
	}{
		{in: "ST_AsBinary(loc)", want: true},
		{in: " AsBinary( loc ) ", want: true},
		{in: "ST_AsBinary()", want: true},
		{in: "ST_AsBinary", want: false},
		{in: "ST_AsBinary(", want: false},
		{in: "ST_AsBinary ( ", want: false},
		{in: " ST_AsBinary ( ) ", want: true},
		{in: " ST_ AsBinary ( ) ", want: false},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tutil.Name(tc.in), func(t *testing.T) {
			got := sqlz.IsSQLFuncExpr(tc.in)

			require.Equal(t, tc.want, got)
		})
	}
}
