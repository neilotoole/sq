package xlsx

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
	excelize "github.com/xuri/excelize/v2"
	"go.uber.org/atomic"

	_ "github.com/mattn/go-sqlite3" // Register the sqlite3 driver for the fake grip.

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// earlyFinishDriver is a SQLDriver whose BatchInsert reports a clean finish
// (a nil on ErrCh) before RecordCh is closed. No real driver does this today.
type earlyFinishDriver struct {
	driver.SQLDriver
	recCh chan []any
}

func (d *earlyFinishDriver) NewBatchInsert(_ context.Context, _ string, _ sqlz.DB,
	_ *source.Source, _ string, _ []string,
) (*driver.BatchInsert, error) {
	d.recCh = make(chan []any) // Unbuffered, and nobody reads it.
	errCh := make(chan error)
	close(errCh) // A receive yields nil: "finished without error".
	return driver.NewBatchInsert(d.recCh, errCh, &atomic.Int64{}, func(record.Record) error { return nil }), nil
}

// earlyFinishGrip is a Grip backed by an in-memory sqlite db.
type earlyFinishGrip struct {
	driver.Grip
	db   *sql.DB
	drvr *earlyFinishDriver
}

func (g *earlyFinishGrip) DB(context.Context) (*sql.DB, error) { return g.db, nil }
func (g *earlyFinishGrip) SQLDriver() driver.SQLDriver         { return g.drvr }
func (g *earlyFinishGrip) Source() *source.Source              { return &source.Source{} }

// TestIngestSheetToTable_batchInsertFinishesEarly verifies that ingest fails,
// instead of silently dropping rows, when the batch inserter finishes cleanly
// before all sheet rows have been sent.
func TestIngestSheetToTable_batchInsertFinishesEarly(t *testing.T) {
	ctx := context.Background()

	f := excelize.NewFile()
	t.Cleanup(func() { _ = f.Close() })
	const sheetName = "Sheet1"
	require.NoError(t, f.SetSheetRow(sheetName, "A1", &[]any{"a", "b"}))
	require.NoError(t, f.SetSheetRow(sheetName, "A2", &[]any{"x", "y"}))

	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)

	tblDef := &schema.Table{
		Name: "t",
		Cols: []*schema.Column{{Name: "a", Kind: kind.Text}, {Name: "b", Kind: kind.Text}},
	}
	sheetTbl := &sheetTable{
		sheet:             &xSheet{file: f, name: sheetName},
		def:               tblDef,
		colIngestMungeFns: make([]kind.MungeFunc, 2),
		hasHeaderRow:      true,
	}

	grip := &earlyFinishGrip{db: db, drvr: &earlyFinishDriver{}}
	err = ingestSheetToTable(ctx, grip, sheetTbl)
	require.Error(t, err)
	require.Contains(t, err.Error(), "batch insert finished before all rows were sent")
}
