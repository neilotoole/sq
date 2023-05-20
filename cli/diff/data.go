package diff

import (
	"context"

	"github.com/neilotoole/sq/libsq/core/errz"

	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/neilotoole/sq/libsq/core/sqlz"

	"github.com/neilotoole/sq/libsq"

	"github.com/neilotoole/sq/cli/run"
)

//
// func buildTableDataDiffOld(ctx context.Context, ru *run.Run, td1, td2 *tableData) (*dataDiff, error) {
//	qc := &libsq.QueryContext{
//		Collection:   ru.Config.Collection,
//		DBOpener:     ru.Databases,
//		JoinDBOpener: ru.Databases,
//	}
//
//	query1 := td1.src.Handle + "." + stringz.DoubleQuote(td1.tblName)
//	recw1 := newSyncRecordWriter(ctx)
//	adapter1 := output.NewRecordWriterAdapter(recw1)
//	execErr := libsq.ExecuteSLQ(ctx, qc, query1, adapter1)
//	written, waitErr := adapter1.Wait()
//	if execErr != nil {
//		return nil, execErr
//	}
//
//	if waitErr != nil {
//		return nil, waitErr
//	}
//
//	time.Sleep(time.Second)
//
//	fmt.Fprintf(ru.Out, "written: %d\n", written)
//
//	return nil, nil
// }

func buildTableDataDiff(ctx context.Context, ru *run.Run, td1, td2 *tableData) (*dataDiff, error) { //nolint:unparam
	qc := &libsq.QueryContext{
		Collection:   ru.Config.Collection,
		DBOpener:     ru.Databases,
		JoinDBOpener: ru.Databases,
	}

	query1 := td1.src.Handle + "." + stringz.DoubleQuote(td1.tblName)
	query2 := td2.src.Handle + "." + stringz.DoubleQuote(td2.tblName)

	errCh := make(chan error, 5)

	recw1 := &dataRecordWriter{
		recCh: make(chan sqlz.Record, adapterRecChSize),
		errCh: errCh,
	}

	recw2 := &dataRecordWriter{
		recCh: make(chan sqlz.Record, adapterRecChSize),
		errCh: errCh,
	}

	ctx2, cancelFn := context.WithCancel(ctx)
	go func() {
		err := libsq.ExecuteSLQ(ctx2, qc, query1, recw1)
		if err != nil {
			cancelFn()
			select {
			case errCh <- err:
			default:
			}
		}
	}()
	go func() {
		err := libsq.ExecuteSLQ(ctx2, qc, query2, recw2)
		if err != nil {
			cancelFn()
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	var rec1, rec2 sqlz.Record
	var err error

	for {
		rec1 = nil
		rec2 = nil
		err = nil

		select {
		case rec1 = <-recw1.recCh:
		case <-ctx.Done():
			err = ctx.Err()
		case err = <-errCh:
		}
		if err != nil {
			cancelFn()
			break
		}

		select {
		case rec2 = <-recw2.recCh:
		case <-ctx.Done():
			err = ctx.Err()
		case err = <-errCh:
		}
		if err != nil {
			cancelFn()
			break
		}
	}

	_ = rec1
	_ = rec2

	return nil, errz.New("not implemented")
}

var _ libsq.RecordWriter = (*dataRecordWriter)(nil)

type dataRecordWriter struct {
	recCh chan sqlz.Record
	errCh chan error
}

// Open implements libsq.RecordWriter.
func (d *dataRecordWriter) Open(_ context.Context, _ context.CancelFunc, _ sqlz.RecordMeta,
) (recCh chan<- sqlz.Record, errCh <-chan error, err error) {
	return d.recCh, d.errCh, nil
}

// Wait implements libsq.RecordWriter.
func (d *dataRecordWriter) Wait() (written int64, err error) {
	// We don't actually use this.
	return 0, nil
}

// var _ output.RecordWriter = (*syncRecordWriter)(nil)

// adapterRecChSize is the size of the record chan (effectively
// the buffer) used by RecordWriterAdapter.
// FIXME: adapterRecChSize should be user-configurable.
const adapterRecChSize = 1000

//
// func newSyncRecordWriter(ctx context.Context) *syncRecordWriter { //nolint:unused
// 	return &syncRecordWriter{
// 		ctx:   ctx,
// 		recCh: make(chan sqlz.Record, adapterRecChSize),
// 	}
// }
//
// type syncRecordWriter struct {
// 	ctx     context.Context
// 	recCh   chan sqlz.Record
// 	written atomic.Int64
// }
//
// func (s syncRecordWriter) Open(recMeta sqlz.RecordMeta) error {
// 	return nil
// }
//
// func (s syncRecordWriter) WriteRecords(recs []sqlz.Record) error {
// 	fmt.Printf("recs: %d\n", len(recs))
// 	for i := range recs {
// 		select {
// 		case <-s.ctx.Done():
// 			return s.ctx.Err()
// 		case s.recCh <- recs[i]:
// 			s.written.Inc()
// 		}
// 	}
// 	return nil
// }
//
// func (s syncRecordWriter) Flush() error {
// 	return nil
// }
//
// func (s syncRecordWriter) Close() error {
// 	return nil
// }
