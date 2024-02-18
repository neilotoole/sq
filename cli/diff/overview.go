package diff

import (
	"context"
	"io"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/core/libdiff"
	"github.com/neilotoole/sq/libsq/core/progress"
)

func diffSourceOverview(ctx context.Context, cfg *Config, sd1, sd2 *sourceData, doc *libdiff.UnifiedDoc) {
	bar := progress.FromContext(ctx).NewWaiter("Diff overview", true, progress.OptMemUsage)
	defer bar.Stop()

	var body1, body2 string
	var err error
	defer func() { doc.Seal(err) }()

	g := &errgroup.Group{}
	g.Go(func() error {
		var gErr error
		body1, gErr = renderSourceMeta2YAML(sd1.srcMeta)
		return gErr
	})
	g.Go(func() error {
		var gErr error
		body2, gErr = renderSourceMeta2YAML(sd2.srcMeta)
		return gErr
	})
	if err = g.Wait(); err != nil {
		return
	}

	var unified string
	if unified, err = libdiff.ComputeUnified(ctx, sd1.handle, sd2.handle, cfg.Lines, body1, body2); err != nil {
		return
	}

	_, err = io.Copy(doc, libdiff.NewColorizer(cfg.Colors, strings.NewReader(unified)))
}
