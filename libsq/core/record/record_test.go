package record_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/timez"
	"github.com/neilotoole/sq/libsq/core/vector"
	"github.com/neilotoole/sq/testh/tu"
)

func TestEqual(t *testing.T) {
	mar1UTC, _ := timez.ParseDateUTC("2023-03-01")
	mar31UTC, _ := timez.ParseDateUTC("2023-03-31")

	stdRec1 := record.Record{
		nil,
		int64(1),
		1.1,
		false,
		"a",
		[]byte("a"),
		mar1UTC,
		decimal.New(7777, -2),
	}

	stdRec2 := record.Record{
		nil,
		int64(2),
		2.2,
		true,
		"b",
		[]byte("b"),
		mar31UTC,
		decimal.New(8888, -2),
	}

	testCases := []struct {
		a    record.Record
		b    record.Record
		want bool
	}{
		{nil, nil, true},
		{stdRec1, nil, false},
		{stdRec1, record.Record{}, false},
		{stdRec1[0:3], stdRec1[0:3], true},
		{stdRec1[0:3], stdRec1[0:4], false},
		{stdRec1, stdRec1, true},
		{stdRec1, stdRec2, false},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.a, tc.b), func(t *testing.T) {
			_, err := record.Valid(tc.a)
			require.NoError(t, err)
			_, err = record.Valid(tc.b)
			require.NoError(t, err)

			got := record.Equal(tc.a, tc.b)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestVector(t *testing.T) {
	v := vector.Vector{Elem: vector.F32, Values: []float64{1, 2}}
	rec := record.Record{v}
	_, err := record.Valid(rec)
	require.NoError(t, err)

	clone := record.Clone(rec)
	require.True(t, record.Equal(rec, clone))
	clone[0] = vector.Vector{Elem: vector.F32, Values: []float64{1, 3}}
	require.False(t, record.Equal(rec, clone))
	require.False(t, record.Equal(rec, record.Record{[]byte{1}}))
}
