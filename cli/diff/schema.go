package diff

import (
	"context"
	"github.com/neilotoole/sq/libsq/source"
	"io"
	"slices"
	"strings"

	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/samber/lo"
)

func differsForSchema(ctx context.Context, cfg *Config, showRowCounts bool,
	sd1, sd2 *sourceData,
) (differs []*diffdoc.Differ, err error) {
	allTblNames := append(sd1.srcMeta.TableNames(), sd2.srcMeta.TableNames()...)
	allTblNames = lo.Uniq(allTblNames)
	slices.Sort(allTblNames)

	differs = make([]*diffdoc.Differ, 0, len(allTblNames))

	for _, tblName := range allTblNames {
		select {
		case <-ctx.Done():
			return nil, errz.Err(context.Cause(ctx))
		default:
		}

		td1 := &tableData{
			tbl:     source.Table{Handle: sd1.src.Handle, Name: tblName},
			tblName: tblName,
			tblMeta: sd1.srcMeta.Table(tblName),
			src:     sd1.src,
			srcMeta: sd1.srcMeta,
		}
		td2 := &tableData{
			tbl:     source.Table{Handle: sd2.src.Handle, Name: tblName},
			tblName: tblName,
			tblMeta: sd2.srcMeta.Table(tblName),
			src:     sd2.src,
			srcMeta: sd2.srcMeta,
		}

		doc := diffdoc.NewUnifiedDoc(diffdoc.Titlef(cfg.Colors, "sq diff %s %s", td1, td2))
		differs = append(differs, diffdoc.NewDiffer(doc, func(ctx context.Context, _ func(error)) {
			diffTableSchema(ctx, cfg, showRowCounts, td1, td2, doc)
		}))
	}

	return differs, nil
}

func diffTableSchema(ctx context.Context, cfg *Config, showRowCounts bool,
	td1, td2 *tableData, doc *diffdoc.UnifiedDoc,
) {
	var (
		body1, body2 string
		err          error
	)

	defer func() { doc.Seal(err) }()

	if body1, err = renderTableMeta2YAML(showRowCounts, td1.tblMeta); err != nil {
		return
	}
	if body2, err = renderTableMeta2YAML(showRowCounts, td2.tblMeta); err != nil {
		return
	}

	handle1 := td1.src.Handle + "." + td1.tblName
	handle2 := td2.src.Handle + "." + td2.tblName

	bar := progress.FromContext(ctx).NewWaiter("Diff table schema "+td1.String(), true, progress.OptMemUsage)
	unified, err := diffdoc.ComputeUnified(ctx, handle1, handle2, cfg.Lines, body1, body2)
	bar.Stop()
	if err != nil {
		return
	}

	_, err = io.Copy(doc, diffdoc.NewColorizer(cfg.Colors, strings.NewReader(unified)))
}
