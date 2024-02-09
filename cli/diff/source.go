package diff

import (
	"context"
	"fmt"
	"slices"

	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// ExecSourceDiff diffs handle1 and handle2.
func ExecSourceDiff(ctx context.Context, ru *run.Run, cfg *Config,
	elems *Elements, handle1, handle2 string,
) error {
	var (
		sd1 = &sourceData{handle: handle1}
		sd2 = &sourceData{handle: handle2}
	)

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		// TODO: This mechanism fetches the entire source metadata. That's
		// only necessary if both opts.DBProperties and opts.Tables are true.
		// This mechanism can be improved to only fetch the relevant data.
		sd1.src, sd1.srcMeta, err = fetchSourceMeta(gCtx, ru, handle1)
		return err
	})
	g.Go(func() error {
		var err error
		sd2.src, sd2.srcMeta, err = fetchSourceMeta(gCtx, ru, handle2)
		return err
	})
	if err := g.Wait(); err != nil {
		return err
	}

	if elems.Overview {
		srcDiff, err := buildSourceOverviewDiff(ctx, cfg, sd1, sd2)
		if err != nil {
			return err
		}

		if err = Print(ctx, ru.Out, ru.Writers.OutPrinting, srcDiff.header, srcDiff.diff); err != nil {
			return err
		}
	}

	if elems.DBProperties {
		propsDiff, err := buildDBPropsDiff(ctx, cfg, sd1, sd2)
		if err != nil {
			return err
		}
		if err = Print(ctx, ru.Out, ru.Writers.OutPrinting, propsDiff.header, propsDiff.diff); err != nil {
			return err
		}
	}

	if elems.Schema {
		tblDiffs, err := buildSourceTableDiffs(ctx, cfg, elems.RowCount, sd1, sd2)
		if err != nil {
			return err
		}
		for _, tblDiff := range tblDiffs {
			if err = Print(ctx, ru.Out, ru.Writers.OutPrinting, tblDiff.header, tblDiff.diff); err != nil {
				return err
			}
		}
	}

	if elems.Data {
		// We're going for it... diff all table data.
		return execSourceDataDiff(ctx, ru, cfg, sd1, sd2)
	}

	return nil
}

func buildSourceTableDiffs(ctx context.Context, cfg *Config, showRowCounts bool,
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

func buildSourceOverviewDiff(ctx context.Context, cfg *Config, sd1, sd2 *sourceData) (*sourceOverviewDiff, error) {
	var (
		body1, body2 string
		err          error
	)

	if body1, err = renderSourceMeta2YAML(sd1.srcMeta); err != nil {
		return nil, err
	}
	if body2, err = renderSourceMeta2YAML(sd2.srcMeta); err != nil {
		return nil, err
	}

	unified, err := computeUnified(ctx, "overview", sd1.handle, sd2.handle, cfg.Lines, body1, body2)
	if err != nil {
		return nil, err
	}

	diff := &sourceOverviewDiff{
		sd1:    sd1,
		sd2:    sd2,
		header: fmt.Sprintf("sq diff --overview %s %s", sd1.handle, sd2.handle),
		diff:   unified,
	}

	return diff, nil
}

func buildDBPropsDiff(ctx context.Context, cfg *Config, sd1, sd2 *sourceData) (*dbPropsDiff, error) {
	var (
		body1, body2 string
		err          error
	)

	if body1, err = renderDBProperties2YAML(sd1.srcMeta.DBProperties); err != nil {
		return nil, err
	}
	if body2, err = renderDBProperties2YAML(sd2.srcMeta.DBProperties); err != nil {
		return nil, err
	}

	unified, err := computeUnified(ctx, "dbprops", sd1.handle, sd2.handle, cfg.Lines, body1, body2)
	if err != nil {
		return nil, err
	}

	return &dbPropsDiff{
		sd1:    sd1,
		sd2:    sd2,
		header: fmt.Sprintf("sq diff --dbprops %s %s", sd1.handle, sd2.handle),
		diff:   unified,
	}, nil
}

func fetchSourceMeta(ctx context.Context, ru *run.Run, handle string) (*source.Source, *metadata.Source, error) {
	src, err := ru.Config.Collection.Get(handle)
	if err != nil {
		return nil, nil, err
	}
	grip, err := ru.Grips.Open(ctx, src)
	if err != nil {
		return nil, nil, err
	}
	md, err := grip.SourceMetadata(ctx, false)
	if err != nil {
		return nil, nil, err
	}

	return src, md, nil
}
