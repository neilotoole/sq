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
func ExecTableDiff(ctx context.Context, ru *run.Run, cfg *Config, elems *Elements,
	handle1, table1, handle2, table2 string,
) error {
	td1, td2 := &tableData{tblName: table1}, &tableData{tblName: table2}

	var err error
	td1.src, err = ru.Config.Collection.Get(handle1)
	if err != nil {
		return err
	}
	td2.src, err = ru.Config.Collection.Get(handle2)
	if err != nil {
		return err
	}

	if elems.Table {
		g, gCtx := errgroup.WithContext(ctx)
		g.Go(func() error {
			var gErr error
			td1.tblMeta, gErr = fetchTableMeta(gCtx, ru, td1.src, table1)
			return gErr
		})
		g.Go(func() error {
			var gErr error
			td2.tblMeta, gErr = fetchTableMeta(gCtx, ru, td2.src, table2)
			return gErr
		})
		if err = g.Wait(); err != nil {
			return err
		}

		var tblDiff *tableDiff
		tblDiff, err = buildTableStructureDiff(cfg, elems.RowCount, td1, td2)
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

	tblDataDiff, err := buildTableDataDiff(ctx, ru, cfg, td1, td2)
	if err != nil {
		return err
	}

	if tblDataDiff == nil {
		return nil
	}

	return Print(ru.Out, ru.Writers.Printing, tblDataDiff.header, tblDataDiff.diff)
}

func buildTableStructureDiff(cfg *Config, showRowCounts bool, td1, td2 *tableData) (*tableDiff, error) {
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
		cfg.Lines,
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

// fetchTableMeta returns the source.TableMetadata for table. If the table
// does not exist, {nil,nil} is returned.
func fetchTableMeta(ctx context.Context, ru *run.Run, src *source.Source, table string) (
	*source.TableMetadata, error,
) {
	dbase, err := ru.Databases.Open(ctx, src)
	if err != nil {
		return nil, err
	}
	md, err := dbase.TableMetadata(ctx, table)
	if err != nil {
		if errz.IsErrNotExist(err) {
			return nil, nil //nolint:nilnil
		}
		return nil, err
	}

	return md, nil
}
