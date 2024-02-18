package diff

import (
	"context"
	"io"

	"github.com/neilotoole/sq/libsq/core/libdiff"

	"github.com/neilotoole/sq/libsq/core/langz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// ExecTableDiff diffs handle1.table1 and handle2.table2.
func ExecTableDiff(ctx context.Context, ru *run.Run, cfg *Config, elems *Elements, //nolint:revive
	handle1, table1, handle2, table2 string,
) error {
	log := lg.FromContext(ctx).With(lga.Left, handle1+"."+table1, lga.Right, handle2+"."+table2)
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

	var docs []libdiff.Doc
	defer func() {
		for i := range docs {
			lg.WarnIfCloseError(log, lgm.CloseDiffDoc, docs[i])
		}
	}()

	var execFns []func()
	var cancelFn context.CancelCauseFunc
	ctx, cancelFn = context.WithCancelCause(ctx)

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

		doc := libdiff.NewUnifiedDoc(libdiff.Titlef(cfg.Colors,
			"sq diff --schema %s %s", td1.String(), td2.String()))
		docs = append(docs, doc)
		execFns = append(execFns, func() {
			execTableStructureDiff(ctx, cfg, elems.RowCount, td1, td2, doc)
			if doc.Err() != nil {
				cancelFn(doc.Err())
			}
		})
	}

	if elems.Data {
		doc := libdiff.NewHunkDoc(
			libdiff.Titlef(cfg.Colors, "sq diff --data %s %s", td1, td2),
			libdiff.Headerf(cfg.Colors, td1.String(), td2.String()))
		docs = append(docs, doc)
		execFns = append(execFns, func() {
			execTableDataDiffDoc(ctx, cancelFn, ru, cfg, td1, td2, doc)
			if doc.Err() != nil {
				cancelFn(doc.Err())
			}
		})
	}

	if len(execFns) == 0 {
		// Shouldn't happen.
		return nil
	}

	rdr := io.MultiReader(langz.MustTypedSlice[io.Reader](docs...)...)

	for i := range execFns {
		go execFns[i]()
	}

	_, err = io.Copy(ru.Out, contextio.NewReader(ctx, rdr))
	return errz.Err(err)
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
