package diff

import (
	"context"
	"io"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/libsq/core/progress"
)

// diffDBProps diffs the dbprops of sd1 and sd2, writing the diff to doc.
func diffDBProps(ctx context.Context, cfg *Config, sd1, sd2 *sourceData, doc *diffdoc.UnifiedDoc) {
	bar := progress.FromContext(ctx).NewWaiter("Diff dbprops", true, progress.OptMemUsage)
	defer bar.Stop()

	var body1, body2 string
	var err error
	defer func() { doc.Seal(err) }()

	g := &errgroup.Group{}
	g.Go(func() error {
		var gErr error
		body1, gErr = renderDBProperties2YAML(sd1.srcMeta.DBProperties)
		return gErr
	})
	g.Go(func() error {
		var gErr error
		body2, gErr = renderDBProperties2YAML(sd2.srcMeta.DBProperties)
		return gErr
	})
	if err = g.Wait(); err != nil {
		return
	}

	var unified string
	if unified, err = diffdoc.ComputeUnified(ctx, sd1.handle, sd2.handle, cfg.Lines, body1, body2); err != nil {
		return
	}

	_, err = io.Copy(doc, diffdoc.NewColorizer(cfg.Colors, strings.NewReader(unified)))
}
