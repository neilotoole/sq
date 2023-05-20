package diff

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"golang.org/x/exp/slices"

	"github.com/neilotoole/sq/cli/diff/internal/go-udiff"
	"github.com/neilotoole/sq/cli/diff/internal/go-udiff/myers"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
	"golang.org/x/sync/errgroup"
)

type Options struct {
	Summary      bool
	DBProperties bool
	Tables       bool
	RowCount     bool
	Data         bool
}

// ExecSourceDiff diffs handle1 and handle2. If diffProps is true, the
// source database properties are diffed. If diffTables is true, the
// individual tables are also diffed.
func ExecSourceDiff(ctx context.Context, ru *run.Run, numLines int,
	opts *Options, handle1, handle2 string,
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

	if opts.Summary {
		srcDiff, err := buildSourceSummaryDiff(numLines, sd1, sd2)
		if err != nil {
			return err
		}

		if err = Print(ru.Out, ru.Writers.Printing, srcDiff.header, srcDiff.diff); err != nil {
			return err
		}
	}

	if opts.DBProperties {
		propsDiff, err := buildDBPropsDiff(numLines, sd1, sd2)
		if err != nil {
			return err
		}
		if err = Print(ru.Out, ru.Writers.Printing, propsDiff.header, propsDiff.diff); err != nil {
			return err
		}
	}

	if opts.Tables {
		tblDiffs, err := buildSourceTableDiffs(numLines, opts.RowCount, sd1, sd2)
		if err != nil {
			return err
		}
		for _, tblDiff := range tblDiffs {
			if err := Print(ru.Out, ru.Writers.Printing, tblDiff.header, tblDiff.diff); err != nil {
				return err
			}
		}
	}

	return nil
}

func buildSourceTableDiffs(numLines int, showRowCounts bool, sd1, sd2 *sourceData) ([]*tableDiff, error) {
	var allTblNames []string

	for _, tbl := range sd1.srcMeta.Tables {
		allTblNames = append(allTblNames, tbl.Name)
	}
	for _, tbl := range sd2.srcMeta.Tables {
		allTblNames = append(allTblNames, tbl.Name)
	}
	allTblNames = lo.Uniq(allTblNames)
	slices.Sort(allTblNames)

	var diffs []*tableDiff
	for _, tblName := range allTblNames {
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

		dff, err := buildTableDiff(numLines, showRowCounts, td1, td2)
		if err != nil {
			return nil, err
		}

		diffs = append(diffs, dff)
	}

	return diffs, nil
}

func buildSourceSummaryDiff(numLines int, sd1, sd2 *sourceData) (*sourceDiff, error) {
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

	edits := myers.ComputeEdits(body1, body2)
	unified, err := udiff.ToUnified(
		sd1.handle,
		sd2.handle,
		body1,
		edits,
		numLines,
	)
	if err != nil {
		return nil, errz.Err(err)
	}

	diff := &sourceDiff{
		sd1:    sd1,
		sd2:    sd2,
		header: fmt.Sprintf("sq diff --summary %s %s", sd1.handle, sd2.handle),
		diff:   unified,
	}

	return diff, nil
}

func buildDBPropsDiff(numLines int, sd1, sd2 *sourceData) (*dbPropsDiff, error) {
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

	edits := myers.ComputeEdits(body1, body2)
	unified, err := udiff.ToUnified(
		sd1.handle,
		sd2.handle,
		body1,
		edits,
		numLines,
	)
	if err != nil {
		return nil, errz.Err(err)
	}

	diff := &dbPropsDiff{
		sd1:    sd1,
		sd2:    sd2,
		header: fmt.Sprintf("sq diff --dbprops %s %s", sd1.handle, sd2.handle),
		diff:   unified,
	}

	return diff, nil
}

func sortTables(tbls []*source.TableMetadata) { //nolint:unused
	slices.SortFunc(tbls, func(a, b *source.TableMetadata) bool {
		return a.FQName < b.FQName
	})
}

func fetchSourceMeta(ctx context.Context, ru *run.Run, handle string) (*source.Source, *source.Metadata, error) {
	src, err := ru.Config.Collection.Get(handle)
	if err != nil {
		return nil, nil, err
	}
	dbase, err := ru.Databases.Open(ctx, src)
	if err != nil {
		return nil, nil, err
	}
	md, err := dbase.SourceMetadata(ctx)
	if err != nil {
		return nil, nil, err
	}

	return src, md, nil
}
