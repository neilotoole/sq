package sqlite3_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ryboe/q"
	"github.com/stretchr/testify/require"
	"github.com/twpayne/go-geom/encoding/wkb"

	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/testsrc"
	"github.com/neilotoole/sq/testh/tutil"
)

const (
	spatialiteTestDB23          = "testdata/spatialite-test-2.3.sqlite"
	spatialiteTestDBAequilibrae = "testdata/spatialite_aequilibrae_reference_files_spatialite.sqlite"
	spatialiteTestDBOSF         = "testdata/spatialite_osf_db_small.sqlite"
)

func TestSpatialiteExtension(t *testing.T) {
	testCases := []struct {
		testdb string
		query  string
	}{
		{testdb: spatialiteTestDB23},
		//{testdb: spatialiteTestDBAequilibrae},
		{testdb: spatialiteTestDBOSF},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tutil.Name(tc.testdb), func(t *testing.T) {
			fp, err := filepath.Abs(tc.testdb)
			require.NoError(t, err)

			src := &source.Source{
				Handle:   "@spatial1",
				Type:     sqlite3.Type,
				Location: "sqlite3://" + fp,
				Options:  options.Options{}.Add("spatialite", "true"),
			}

			th := testh.New(t)
			drvr := th.DriverFor(src)

			db, err := drvr.Open(th.Context, src)
			require.NoError(t, err)
			require.NotNil(t, db)

			err = db.DB().PingContext(th.Context)
			require.NoError(t, err)

			md, err := db.SourceMetadata(th.Context)
			require.NoError(t, err)

			t.Log(md)
		})
	}
}

// TestSpatialiteOSF executes tests against testsrc.SpatialiteOSF,
// which has a decent variety of spatial types.
func TestSpatialiteOSF(t *testing.T) {
	th := testh.New(t)
	src := th.Source(testsrc.SpatialiteOSF)

	dbase := th.Open(src)
	md, err := dbase.SourceMetadata(th.Context)
	require.NoError(t, err)
	require.NotNil(t, md)

	sink, err := th.QuerySQL(src, `SELECT ST_AsBinary(Geometry) FROM waterways_lines LIMIT 3`)
	require.NoError(t, err)
	require.Equal(t, 3, len(sink.Recs))

	rez := sink.Recs[0][0]
	var data []byte
	switch rez.(type) {
	case *string:
		data = []byte(*rez.(*string))
	case *[]byte:
		data = *rez.(*[]byte)
	default:
		panic(fmt.Sprintf("type is %T", rez))
	}

	t.Log("huzzah!", string(data))

	wkb1 := wkb.LineString{}
	err = wkb1.Scan(data)
	require.NoError(t, err)
	q.Q(wkb1)

}
