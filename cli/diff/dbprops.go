package diff

import (
	"context"
	"github.com/neilotoole/sq/libsq/core/libdiff"
	"github.com/neilotoole/sq/libsq/core/progress"
	"io"
	"strings"
)

// execDiffDBProps diffs the dbprops of sd1 and sd2, writing the diff to doc.
func execDiffDBProps(ctx context.Context, cfg *Config, sd1, sd2 *sourceData, doc *UnifiedDoc) {
	var err error
	defer func() { doc.Seal(err) }()

	var body1, body2 string

	if body1, err = renderDBProperties2YAML(sd1.srcMeta.DBProperties); err != nil {
		return
	}
	if body2, err = renderDBProperties2YAML(sd2.srcMeta.DBProperties); err != nil {
		return
	}

	bar := progress.FromContext(ctx).NewWaiter("Diff dbprops", true, progress.OptMemUsage)
	unified, err := libdiff.ComputeUnified(ctx, sd1.handle, sd2.handle, cfg.Lines, body1, body2)
	bar.Stop()
	if err != nil {
		return
	}

	_, err = io.Copy(doc, libdiff.NewColorizer(cfg.Colors, strings.NewReader(unified)))
}
