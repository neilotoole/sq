package json

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh/sakila"
)

// export for testing.
var (
	IngestJSON      = ingestJSON
	IngestJSONA     = ingestJSONA
	IngestJSONL     = ingestJSONL
	ColumnOrderFlat = columnOrderFlat
	NewIngestJob    = newIngestJob
)

// newIngestJob is a constructor for the unexported ingestJob type.
// If sampleSize <= 0, a default value is used.
func newIngestJob(fromSrc *source.Source, newRdrFn files.NewReaderFunc, destGrip driver.Grip, sampleSize int,
	flatten bool,
) *ingestJob {
	if sampleSize <= 0 {
		sampleSize = driver.OptIngestSampleSize.Get(fromSrc.Options)
	}

	return &ingestJob{
		fromSrc:    fromSrc,
		newRdrFn:   newRdrFn,
		destGrip:   destGrip,
		sampleSize: sampleSize,
		flatten:    flatten,
		stmtCache:  map[string]*driver.StmtExecer{},
	}
}

func TestDetectColKindsJSONA(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		tbl       string
		wantKinds []kind.Kind
	}{
		{tbl: sakila.TblActor, wantKinds: sakila.TblActorColKinds()},
		{tbl: sakila.TblFilmActor, wantKinds: sakila.TblFilmActorColKinds()},
		{tbl: sakila.TblPayment, wantKinds: sakila.TblPaymentColKinds()},
	}

	for _, tc := range testCases {
		t.Run(tc.tbl, func(t *testing.T) {
			t.Parallel()

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
func ScanObjectsInArray(log *slog.Logger, r io.Reader) (objs []map[string]any, chunks [][]byte, err error) {
	sc := newObjectInArrayScanner(log, r)

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

func TestCannotBeJSON(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		in   io.Reader
		want bool
	}{
		{in: strings.NewReader(""), want: true},
		{in: strings.NewReader("{"), want: true},
		{in: strings.NewReader("{}"), want: false},
		{in: strings.NewReader(`{"key": "value"}`), want: false},
		{in: strings.NewReader(`abcd`), want: true},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()

			got := cannotBeJSON(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}
