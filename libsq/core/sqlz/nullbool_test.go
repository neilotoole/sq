package sqlz_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/sqlz"
)

func TestNullBool_Scan(t *testing.T) {
	var tests = []struct {
		input       any
		expectValid bool
		expectBool  bool
	}{
		{"yes", true, true},
		{"Yes", true, true},
		{"YES", true, true},
		{"Y", true, true},
		{" Yes ", true, true},

		{"no", true, false},
		{"No", true, false},
		{"NO", true, false},
		{"N", true, false},
		{" No ", true, false},

		// check that the pre-existing sql.NullBool stuff works
		{"true", true, true},
		{true, true, true},
		{1, true, true},
		{"1", true, true},
		{"false", true, false},
		{false, true, false},
		{0, true, false},
		{"0", true, false},
		{"garbage", false, false},
	}

	for i, tt := range tests {
		var nb sqlz.NullBool

		err := nb.Scan(tt.input)
		if err != nil {
			if tt.expectValid == false {
				continue
			}
			t.Errorf("[%d] %q: did not expect error: %v", i, tt.input, err)
			continue
		}

		require.Nil(t, err, "[%d] %q: did not expect error: %v", i, tt.input, err)
		require.Equal(t, tt.expectValid, nb.Valid, "[%d] %q: expected Valid to be %v but got %v", i, tt.input,
			tt.expectValid, nb.Valid)
		require.Equal(t, tt.expectBool, nb.Bool, "[%d] %q: expected Bool to be %v but got %v", i, tt.input,
			tt.expectBool, nb.Bool)
	}
}
