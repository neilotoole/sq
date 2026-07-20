package datasize_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/datasize"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/testh/tu"
)

func TestParse(t *testing.T) {
	got, err := datasize.Parse([]byte("10MB"))
	require.NoError(t, err)
	require.Equal(t, datasize.MustParseString("10MB"), got)

	_, err = datasize.Parse([]byte("not_a_size"))
	require.Error(t, err)
}

func TestParseString(t *testing.T) {
	got, err := datasize.ParseString("7KB")
	require.NoError(t, err)
	require.Equal(t, datasize.MustParseString("7KB"), got)

	_, err = datasize.ParseString("bad input")
	require.Error(t, err)
}

func TestMustParse(t *testing.T) {
	require.Equal(t, datasize.MustParseString("1MB"), datasize.MustParse([]byte("1MB")))
	require.Panics(t, func() { datasize.MustParse([]byte("not_a_size")) })
}

func TestMustParseString(t *testing.T) {
	require.Equal(t, datasize.ByteSize(7), datasize.MustParseString("7"))
	require.Panics(t, func() { datasize.MustParseString("not_a_size") })
}

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
		{"bytesize", 8, datasize.ByteSize(7), 7, false},
		{"string", 8, "7", 7, false},
		{"string_mb", 8, "7MB", datasize.MustParseString("7MB"), false},
		{"string_mb_bad", 8, "7M B ", 0, true},
		{"string_gb", 8, "7GB", datasize.MustParseString("7GB"), false},
		{"string_bad", 8, "not_int", 0, true},
		{"wrong_type", 8, true, 0, true},
		{"nil", 8, nil, 8, false},
	}

	for i, tc := range testCases {
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

// TestOpt_Process_nilAndAbsent covers the early-return branches of Process:
// a nil Options, and an Options that does not contain the opt's key.
func TestOpt_Process_nilAndAbsent(t *testing.T) {
	opt := datasize.NewOpt("size", nil, datasize.ByteSize(8), "Use me", "Help me")

	// Nil Options returns nil, nil.
	got, err := opt.Process(nil)
	require.NoError(t, err)
	require.Nil(t, got)

	// Key absent: input is returned unchanged.
	o := options.Options{"other": "value"}
	got, err = opt.Process(o)
	require.NoError(t, err)
	require.Equal(t, o, got)

	// Key present but nil value: input is returned unchanged.
	o = options.Options{"size": nil}
	got, err = opt.Process(o)
	require.NoError(t, err)
	require.Equal(t, o, got)
}

func TestOpt_Default(t *testing.T) {
	opt := datasize.NewOpt("size", nil, datasize.MustParseString("4MB"), "Use me", "Help me")
	require.Equal(t, datasize.MustParseString("4MB"), opt.Default())
	require.Equal(t, datasize.MustParseString("4MB"), opt.DefaultAny())
}

func TestOpt_Get(t *testing.T) {
	opt := datasize.NewOpt("size", nil, datasize.ByteSize(8), "Use me", "Help me")

	// Nil Options returns the default.
	require.Equal(t, datasize.ByteSize(8), opt.Get(nil))

	// Key absent returns the default.
	require.Equal(t, datasize.ByteSize(8), opt.Get(options.Options{}))

	// Value of the wrong type returns the default.
	require.Equal(t, datasize.ByteSize(8), opt.Get(options.Options{"size": "not_a_bytesize"}))

	// Value of the correct type is returned.
	require.Equal(t, datasize.ByteSize(16), opt.Get(options.Options{"size": datasize.ByteSize(16)}))

	// GetAny delegates to Get.
	require.Equal(t, datasize.ByteSize(16), opt.GetAny(options.Options{"size": datasize.ByteSize(16)}))
}
