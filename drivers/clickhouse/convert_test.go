package clickhouse_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/clickhouse"
)

// TestConvertArrayToString tests that all slice types returned by ClickHouse
// Array columns are correctly converted to comma-separated strings.
func TestConvertArrayToString(t *testing.T) {
	testCases := []struct {
		name  string
		input any
		want  any
	}{
		// String slices.
		{"[]string", []string{"a", "b", "c"}, "a,b,c"},
		{"[]string/single", []string{"hello"}, "hello"},
		{"[]string/empty", []string{}, ""},

		// Signed integer slices.
		{"[]int", []int{1, 2, 3}, "1,2,3"},
		{"[]int8", []int8{-1, 0, 127}, "-1,0,127"},
		{"[]int16", []int16{-32768, 0, 32767}, "-32768,0,32767"},
		{"[]int32", []int32{-100, 0, 100}, "-100,0,100"},
		{"[]int64", []int64{-9223372036854775808, 0, 9223372036854775807}, "-9223372036854775808,0,9223372036854775807"},

		// Unsigned integer slices.
		{"[]uint", []uint{0, 1, 42}, "0,1,42"},
		{"[]uint8", []uint8{0, 128, 255}, "0,128,255"},
		{"[]uint16", []uint16{0, 1000, 65535}, "0,1000,65535"},
		{"[]uint32", []uint32{0, 100000, 4294967295}, "0,100000,4294967295"},
		{"[]uint64", []uint64{0, 1, 18446744073709551615}, "0,1,18446744073709551615"},

		// Float slices.
		{"[]float32", []float32{1.5, 2.25, 3.0}, "1.5,2.25,3"},
		{"[]float64", []float64{1.5, 2.25, 3.0}, "1.5,2.25,3"},

		// Bool slices.
		{"[]bool", []bool{true, false, true}, "true,false,true"},

		// Non-slice values should be returned unchanged.
		{"string", "hello", "hello"},
		{"int", 42, 42},
		{"nil", nil, nil},
		{"float64", 3.14, 3.14},
		{"bool", true, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := clickhouse.ConvertArrayToString(tc.input)
			require.Equal(t, tc.want, got)
		})
	}
}
