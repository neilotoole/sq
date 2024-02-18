package diff

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/libdiff"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/samber/lo"
)

func buildSourceTableStructureDiffs(ctx context.Context, cfg *Config, showRowCounts bool,
	sd1, sd2 *sourceData,
) ([]*tableDiff, error) {
	allTblNames := append(sd1.srcMeta.TableNames(), sd2.srcMeta.TableNames()...)
	allTblNames = lo.Uniq(allTblNames)
	slices.Sort(allTblNames)

	var diffs []*tableDiff
	for _, tblName := range allTblNames {
		select {
		case <-ctx.Done():
			return nil, errz.Err(ctx.Err())
		default:
		}

		td1 := &tableData{
			tblName: tblName,
			tblMeta: sd1.srcMeta.Table(tblName),
			src:     sd1.src,
			srcMeta: sd1.srcMeta,
		}
		td2 := &tableData{
			tblName: tblName,
			tblMeta: sd2.srcMeta.Table(tblName),
			src:     sd2.src,
			srcMeta: sd2.srcMeta,
		}

		dff, err := buildTableStructureDiff(ctx, cfg, showRowCounts, td1, td2)
		if err != nil {
			return nil, err
		}

		diffs = append(diffs, dff)
	}

	return diffs, nil
}

func execTableStructureDiff(ctx context.Context, cfg *Config, showRowCounts bool,
	td1, td2 *tableData, doc *libdiff.UnifiedDoc,
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
	unified, err := libdiff.ComputeUnified(ctx, handle1, handle2, cfg.Lines, body1, body2)
	bar.Stop()
	if err != nil {
		return
	}

	_, err = io.Copy(doc, libdiff.NewColorizer(cfg.Colors, strings.NewReader(unified)))
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
