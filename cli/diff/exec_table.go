package diff

import (
	"context"
	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// ExecTableDiff is the entrypoint to diff two tables, @handle1.table1 and
// @handle2.table2.
//
// Contrast with [ExecSourceDiff], which diffs two sources.
func ExecTableDiff(ctx context.Context, ru *run.Run, cfg *Config, elems *Elements, //nolint:revive
	handle1, table1, handle2, table2 string,
) error {
	td1, td2 := &tableData{tblName: table1}, &tableData{tblName: table2}

	source.ParseTableHandle()

	var err error
	if td1.src, err = ru.Config.Collection.Get(handle1); err != nil {
		return err
	}
	if td2.src, err = ru.Config.Collection.Get(handle2); err != nil {
		return err
	}

	var differs []*diffdoc.Differ
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

		doc := diffdoc.NewUnifiedDoc(diffdoc.Titlef(cfg.Colors,
			"sq diff --schema %s %s", td1.String(), td2.String()))
		differ := diffdoc.NewDiffer(doc, func(ctx context.Context, _ func(error)) {
			diffTableSchema(ctx, cfg, elems.RowCount, td1, td2, doc)
		})
		differs = append(differs, differ)
	}

	if elems.Data {
		differ := differForTableData(ru, cfg, td1, td2)
		differs = append(differs, differ)
	}

	return diffdoc.Execute(ctx, ru.Out, cfg.Concurrency, differs)
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
