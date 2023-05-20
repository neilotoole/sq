package record_test

import (
	"strconv"
	"testing"

	"github.com/neilotoole/sq/libsq/core/timez"

	"github.com/samber/lo"

	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/stretchr/testify/require"
)

func TestEqual(t *testing.T) {
	mar1UTC, _ := timez.ParseDateUTC("2023-03-01")
	mar31UTC, _ := timez.ParseDateUTC("2023-03-31")

	stdRec1 := record.Record{
		nil,
		lo.ToPtr(int64(1)),
		lo.ToPtr(1.1),
		lo.ToPtr(false),
		lo.ToPtr("a"),
		lo.ToPtr([]byte("a")),
		lo.ToPtr(mar1UTC),
	}

	stdRec2 := record.Record{
		nil,
		lo.ToPtr(int64(2)),
		lo.ToPtr(2.2),
		lo.ToPtr(true),
		lo.ToPtr("b"),
		lo.ToPtr([]byte("b")),
		lo.ToPtr(mar31UTC),
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
		tc := tc
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := record.Valid(nil, tc.a)
			require.NoError(t, err)
			_, err = record.Valid(nil, tc.b)
			require.NoError(t, err)

			got := record.Equal(tc.a, tc.b)
			require.Equal(t, tc.want, got)
		})
	}
}
