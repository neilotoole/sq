//nolint:dupl
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

// diffDBProps diffs the dbprops of src1 and src2, writing the diff to doc.
func diffDBProps(ctx context.Context, cfg *Config, src1, src2 *source.Source, doc *diffdoc.UnifiedDoc) {
	bar := progress.FromContext(ctx).NewWaiter("Diff dbprops", progress.OptMemUsage)
	defer bar.Stop()

	var body1, body2 string
	var err error
	defer func() { doc.Seal(err) }()

	dbp1, dbp2, err := cfg.Run.MDCache.DBPropertiesPair(ctx, src1, src2)
	if err != nil {
		return
	}

	g := &errgroup.Group{}
	g.Go(func() error {
		var gErr error
		body1, gErr = renderDBProperties2YAML(dbp1)
		return gErr
	})
	g.Go(func() error {
		var gErr error
		body2, gErr = renderDBProperties2YAML(dbp2)
		return gErr
	})
	if err = g.Wait(); err != nil {
		return
	}

	var unified string
	if unified, err = diffdoc.ComputeUnified(ctx, src1.Handle, src2.Handle, cfg.Lines, body1, body2); err != nil {
		return
	}

	_, err = io.Copy(doc, diffdoc.NewColorizer(ctx, cfg.Colors, strings.NewReader(unified)))
}
