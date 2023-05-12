package cli

// FIXME: remove nolint

import (
	"bytes"
	"context"
	"strings"

	"github.com/neilotoole/sq/cli/run"

	"github.com/neilotoole/sq/cli/output/diffw"

	"github.com/aymanbagabas/go-udiff"
	"github.com/aymanbagabas/go-udiff/myers"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/yamlw"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "diff @HANDLE1[.TABLE] @HANDLE[.TABLE]",
		Hidden: true, // hidden during development
		Short:  "Compare schemas or tables",
		Long:   `BETA: Compare two schemas, or tables. `,
		Args:   cobra.ExactArgs(2),
		ValidArgsFunction: (&handleTableCompleter{
			handleRequired: true,
			max:            2,
		}).complete,
		RunE: execDiff,
		Example: `  # Compare actor table in prod vs staging
  $ sq diff @prod/sakila.actor @staging/sakila.actor`,
	}

	// cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	// cmd.Flags().BoolP(flag.Compact, flag.CompactShort, false, flag.CompactUsage)
	// cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)
	return cmd
}

// execDiff compares schemas or tables.
func execDiff(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())

	handle1, table1, err := source.ParseTableHandle(args[0])
	if err != nil {
		return errz.Wrapf(err, "invalid input (1st arg): %s", args[0])
	}

	handle2, table2, err := source.ParseTableHandle(args[1])
	if err != nil {
		return errz.Wrapf(err, "invalid input (2nd arg): %s", args[1])
	}

	handle1 = strings.TrimSpace(handle1)
	table1 = strings.TrimSpace(table1)
	handle2 = strings.TrimSpace(handle2)
	table2 = strings.TrimSpace(table2)

	if table1 == "" || table2 == "" {
		return errz.Errorf("invalid input: TABLE value in @HANDLE.TABLE must not be empty")
	}

	return doTableDiff(cmd.Context(), ru, handle1, table1, handle2, table2)
}

func doTableDiff(ctx context.Context, ru *run.Run, handle1, table1, handle2, table2 string) error {
	_, coll := ru.Config, ru.Config.Collection

	src1, err := coll.Get(handle1)
	if err != nil {
		return err
	}

	src2, err := coll.Get(handle2)
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

	return printTableDiff(ctx, ru, src1, tblMeta1, src2, tblMeta2)
}

//nolint:gocritic
func printTableDiff(ctx context.Context, ru *run.Run,
	src1 *source.Source, tblMeta1 *source.TableMetadata,
	src2 *source.Source, tblMeta2 *source.TableMetadata,
) error {
	_ = ctx
	buf1, buf2 := &bytes.Buffer{}, &bytes.Buffer{}

	pr := output.NewPrinting()
	pr.EnableColor(false)

	w1 := yamlw.NewMetadataWriter(buf1, pr)
	w2 := yamlw.NewMetadataWriter(buf2, pr)

	if err := w1.TableMetadata(tblMeta1); err != nil {
		return err
	}
	if err := w2.TableMetadata(tblMeta2); err != nil {
		return err
	}

	body1 := buf1.String()
	body2 := buf2.String()

	edits := myers.ComputeEdits(body1, body2)
	unified, err := udiff.ToUnified(src1.Handle+"."+tblMeta1.Name, src2.Handle+"."+tblMeta2.Name, body1, edits)
	if err != nil {
		return errz.Err(err)
	}

	printCfg := diffw.NewConfig()
	// return diffw.PrintSG(ru.Out, printCfg, unified)
	return diffw.Print2(ru.Out, printCfg, unified)

	//fdr := diff.NewFileDiffReader(strings.NewReader(unified))
	//fdiff, err := fdr.Read()
	//if err != nil {
	//	return errz.Err(err)
	//}
	//
	//out, err := diff.PrintFileDiff(fdiff)
	//if err != nil {
	//	return errz.Err(err)
	//}
	//
	//_, err = fmt.Fprintln(ru.Out, string(out))
	//return errz.Err(err)

	// diff.NewMultiFileDiffReader(diffFile)

	// fmt.Fprintf(ru.Out, unified)

	//
	//
	//	const tpl = `========== %s.%s ==========
	//
	//%s
	//
	//`
	//
	//	fmt.Fprintf(ru.Out, tpl, src1.Handle, tblMeta1.Name, buf1.String())
	//	fmt.Fprintf(ru.Out, "\n\n")
	//	fmt.Fprintf(ru.Out, tpl, src2.Handle, tblMeta2.Name, buf2.String())
	//return nil
}
