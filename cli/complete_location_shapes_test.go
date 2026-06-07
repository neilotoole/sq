package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
)

func TestDriverShape_Postgres(t *testing.T) {
	th := testh.New(t)
	drvr, err := th.Registry().SQLDriverFor(drivertype.Pg)
	require.NoError(t, err)
	shape := drvr.LocationShape()
	require.Equal(t, drivertype.Pg, shape.Type)
	require.Equal(t, []string{"postgres"}, shape.Schemes)
	require.Len(t, shape.Segments, 4)
	require.Equal(t, driver.SegCredentials, shape.Segments[0].Kind)
	require.True(t, shape.Segments[0].Optional)
	require.Equal(t, driver.SegAuthority, shape.Segments[1].Kind)
	require.False(t, shape.Segments[1].Optional)
	require.Equal(t, driver.SegPathName, shape.Segments[2].Kind)
	require.True(t, shape.Segments[2].Optional)
	require.Equal(t, "db", shape.Segments[2].Placeholder)
	require.Equal(t, driver.SegConnParams, shape.Segments[3].Kind)
	require.True(t, shape.Segments[3].Optional)
}

func TestDriverShape_MySQL(t *testing.T) {
	th := testh.New(t)
	drvr, err := th.Registry().SQLDriverFor(drivertype.MySQL)
	require.NoError(t, err)
	shape := drvr.LocationShape()
	require.Equal(t, drivertype.MySQL, shape.Type)
	require.Equal(t, []string{"mysql"}, shape.Schemes)
	require.Len(t, shape.Segments, 4)
	require.Equal(t, driver.SegCredentials, shape.Segments[0].Kind)
	require.Equal(t, driver.SegAuthority, shape.Segments[1].Kind)
	require.Equal(t, driver.SegPathName, shape.Segments[2].Kind)
	require.Equal(t, "db", shape.Segments[2].Placeholder)
	require.Equal(t, driver.SegConnParams, shape.Segments[3].Kind)
}

func TestDriverShape_SQLServer(t *testing.T) {
	th := testh.New(t)
	drvr, err := th.Registry().SQLDriverFor(drivertype.MSSQL)
	require.NoError(t, err)
	shape := drvr.LocationShape()
	require.Equal(t, drivertype.MSSQL, shape.Type)
	require.Equal(t, []string{"sqlserver"}, shape.Schemes)
	require.Len(t, shape.Segments, 4)
	require.Equal(t, driver.SegCredentials, shape.Segments[0].Kind)
	require.Equal(t, driver.SegAuthority, shape.Segments[1].Kind)
	require.Equal(t, driver.SegPathName, shape.Segments[2].Kind)
	require.Equal(t, "instance", shape.Segments[2].Placeholder)
	require.Equal(t, driver.SegConnParams, shape.Segments[3].Kind)
	require.Equal(t, "database", shape.Segments[3].LeadingKey)
}

func TestDriverShape_SQLite3(t *testing.T) {
	th := testh.New(t)
	drvr, err := th.Registry().SQLDriverFor(drivertype.SQLite)
	require.NoError(t, err)
	shape := drvr.LocationShape()
	require.Equal(t, drivertype.SQLite, shape.Type)
	require.Equal(t, []string{"sqlite3"}, shape.Schemes)
	require.Len(t, shape.Segments, 2)
	require.Equal(t, driver.SegPathFile, shape.Segments[0].Kind)
	require.False(t, shape.Segments[0].Optional)
	require.Equal(t, driver.SegConnParams, shape.Segments[1].Kind)
	require.True(t, shape.Segments[1].Optional)
}
