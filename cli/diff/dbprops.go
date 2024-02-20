package diff

import (
	"context"
	"io"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/source"
)

// diffDBProps diffs the dbprops of sd1 and sd2, writing the diff to doc.
func diffDBProps(ctx context.Context, cfg *Config, src1, src2 *source.Source, doc *diffdoc.UnifiedDoc) {
	bar := progress.FromContext(ctx).NewWaiter("Diff dbprops", true, progress.OptMemUsage)
	defer bar.Stop()

	var body1, body2 string
	var err error
	defer func() { doc.Seal(err) }()

	md1, md2, err := cfg.Run.MDCache.SourceMetaPair(ctx, src1, src2)
	if err != nil {
		return
	}

	// FIXME: we need to optimize for just the DBProperties, we don't need
	// the entire thing.

	g := &errgroup.Group{}
	g.Go(func() error {
		var gErr error
		body1, gErr = renderDBProperties2YAML(md1.DBProperties)
		return gErr
	})
	g.Go(func() error {
		var gErr error
		body2, gErr = renderDBProperties2YAML(md2.DBProperties)
		return gErr
	})
	if err = g.Wait(); err != nil {
		return
	}

	var unified string
	if unified, err = diffdoc.ComputeUnified(ctx, src1.Handle, src2.Handle, cfg.Lines, body1, body2); err != nil {
		return
	}

	_, err = io.Copy(doc, diffdoc.NewColorizer(cfg.Colors, strings.NewReader(unified)))
}
