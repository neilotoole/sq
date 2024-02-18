package diff

import (
	"context"
	"io"
	"slices"

	"github.com/neilotoole/sq/libsq/core/langz"

	"github.com/neilotoole/sq/libsq/core/libdiff"

	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/tuning"
)

// execDiffSourceTablesData compares the row data of each table in sd1 and sd2.
func execDiffSourceTablesData(ctx context.Context, ru *run.Run, cfg *Config, sd1, sd2 *sourceData) (err error) {
	log := lg.FromContext(ctx).With(lga.Left, sd1.src.Handle, lga.Right, sd2.src.Handle)
	log.Info("Diffing source tables data")

	allTblNames := append(sd1.srcMeta.TableNames(), sd2.srcMeta.TableNames()...)
	allTblNames = lo.Uniq(allTblNames)
	slices.Sort(allTblNames)

	var cancelFn context.CancelCauseFunc
	ctx, cancelFn = context.WithCancelCause(ctx)
	defer func() { cancelFn(err) }()

	docs := make([]*libdiff.HunkDoc, len(allTblNames))
	defer func() {
		for i := range docs {
			lg.WarnIfCloseError(log, lgm.CloseDiffDoc, docs[i])
		}
	}()

	execFns := make([]func() error, len(allTblNames))
	for i, tblName := range allTblNames {
		td1 := &tableData{src: sd1.src, tblName: tblName}
		td1.tblMeta = sd1.srcMeta.Table(tblName)

		// REVISIT: What if there isn't table metadata? Or is it guaranteed to
		// be present?

		td2 := &tableData{src: sd2.src, tblName: tblName}
		td2.tblMeta = sd2.srcMeta.Table(tblName)

		doc := libdiff.NewHunkDoc(
			libdiff.Titlef(cfg.Colors, "sq diff --data %s %s", td1, td2),
			libdiff.Headerf(cfg.Colors, td1.String(), td2.String()),
		)
		docs[i] = doc
		execFns[i] = func() error {
			execDiffTableData(ctx, cancelFn, ru, cfg, td1, td2, doc)
			return doc.Err()
		}
	}

	g := &errgroup.Group{}
	g.SetLimit(tuning.OptErrgroupLimit.Get(options.FromContext(ctx)))
	for i := range execFns {
		g.Go(execFns[i])
	}

	if err = g.Wait(); err != nil {
		cancelFn(err)
		return err
	}

	rdr := io.MultiReader(langz.MustTypedSlice[io.Reader](docs...)...)
	_, err = io.Copy(ru.Out, contextio.NewReader(ctx, rdr))
	cancelFn(err)
	return err
}
