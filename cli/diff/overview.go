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

func diffOverview(ctx context.Context, cfg *Config, src1, src2 *source.Source, doc *diffdoc.UnifiedDoc) {
	bar := progress.FromContext(ctx).NewWaiter("Diff overview", true, progress.OptMemUsage)
	defer bar.Stop()

	var body1, body2 string
	var err error
	defer func() { doc.Seal(err) }()

	md1, md2, err := cfg.Run.MDCache.SourceMetaPair(ctx, src1, src2)
	if err != nil {
		return
	}

	g := &errgroup.Group{}
	g.Go(func() error {
		var gErr error
		body1, gErr = renderSourceMeta2YAML(md1)
		return gErr
	})
	g.Go(func() error {
		var gErr error
		body2, gErr = renderSourceMeta2YAML(md2)
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
