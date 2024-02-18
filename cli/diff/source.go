package diff

import (
	"context"
	"io"
	"strings"

	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/libdiff"
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

		// TODO: I think in some places we need just the table names, so we should
		// be able to call SQLDriver.ListTableNames instead of getting the entire
		// metadata.Source.
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
		doc := libdiff.NewUnifiedDoc(libdiff.Titlef(cfg.Colors,
			"sq diff --overview %s %s", sd1.handle, sd2.handle))

		diffSourceOverview(ctx, cfg, sd1, sd2, doc)
		if err := doc.Err(); err != nil {
			return err
		}

		_, err := errz.Return(io.Copy(ru.Out, contextio.NewReader(ctx, doc)))
		lg.WarnIfCloseError(lg.FromContext(ctx), lgm.CloseDiffDoc, doc)
		if err != nil {
			return err
		}
	}

	if elems.DBProperties {
		doc := libdiff.NewUnifiedDoc(libdiff.Titlef(cfg.Colors,
			"sq diff --dbprops %s %s", sd1.handle, sd2.handle))

		diffDBProps(ctx, cfg, sd1, sd2, doc)
		if err := doc.Err(); err != nil {
			return err
		}

		_, err := errz.Return(io.Copy(ru.Out, contextio.NewReader(ctx, doc)))
		lg.WarnIfCloseError(lg.FromContext(ctx), lgm.CloseDiffDoc, doc)
		if err != nil {
			return err
		}
	}

	if elems.Schema {
		tblDiffs, err := buildSourceTableStructureDiffs(ctx, cfg, elems.RowCount, sd1, sd2)
		if err != nil {
			return err
		}
		for _, tblDiff := range tblDiffs {
			if err = Print(
				ctx,
				ru.Out,
				cfg.Colors,
				tblDiff.header,
				strings.NewReader(tblDiff.diff),
			); err != nil {
				return err
			}
		}
	}

	if elems.Data {
		// We're going for it... diff all table data.
		return execDiffSourceTablesData(ctx, ru, cfg, sd1, sd2)
	}

	return nil
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
