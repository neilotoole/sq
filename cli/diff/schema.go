package diff

import (
	"context"
	"io"
	"slices"
	"strings"

	"github.com/samber/lo"

	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/langz"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

func differsForSchema(ctx context.Context, cfg *Config, showRowCounts bool,
	src1, src2 *source.Source,
) (differs []*diffdoc.Differ, err error) {
	tbls1, tbls2, err := cfg.Run.MDCache.TableNamesPair(ctx, src1, src2)
	if err != nil {
		return nil, err
	}

	allTblNames := lo.Uniq(langz.JoinSlices(tbls1, tbls2))
	slices.Sort(allTblNames)

	differs = make([]*diffdoc.Differ, 0, len(allTblNames))

	for _, tblName := range allTblNames {
		select {
		case <-ctx.Done():
			return nil, errz.Err(context.Cause(ctx))
		default:
		}

		td1 := source.Table{Handle: src1.Handle, Name: tblName}
		td2 := source.Table{Handle: src2.Handle, Name: tblName}

		doc := diffdoc.NewUnifiedDoc(diffdoc.Titlef(cfg.Colors, "sq diff %s %s", td1, td2))
		differs = append(differs, diffdoc.NewDiffer(doc, func(ctx context.Context, _ func(error)) {
			diffTableSchema(ctx, cfg, showRowCounts, td1, td2, doc)
		}))
	}

	return differs, nil
}

func diffTableSchema(ctx context.Context, cfg *Config, showRowCounts bool,
	td1, td2 source.Table, doc *diffdoc.UnifiedDoc,
) {
	var (
		body1, body2 string
		md1, md2     *metadata.Table
		err          error
	)

	defer func() { doc.Seal(err) }()

	md1, md2, err = cfg.Run.MDCache.TableMetaPair(ctx, td1, td2)
	if err != nil {
		return
	}

	if body1, err = renderTableMeta2YAML(showRowCounts, md1); err != nil {
		return
	}
	if body2, err = renderTableMeta2YAML(showRowCounts, md2); err != nil {
		return
	}

	bar := progress.FromContext(ctx).NewWaiter("Diff table schema "+td1.String(), true, progress.OptMemUsage)
	unified, err := diffdoc.ComputeUnified(ctx, td1.String(), td2.String(), cfg.Lines, body1, body2)
	bar.Stop()
	if err != nil {
		return
	}

	_, err = io.Copy(doc, diffdoc.NewColorizer(cfg.Colors, strings.NewReader(unified)))
}
