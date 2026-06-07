package parquet_test

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/parquet"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

func nopLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestDriverMetadata(t *testing.T) {
	d, err := (&parquet.Provider{Log: nopLogger()}).DriverFor(drivertype.Parquet)
	require.NoError(t, err)
	md := d.DriverMetadata()
	require.Equal(t, drivertype.Parquet, md.Type)
	require.True(t, md.Monotable)
	require.Equal(t, "Apache Parquet", md.Description)
}

func TestDriverFor_WrongType(t *testing.T) {
	_, err := (&parquet.Provider{Log: nopLogger()}).DriverFor(drivertype.CSV)
	require.Error(t, err)
}

func TestValidateSource_RejectsWrongType(t *testing.T) {
	d, err := (&parquet.Provider{Log: nopLogger()}).DriverFor(drivertype.Parquet)
	require.NoError(t, err)
	_, err = d.ValidateSource(&source.Source{Type: drivertype.CSV})
	require.Error(t, err)
}

func TestValidateSource_AcceptsParquet(t *testing.T) {
	d, err := (&parquet.Provider{Log: nopLogger()}).DriverFor(drivertype.Parquet)
	require.NoError(t, err)
	src := &source.Source{Type: drivertype.Parquet, Handle: "@p", Location: "/tmp/x.parquet"}
	got, err := d.ValidateSource(src)
	require.NoError(t, err)
	require.Same(t, src, got)
}
