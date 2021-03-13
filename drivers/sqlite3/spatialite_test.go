package sqlite3_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/tutil"
)

const (
	spatialite_testdb1 = "testdata/spatialite_test-2.3.sqlite"
	spatialite_testdb2 = "testdata/spatialite_aequilibrae_reference_files_spatialite.sqlite"
)

func TestSpatialiteExtension(t *testing.T) {
	testCases := []string{
		spatialite_testdb1,
		spatialite_testdb2,
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tutil.Name(tc), func(t *testing.T) {
			fp, err := filepath.Abs(tc)
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
