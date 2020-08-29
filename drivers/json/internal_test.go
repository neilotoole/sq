package json

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/testh/sakila"
)

// export for testing
var (
	ImportJSON         = importJSON
	ImportJSONA        = importJSONA
	ImportJSONL        = importJSONL
	ScanObjectsInArray = scanObjectsInArray
)

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

			kinds, _, err := detectColKindsJSONA(context.Background(), f)
			require.NoError(t, err)
			require.Equal(t, tc.wantKinds, kinds)
		})
	}
}
