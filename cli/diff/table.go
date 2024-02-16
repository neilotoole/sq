package diff

import (
	"context"
	"fmt"
	"github.com/neilotoole/sq/cli/diff/libdiff"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/progress"
	"io"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// ExecTableDiff diffs handle1.table1 and handle2.table2.
func ExecTableDiff(ctx context.Context, ru *run.Run, cfg *Config, elems *Elements, //nolint:revive
	handle1, table1, handle2, table2 string,
) error {
	cfg.prMain = ru.Writers.OutPrinting.Clone()
	cfg.prDiff = cfg.prMain.Diff.Clone()

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

	if elems.Schema {
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
		tblDiff, err = buildTableStructureDiff(ctx, cfg, elems.RowCount, td1, td2)
		if err != nil {
			return err
		}

		if err = Print(
			ctx,
			ru.Out,
			cfg.prDiff,
			tblDiff.header,
			strings.NewReader(tblDiff.diff),
		); err != nil {
			return err
		}
	}

	if !elems.Data {
		return nil
	}

	doc := NewHunkDoc("", NewDocHeader(cfg.prDiff, td1.String(), td2.String()))
	if err = execTableDataDiffDoc(ctx, ru, cfg, doc, td1, td2); err != nil {
		return err
	}

	_, err = io.Copy(ru.Out, contextio.NewReader(ctx, doc))
	return errz.Err(err)
}

func buildTableStructureDiff(ctx context.Context, cfg *Config, showRowCounts bool,
	td1, td2 *tableData,
) (*tableDiff, error) {
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

	handle1 := td1.src.Handle + "." + td1.tblName
	handle2 := td2.src.Handle + "." + td2.tblName

	bar := progress.FromContext(ctx).NewWaiter("Diff table schema "+td1.String(), true, progress.OptMemUsage)
	unified, err := libdiff.ComputeUnified(ctx, handle1, handle2, cfg.Lines, body1, body2)
	bar.Stop()
	if err != nil {
		return nil, err
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

// fetchTableMeta returns the metadata.Table for table. If the table
// does not exist, {nil,nil} is returned.
func fetchTableMeta(ctx context.Context, ru *run.Run, src *source.Source, table string) (
	*metadata.Table, error,
) {
	grip, err := ru.Grips.Open(ctx, src)
	if err != nil {
		return nil, err
	}
	md, err := grip.TableMetadata(ctx, table)
	if err != nil {
		if errz.Has[*driver.NotExistError](err) {
			return nil, nil //nolint:nilnil
		}
		return nil, err
	}

	return md, nil
}
