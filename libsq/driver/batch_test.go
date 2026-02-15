package driver_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestNewBatchInsert(t *testing.T) {
	// This value is chosen as it's not a neat divisor of 200 (sakila.TblActorSize).
	const batchSize = 70

	for _, handle := range sakila.SQLAll() {
		t.Run(handle, func(t *testing.T) {
			th, src, drvr, _, db := testh.NewWith(t, handle)

			tblName := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, false)
			conn, err := db.Conn(th.Context)
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = conn.Close()
			})

			// Get records from TblActor that we'll write to the new tbl
			recMeta, recs := testh.RecordsFromTbl(t, handle, sakila.TblActor)
			bi, err := drvr.NewBatchInsert(
				th.Context,
				"Insert records",
				conn,
				src,
				tblName,
				recMeta.Names(),
				batchSize,
			)
			require.NoError(t, err)

			for _, rec := range recs {
				err = bi.Munge(rec)
				require.NoError(t, err)

				select {
				case <-th.Context.Done():
					close(bi.RecordCh)
					// Should never happen
					t.Fatal(th.Context.Err())
				case err = <-bi.ErrCh:
					close(bi.RecordCh)
					// Should not happen
					t.Fatal(err)
				case bi.RecordCh <- rec:
				}
			}
			close(bi.RecordCh) // Indicates end of records

			err = <-bi.ErrCh
			require.Nil(t, err)

			require.NoError(t, conn.Close())

			sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+tblName)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
			th.TruncateTable(src, tblName) // cleanup
		})
	}
}
