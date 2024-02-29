package diff

import (
	"context"

	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/libsq/source"
)

// ExecSourceDiff is the entrypoint to diff two sources, handle1 and handle2.
// If differences are found, hasDiffs returns true.
//
// Contrast with [ExecTableDiff], which diffs two tables.
func ExecSourceDiff(ctx context.Context, cfg *Config, src1, src2 *source.Source) (hasDiffs bool, err error) {
	modes := cfg.Modes

	var differs []*diffdoc.Differ

	if modes.Overview {
		doc := diffdoc.NewUnifiedDoc(diffdoc.Titlef(cfg.Colors,
			"sq diff --overview %s %s", src1.Handle, src2.Handle))
		differs = append(differs, diffdoc.NewDiffer(doc, func(ctx context.Context, _ func(error)) {
			diffOverview(ctx, cfg, src1, src2, doc)
		}))
	}

	if modes.DBProperties {
		doc := diffdoc.NewUnifiedDoc(diffdoc.Titlef(cfg.Colors,
			"sq diff --dbprops %s %s", src1.Handle, src2.Handle))
		differs = append(differs, diffdoc.NewDiffer(doc, func(ctx context.Context, _ func(error)) {
			diffDBProps(ctx, cfg, src1, src2, doc)
		}))
	}

	if modes.Schema {
		schemaDiffers, err := differsForSchema(ctx, cfg, modes.RowCount, src1, src2)
		if err != nil {
			return hasDiffs, err
		}
		differs = append(differs, schemaDiffers...)
	}

	if modes.Data {
		// We're going for it... diff all table data.
		dataDiffers, err := differsForAllTableData(ctx, cfg, src1, src2)
		if err != nil {
			return hasDiffs, err
		}
		differs = append(differs, dataDiffers...)
	}

	return diffdoc.Execute(ctx, cfg.Run.Out, cfg.Concurrency, differs)
}
