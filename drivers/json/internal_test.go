package json

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh/sakila"
)

// export for testing.
var (
	ImportJSON      = importJSON
	ImportJSONA     = importJSONA
	ImportJSONL     = importJSONL
	ColumnOrderFlat = columnOrderFlat
	NewImportJob    = newImportJob
)

// newImportJob is a constructor for the unexported importJob type.
// If sampleSize <= 0, a default value is used.
func newImportJob(fromSrc *source.Source, openFn source.FileOpenFunc, destDB driver.Database, sampleSize int,
	flatten bool,
) importJob {
	if sampleSize <= 0 {
		sampleSize = driver.OptIngestSampleSize.Get(fromSrc.Options)
	}

	return importJob{
		fromSrc:    fromSrc,
		openFn:     openFn,
		destDB:     destDB,
		sampleSize: sampleSize,
		flatten:    flatten,
	}
}

func TestDetectColKindsJSONA(t *testing.T) {
	testCases := []struct {
		tbl       string
		wantKinds []kind.Kind
	}{
		{tbl: sakila.TblActor, wantKinds: sakila.TblActorColKinds()},
		{tbl: sakila.TblFilmActor, wantKinds: sakila.TblFilmActorColKinds()},
		{tbl: sakila.TblPayment, wantKinds: sakila.TblPaymentColKinds()},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.tbl, func(t *testing.T) {
			f, err := os.Open(fmt.Sprintf("testdata/%s.jsona", tc.tbl))
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, f.Close()) })

			kinds, _, err := detectColKindsJSONA(context.Background(), f, 1000)
			require.NoError(t, err)
			require.Equal(t, tc.wantKinds, kinds)
		})
	}
}

// ScanObjectsInArray is a convenience function
// for objectsInArrayScanner.
func ScanObjectsInArray(r io.Reader) (objs []map[string]any, chunks [][]byte, err error) {
	sc := newObjectInArrayScanner(r)

	for {
		var obj map[string]any
		var chunk []byte

		obj, chunk, err = sc.next()
		if err != nil {
			return nil, nil, err
		}

		if obj == nil {
			// No more objects to be scanned
			break
		}

		objs = append(objs, obj)
		chunks = append(chunks, chunk)
	}

	return objs, chunks, nil
}
