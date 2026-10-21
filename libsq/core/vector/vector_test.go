package vector_test

import (
	"encoding/binary"
	"encoding/hex"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/vector"
)

func mustHex(tb testing.TB, s string) []byte {
	tb.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(tb, err)
	return b
}

func f64Blob() []byte {
	var b []byte
	for _, f := range []float64{1, 2, -3.5, 0.25} {
		b = binary.LittleEndian.AppendUint64(b, math.Float64bits(f))
	}
	return append(b, 2)
}

func ty(dim int, e vector.Elem) vector.Type {
	return vector.Type{Dim: dim, Elem: e}
}

func js(b []byte, t vector.Type) (vector.Vector, error) {
	return vector.ParseJSON(string(b), t)
}

func TestDecoders(t *testing.T) {
	v0, ls := vector.DecodeVec0, vector.DecodeLibSQL
	bits12 := []float64{1, 0, 1, 1, 0, 0, 0, 0, 1, 1, 1, 0}

	want4 := []float64{1, 2, -3.5, 0.25}
	f32hex := "0000803F00000040000060C00000803E"

	testCases := []struct {
		name    string
		decode  func([]byte, vector.Type) (vector.Vector, error)
		in      []byte
		typ     vector.Type
		want    []float64
		delta   float64
		wantErr bool
	}{
		{"vec0_f32", v0, mustHex(t, f32hex), ty(4, vector.F32), want4, 0, false},
		{"vec0_i8", v0, mustHex(t, "01020304"), ty(4, vector.I8), []float64{1, 2, 3, 4}, 0, false},
		{"vec0_i8_neg", v0, mustHex(t, "FF80"), ty(2, vector.I8), []float64{-1, -128}, 0, false},
		{"vec0_bit", v0, mustHex(t, "F0"), ty(8, vector.Bit), []float64{0, 0, 0, 0, 1, 1, 1, 1}, 0, false},
		{"vec0_f32_len", v0, mustHex(t, "0000803F"), ty(4, vector.F32), nil, 0, true},
		{"libsql_f32", ls, mustHex(t, f32hex), ty(4, vector.F32), want4, 0, false},
		{"libsql_f64", ls, f64Blob(), ty(4, vector.F64), want4, 0, false},
		{"libsql_f16", ls, mustHex(t, "003C004000C3003405"), ty(4, vector.F16), want4, 0, false},
		{"libsql_bf16", ls, mustHex(t, "803F004060C0803E06"), ty(4, vector.BF16), want4, 0, false},
		{"libsql_f8", ls, mustHex(t, "D1FF00AEB1B0B03C000060C0000004"), ty(4, vector.F8), want4, 0.02, false},
		{"libsql_bit", ls, mustHex(t, "0D07001403"), ty(12, vector.Bit), bits12, 0, false},
		{"libsql_bit_short_data", ls, mustHex(t, "010003"), ty(16, vector.Bit), nil, 0, true},
		{"libsql_bad_tag", ls, mustHex(t, "0000803F0000004009"), ty(4, vector.F32), nil, 0, true},
		{"libsql_dim_mismatch", ls, mustHex(t, f32hex), ty(3, vector.F32), nil, 0, true},
		{"libsql_f16_dim_mismatch", ls, mustHex(t, "003C004000C3003405"), ty(5, vector.F16), nil, 0, true},
		{"json", js, []byte("[0.5,0.5,0.5,0.5]"), ty(4, vector.F32), []float64{0.5, 0.5, 0.5, 0.5}, 0, false},
		{"json_dim", js, []byte("[1,2]"), ty(4, vector.F32), nil, 0, true},
		{"json_bad", js, []byte("nope"), ty(4, vector.F32), nil, 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.decode(tc.in, tc.typ)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, len(tc.want), got.Dim())
			for i := range tc.want {
				require.InDelta(t, tc.want[i], got.Values[i], tc.delta)
			}
			require.Equal(t, tc.in, got.Raw)
		})
	}
}

func TestF16Special(t *testing.T) {
	// 0x0001 subnormal, 0x7C00 +Inf, 0x7E00 NaN, 0xFC00 -Inf, 0x8000 -0.
	v, err := vector.DecodeLibSQL(mustHex(t, "0100007C007E00FC05"), ty(4, vector.F16))
	require.NoError(t, err)
	require.InDelta(t, math.Ldexp(1, -24), v.Values[0], 0)
	require.True(t, math.IsInf(v.Values[1], 1))
	require.True(t, math.IsNaN(v.Values[2]))
	require.True(t, math.IsInf(v.Values[3], -1))
}

func TestFormat(t *testing.T) {
	v := vector.Vector{Elem: vector.F32, Values: []float64{float64(float32(0.9)), float64(float32(-0.12345679))}}
	require.Equal(t, "[0.9,-0.12345679]", v.String())
	require.Equal(t, "[2 × f32]", v.Summary())
	require.Equal(t, "[0.9,-0.12345679]", string(v.AppendJSON(nil)))
	require.Equal(t, "x[0.9,-0.12345679]", string(v.AppendJSON([]byte("x"))))

	i8 := vector.Vector{Elem: vector.I8, Values: []float64{1, 2}}
	require.Equal(t, "[1,2]", i8.String())

	f64 := vector.Vector{Elem: vector.F64, Values: []float64{0.1}}
	require.Equal(t, "[0.1]", f64.String())

	require.Equal(t, "[0 × bit]", vector.Vector{Elem: vector.Bit}.Summary())
	require.Equal(t, "[]", vector.Vector{Elem: vector.Bit}.String())
}

func TestFormat_NonFinite(t *testing.T) {
	v := vector.Vector{Elem: vector.F32, Values: []float64{1, math.NaN(), math.Inf(1), math.Inf(-1)}}
	require.Equal(t, "[1,null,null,null]", v.String())
}

func TestEqualClone(t *testing.T) {
	a := vector.Vector{Elem: vector.F32, Values: []float64{1, 2}, Raw: []byte{1, 2}}
	b := a.Clone()
	require.True(t, a.Equal(b))
	b.Values[0] = 9
	b.Raw[0] = 9
	require.Equal(t, 1.0, a.Values[0])
	require.Equal(t, byte(1), a.Raw[0])
	require.False(t, a.Equal(b))
	require.False(t, a.Equal(vector.Vector{Elem: vector.F64, Values: []float64{1, 2}, Raw: []byte{1, 2}}))
}
