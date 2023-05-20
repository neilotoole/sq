package diff

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/cli/diff/internal/go-udiff"
	"github.com/neilotoole/sq/cli/diff/internal/go-udiff/myers"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
)

// ExecTableDiff diffs handle1.table1 and handle2.table2.
func ExecTableDiff(ctx context.Context, ru *run.Run, numLines int, elems *Elements,
	handle1, table1, handle2, table2 string,
) error {
	td1, td2 := &tableData{tblName: table1}, &tableData{tblName: table2}

	if elems.Table {
		g, gCtx := errgroup.WithContext(ctx)
		g.Go(func() error {
			var err error
			td1.src, td1.tblMeta, err = fetchTableMeta(gCtx, ru, handle1, table1)
			return err
		})
		g.Go(func() error {
			var err error
			td2.src, td2.tblMeta, err = fetchTableMeta(gCtx, ru, handle2, table2)
			return err
		})
		if err := g.Wait(); err != nil {
			return err
		}

		tblDiff, err := buildTableDiff(numLines, elems.RowCount, td1, td2)
		if err != nil {
			return err
		}

		if err = Print(ru.Out, ru.Writers.Printing, tblDiff.header, tblDiff.diff); err != nil {
			return err
		}
	}

	if !elems.Data {
		return nil
	}

	// We want to diff table data. Make sure that the src has already
	// been set.
	if td1.src == nil {
		var err error
		if td1.src, err = ru.Config.Collection.Get(handle1); err != nil {
			return err
		}
	}

	if td2.src == nil {
		var err error
		if td2.src, err = ru.Config.Collection.Get(handle2); err != nil {
			return err
		}
	}

	recDiff, err := findDataDiffs(ctx, ru, numLines, td1, td2)
	if err != nil {
		return err
	}

	if recDiff == nil {
		return nil
	}

	return Print(ru.Out, ru.Writers.Printing, recDiff.header, recDiff.diff)
}

func buildTableDiff(lines int, showRowCounts bool, td1, td2 *tableData) (*tableDiff, error) {
	var (
		body1, body2 string
		err          error
	)

	if body1, err = renderTableMeta2YAML(showRowCounts, td1.tblMeta); err != nil {
		return nil, err
	}
	if body2, err = renderTableMeta2YAML(showRowCounts, td2.tblMeta); err != nil {
		return nil, err
	}

	edits := myers.ComputeEdits(body1, body2)
	unified, err := udiff.ToUnified(
		td1.src.Handle+"."+td1.tblName,
		td2.src.Handle+"."+td2.tblName,
		body1,
		edits,
		lines,
	)
	if err != nil {
		return nil, errz.Err(err)
	}

	tblDiff := &tableDiff{
		td1: td1,
		td2: td2,
		header: fmt.Sprintf("sq diff %s.%s %s.%s",
			td1.src.Handle, td1.tblName, td2.src.Handle, td2.tblName),
		diff: unified,
	}

	return tblDiff, nil
}

func fetchTableMeta(ctx context.Context, ru *run.Run, handle, table string) (
	*source.Source, *source.TableMetadata, error,
) {
	src, err := ru.Config.Collection.Get(handle)
	if err != nil {
		return nil, nil, err
	}
	dbase, err := ru.Databases.Open(ctx, src)
	if err != nil {
		return nil, nil, err
	}
	md, err := dbase.TableMetadata(ctx, table)
	if err != nil {
		return nil, nil, err
	}

	return src, md, nil
}
