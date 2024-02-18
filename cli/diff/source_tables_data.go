package diff

import (
	"context"
	"slices"

	"github.com/samber/lo"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/tuning"
)

// execDiffAllData compares the row data of each table in sd1 and sd2.
//
//nolint:unused
func execDiffAllData(ctx context.Context, ru *run.Run, cfg *Config, sd1, sd2 *sourceData) error {
	log := lg.FromContext(ctx).With(lga.Left, sd1.src.Handle, lga.Right, sd2.src.Handle)
	log.Info("Diffing source tables data")

	differs, err := prepareAllDataDiffers(ctx, ru, cfg, sd1, sd2)
	if err != nil {
		return err
	}

	concurrency := tuning.OptErrgroupLimit.Get(options.FromContext(ctx))
	return diffdoc.Execute(ctx, ru.Out, concurrency, differs)
}

// prepareAllDataDiffers compares the row data of each table in sd1 and sd2.
func prepareAllDataDiffers(ctx context.Context, ru *run.Run, cfg *Config, sd1, sd2 *sourceData,
) (execDocs []*diffdoc.Differ, err error) { //nolint:unparam
	log := lg.FromContext(ctx).With(lga.Left, sd1.src.Handle, lga.Right, sd2.src.Handle)
	log.Info("Diffing source tables data")

	allTblNames := append(sd1.srcMeta.TableNames(), sd2.srcMeta.TableNames()...)
	allTblNames = lo.Uniq(allTblNames)
	slices.Sort(allTblNames)

	differs := make([]*diffdoc.Differ, len(allTblNames))
	for i, tblName := range allTblNames {
		td1 := &tableData{src: sd1.src, tblName: tblName}
		td1.tblMeta = sd1.srcMeta.Table(tblName)

		// REVISIT: What if there isn't table metadata? Or is it guaranteed to
		// be present?

		td2 := &tableData{src: sd2.src, tblName: tblName}
		td2.tblMeta = sd2.srcMeta.Table(tblName)

		doc := diffdoc.NewHunkDoc(
			diffdoc.Titlef(cfg.Colors, "sq diff --data %s %s", td1, td2),
			diffdoc.Headerf(cfg.Colors, td1.String(), td2.String()),
		)

		differs[i] = diffdoc.NewDiffer(doc, func(ctx context.Context, cancelFn func(error)) {
			diffTableData(ctx, cancelFn, ru, cfg, td1, td2, doc)
		})
	}

	return differs, nil
}
