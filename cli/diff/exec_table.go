package diff

import (
	"context"

	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/libsq/source"
)

// ExecTableDiff is the entrypoint to diff two tables, src1.table1 and
// src2.table2.
//
// Contrast with [ExecSourceDiff], which diffs two sources.
func ExecTableDiff(ctx context.Context, cfg *Config,
	src1 *source.Source, table1 string, src2 *source.Source, table2 string,
) error {
	cfg.init()
	var (
		ru      = cfg.Run
		elems   = cfg.Elements
		td1     = source.Table{Handle: src1.Handle, Name: table1}
		td2     = source.Table{Handle: src2.Handle, Name: table2}
		differs []*diffdoc.Differ
	)

	if elems.Schema {
		doc := diffdoc.NewUnifiedDoc(diffdoc.Titlef(cfg.Colors,
			"sq diff --schema %s %s", td1.String(), td2.String()))
		differs = append(differs, diffdoc.NewDiffer(doc, func(ctx context.Context, _ func(error)) {
			diffTableSchema(ctx, cfg, elems.RowCount, td1, td2, doc)
		}))
	}

	if elems.Data {
		differ := differForTableData(cfg, false, td1, td2)
		differs = append(differs, differ)
	}

	return diffdoc.Execute(ctx, ru.Out, cfg.Concurrency, differs)
}
