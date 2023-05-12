package diff

import (
	"bytes"
	"context"
	"fmt"

	"github.com/aymanbagabas/go-udiff"
	"github.com/aymanbagabas/go-udiff/myers"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/yamlw"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
)

// ExecTableDiff diffs handle1.table1 and handle2.table2.
func ExecTableDiff(ctx context.Context, ru *run.Run, handle1, table1, handle2, table2 string) error {
	src1, err := ru.Config.Collection.Get(handle1)
	if err != nil {
		return err
	}

	src2, err := ru.Config.Collection.Get(handle2)
	if err != nil {
		return err
	}

	dbase1, err := ru.Databases.Open(ctx, src1)
	if err != nil {
		return err
	}
	dbase2, err := ru.Databases.Open(ctx, src2)
	if err != nil {
		return err
	}

	tblMeta1, err := dbase1.TableMetadata(ctx, table1)
	if err != nil {
		return err
	}

	tblMeta2, err := dbase2.TableMetadata(ctx, table2)
	if err != nil {
		return err
	}

	diffText, err := buildTableDiff(src1, tblMeta1, src2, tblMeta2)
	if err != nil {
		return err
	}

	if ru.Writers.Printing.IsMonochrome() {
		_, err := fmt.Fprintln(ru.Out, diffText)
		return errz.Err(err)
	}

	return doPrintColor(ru.Out, ru.Writers.Printing, diffText)
}

func buildTableDiff(src1 *source.Source, tblMeta1 *source.TableMetadata,
	src2 *source.Source, tblMeta2 *source.TableMetadata,
) (string, error) {
	pr := output.NewPrinting()
	// We want monochrome yaml; any colorization happens to the diff text
	// after it's computed.
	pr.EnableColor(false)

	buf1, buf2 := &bytes.Buffer{}, &bytes.Buffer{}
	w1 := yamlw.NewMetadataWriter(buf1, pr)
	w2 := yamlw.NewMetadataWriter(buf2, pr)

	if err := w1.TableMetadata(tblMeta1); err != nil {
		return "", err
	}
	if err := w2.TableMetadata(tblMeta2); err != nil {
		return "", err
	}

	body1 := buf1.String()
	body2 := buf2.String()

	edits := myers.ComputeEdits(body1, body2)
	unified, err := udiff.ToUnified(
		src1.Handle+"."+tblMeta1.Name,
		src2.Handle+"."+tblMeta2.Name,
		body1,
		edits,
	)
	if err != nil {
		return "", errz.Err(err)
	}

	return unified, nil
}
