package datasize_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/datasize"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/testh/tu"
)

func TestOpt(t *testing.T) {
	testCases := []struct {
		key        string
		defaultVal int
		input      any
		want       datasize.ByteSize
		wantErr    bool
	}{
		{"int", 8, int(7), 7, false},
		{"uint", 8, uint(7), 7, false},
		{"int64", 8, int64(7), 7, false},
		{"uint64", 8, uint64(7), 7, false},
		{"string", 8, "7", 7, false},
		{"string_mb", 8, "7MB", datasize.MustParseString("7MB"), false},
		{"string_mb_bad", 8, "7M B ", 0, true},
		{"string_gb", 8, "7GB", datasize.MustParseString("7GB"), false},
		{"string_bad", 8, "not_int", 0, true},
		{"nil", 8, nil, 8, false},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.key), func(t *testing.T) {
			reg := &options.Registry{}

			opt := datasize.NewOpt(tc.key, nil, datasize.ByteSize(tc.defaultVal), "Use me", "Help me")
			reg.Add(opt)

			o := options.Options{tc.key: tc.input}

			o2, err := reg.Process(o)
			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, o2)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, o2)

			got := opt.Get(o2)
			require.Equal(t, tc.want, got)
		})
	}
}
