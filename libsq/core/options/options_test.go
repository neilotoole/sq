package options_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/output/format"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/testh/tu"
)

type config struct {
	Options options.Options `yaml:"options"`
}

func TestOptions(t *testing.T) {
	log := lgt.New(t)
	b, err := os.ReadFile("testdata/good.01.yml")
	require.NoError(t, err)

	reg := &options.Registry{}
	cli.RegisterDefaultOpts(reg)
	log.Debug("Registry", "reg", reg)

	cfg := &config{Options: options.Options{}}
	require.NoError(t, ioz.UnmarshallYAML(b, cfg))
	cfg.Options, err = reg.Process(cfg.Options)
	require.NoError(t, err)

	require.Equal(t, format.CSV, cli.OptFormat.Get(cfg.Options))
	require.Equal(t, true, cli.OptPrintHeader.Get(cfg.Options))
	require.Equal(t, time.Second*10, cli.OptPingCmdTimeout.Get(cfg.Options))
	require.Equal(t, time.Millisecond*500, cli.OptShellCompletionTimeout.Get(cfg.Options))

	require.Equal(t, 50, driver.OptConnMaxOpen.Get(cfg.Options))
	require.Equal(t, 100, driver.OptConnMaxIdle.Get(cfg.Options))
	require.Equal(t, time.Second*100, driver.OptConnMaxIdleTime.Get(cfg.Options))
	require.Equal(t, time.Minute*5, driver.OptConnMaxLifetime.Get(cfg.Options))
}

func TestInt(t *testing.T) {
	testCases := []struct {
		key        string
		defaultVal int
		input      any
		want       int
		wantErr    bool
	}{
		{"int", 8, 7, 7, false},
		{"uint", 8, uint(7), 7, false},
		{"int64", 8, int64(7), 7, false},
		{"uint64", 8, uint64(7), 7, false},
		{"string", 8, "7", 7, false},
		{"string_bad", 8, "not_int", 0, true},
		{"nil", 8, nil, 8, false},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.key), func(t *testing.T) {
			reg := &options.Registry{}

			opt := options.NewInt(tc.key, "", 0, tc.defaultVal, "", "")
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

func TestBool(t *testing.T) {
	testCases := []struct {
		key        string
		defaultVal bool
		input      any
		want       bool
		wantErr    bool
	}{
		{"bool_true", false, true, true, false},
		{"bool_false", true, false, false, false},
		{"int_1", false, int(1), true, false},
		{"int_0", true, int(0), false, false},
		{"string_int_1", false, "1", true, false},
		{"string_int_0", true, "0", false, false},
		{"string_true", false, "true", true, false},
		{"string_false", true, "false", false, false},
		{"string_bad", true, "not_bool", false, true},
		{"nil", true, nil, true, false},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.key), func(t *testing.T) {
			reg := &options.Registry{}

			opt := options.NewBool(tc.key, nil, tc.defaultVal, "", "")
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

func TestMerge(t *testing.T) {
	o1 := options.Options{"a": 1, "b": 1, "c": 1}
	o2 := options.Options{"b": 2, "c": 2}
	o3 := options.Options{"c": 3}

	got := options.Merge(o1, o2)
	require.NotEqual(t, o1, got)
	require.Equal(t, got, options.Options{"a": 1, "b": 2, "c": 2})
	got = options.Merge(o1, o2, o3)
	require.NotEqual(t, o1, got)
	require.Equal(t, got, options.Options{"a": 1, "b": 2, "c": 3})
}

func TestOptions_LogValue(t *testing.T) {
	o1 := options.Options{"a": 1, "b": true, "c": "hello"}
	log := lgt.New(t)

	log.Debug("Logging options", lga.Opts, o1)
}

func TestEffective(t *testing.T) {
	optHello := options.NewString("hello", "", 0, "world", nil, "", "")
	optCount := options.NewInt("count", "", 0, 1, "", "")

	in := options.Options{"count": 7}
	want := options.Options{"count": 7, "hello": "world"}
	got := options.Effective(in, optHello, optCount)
	require.Equal(t, want, got)
}

func TestDeleteNil(t *testing.T) {
	o := options.Options{"a": 1, "b": nil, "c": nil, "d": 2, "e": nil}
	got := options.DeleteNil(o)
	require.Lenf(t, o, 5, "o should not be modified")
	require.Equal(t, options.Options{"a": 1, "d": 2}, got)
}

func TestContext(t *testing.T) {
	ctx := context.Background()

	ctx = options.NewContext(ctx, nil)
	gotOpts := options.FromContext(ctx)
	require.Nil(t, gotOpts)

	opts := options.Options{"a": 1}
	ctx = options.NewContext(ctx, opts)
	gotOpts = options.FromContext(ctx)
	require.Equal(t, opts, gotOpts)
}
