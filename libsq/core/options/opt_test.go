package options_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/options"
)

// TestOptGetRawValue verifies that Opt.Get converts a raw config value, of the
// kind that YAML unmarshalling produces, into the Opt's type. Without this, a
// consumer reading an Options that hasn't been through Registry.Process gets
// the Opt's default instead of the configured value, silently. See #1209.
//
// Each case is asserted twice: once against the raw Options, and once against
// the same Options after processing. Get must agree with itself either way,
// which is the invariant whose absence caused #1165.
func TestOptGetRawValue(t *testing.T) {
	testCases := []struct {
		name string
		opt  options.Opt
		raw  any
		want any
	}{
		{
			name: "duration from string",
			opt:  options.NewDuration("d", nil, 2*time.Second, "", ""),
			raw:  "100s",
			want: 100 * time.Second,
		},
		{
			name: "duration already typed",
			opt:  options.NewDuration("d", nil, 2*time.Second, "", ""),
			raw:  90 * time.Second,
			want: 90 * time.Second,
		},
		{
			name: "int from string",
			opt:  options.NewInt("i", nil, 7, "", ""),
			raw:  "42",
			want: 42,
		},
		{
			name: "int from float64",
			opt:  options.NewInt("i", nil, 7, "", ""),
			raw:  float64(42),
			want: 42,
		},
		{
			name: "bool from string",
			opt:  options.NewBool("b", nil, false, "", ""),
			raw:  "true",
			want: true,
		},
		{
			name: "string from int",
			opt:  options.NewString("s", nil, "fallback", nil, "", ""),
			raw:  123,
			want: "123",
		},
		{
			name: "string from bool",
			opt:  options.NewString("s", nil, "fallback", nil, "", ""),
			raw:  true,
			want: "true",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := tc.opt.Key()
			raw := options.Options{key: tc.raw}

			require.Equal(t, tc.want, tc.opt.GetAny(raw),
				"Get must convert the raw value, not fall back to the default")

			reg := &options.Registry{}
			reg.Add(tc.opt)
			processed, err := reg.Process(raw)
			require.NoError(t, err)
			require.Equal(t, tc.want, tc.opt.GetAny(processed),
				"Get must agree with itself on processed and unprocessed options")
		})
	}
}

// TestOptGetUnconvertibleValue verifies that a value that can't be converted
// still yields the Opt's default. Get has no error channel; rejecting a bad
// value remains Process's job.
func TestOptGetUnconvertibleValue(t *testing.T) {
	optDur := options.NewDuration("d", nil, 2*time.Second, "", "")
	require.Equal(t, 2*time.Second, optDur.Get(options.Options{"d": "not-a-duration"}))

	optInt := options.NewInt("i", nil, 7, "", "")
	require.Equal(t, 7, optInt.Get(options.Options{"i": "not-an-int"}))

	optBool := options.NewBool("b", nil, false, "", "")
	require.Equal(t, false, optBool.Get(options.Options{"b": "not-a-bool"}))
}
