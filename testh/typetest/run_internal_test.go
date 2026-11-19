package typetest

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestDiffValue(t *testing.T) {
	m := NotNullOf[int64]()
	testCases := []struct {
		name    string
		want    any
		got     any
		wantErr string // empty: no error
	}{
		{name: "matcher_pass", want: m, got: int64(1)},
		{name: "matcher_null", want: m, got: nil, wantErr: "want non-NULL int64"},
		{name: "matcher_type", want: m, got: float64(1), wantErr: "got float64"},
		{name: "equal", want: int64(7), got: int64(7)},
		{name: "equal_bytes", want: []byte("se"), got: []byte("se")},
		{name: "equal_nil", want: nil, got: nil},
		{name: "type_mismatch", want: int64(7), got: float64(7), wantErr: "want int64(7), got float64(7)"},
		{name: "value_mismatch", want: "seven", got: "eight", wantErr: `want "seven", got "eight"`},
		{
			name:    "stringer",
			want:    decimal.RequireFromString("999.99"),
			got:     decimal.RequireFromString("1"),
			wantErr: "want decimal.Decimal(999.99), got decimal.Decimal(1)",
		},
		{name: "null_mismatch", want: int64(7), got: nil, wantErr: "want int64(7), got <nil>"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := diffValue(tc.want, tc.got)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}
