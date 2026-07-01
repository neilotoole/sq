package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/neilotoole/sq/cli/explore"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

func newExploreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "explore [@HANDLE|@HANDLE.TABLE|.TABLE]",
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: (&handleTableCompleter{
			max: 1,
		}).complete,
		RunE:  execExplore,
		Short: "Inspect data source schema interactively (TUI)",
		Long: `Inspect data source schema interactively. Opens a terminal UI
that browses the schema, columns, indexes, foreign keys, and a preview
of rows for a data source.

If @HANDLE is not provided, the active data source is used. If
@HANDLE.TABLE is provided, that table is focused on launch.

Quit with q. Use --emit-handle (-q) to write the last-focused handle
(e.g. "@src.actor") to stdout — handy with shell substitution:

  sq $(sq explore -q) --csv > rows.csv

Use --no-tui when stdout is a TTY but you want the source overview
text output instead of the interactive UI. The TUI also refuses to
start when stdout is not a TTY (e.g. piped to a file).`,
		Example: `  # Explore the active data source.
  $ sq explore

  # Explore @sakila, focused on the actor table.
  $ sq explore @sakila.actor

  # Print the source overview instead of opening the UI.
  $ sq explore --no-tui @sakila

  # Compose with the shell: query the table you navigated to.
  $ sq $(sq explore -q @sakila) --csv > rows.csv`,
	}

	cmd.Flags().BoolP(flag.ExploreEmitHandle, flag.ExploreEmitHandleShort, false, flag.ExploreEmitHandleUsage)
	cmd.Flags().Bool(flag.ExploreNoTUI, false, flag.ExploreNoTUIUsage)
	cmd.Flags().Int(flag.ExplorePreviewRows, 100, flag.ExplorePreviewRowsUsage)
	cmd.Flags().String(flag.ActiveSchema, "", flag.ActiveSchemaUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.ActiveSchema,
		activeSchemaCompleter{getActiveSourceViaArgs}.complete))
	return cmd
}

func execExplore(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	ru := run.FromContext(ctx)

	src, table, err := determineInspectTarget(ctx, ru, args)
	if err != nil {
		return err
	}

	if _, err = processFlagActiveSchema(cmd, src); err != nil {
		return err
	}
	if err = applySourceOptions(cmd, src); err != nil {
		return err
	}

	emit, _ := cmd.Flags().GetBool(flag.ExploreEmitHandle)
	noTUI, _ := cmd.Flags().GetBool(flag.ExploreNoTUI)
	previewRows, _ := cmd.Flags().GetInt(flag.ExplorePreviewRows)

	if noTUI || !isStdoutTTY(ru) {
		return runExploreNoTUI(ctx, ru, src, emit, table)
	}

	o, err := getOptionsFromCmd(cmd)
	if err != nil {
		return err
	}

	// Collection.Sources returns config-file (insertion) order; sort by
	// handle to match sq ls. The slice is a copy, safe to sort in place.
	sources := ru.Config.Collection.Sources()
	source.Sort(sources)

	cfg := explore.Config{
		Sources:      sources,
		FocusedSrc:   src,
		FocusedTable: table,
		EmitHandle:   emit,
		NoColor:      noColorFor(ru) || OptMonochrome.Get(o),
		PreviewRows:  previewRows,
	}

	// The TUI owns the terminal. Progress bars (rendered to stderr by
	// metadata fetches and ingest jobs) would scribble over bubbletea's
	// alt screen, so stop the progress widget and detach it from the
	// context for the TUI's lifetime. Progress consumers are nil-safe.
	if prog := progress.FromContext(ctx); prog != nil {
		prog.Stop()
		ctx = progress.NewContext(ctx, nil)
	}

	finalHandle, err := explore.Run(ctx, ru, cfg)
	if err != nil {
		return errz.Wrapf(err, "explore failed")
	}
	if emit && finalHandle != "" {
		fmt.Fprintln(ru.Out, finalHandle)
	}
	return nil
}

// runExploreNoTUI delegates to the inspect overview path when the TUI
// can't or shouldn't run. It honors --emit-handle by also writing the
// initial address.
func runExploreNoTUI(ctx context.Context, ru *run.Run, src *source.Source, emit bool, table string) error {
	grip, err := ru.Grips.Open(ctx, src, driver.ModeReadOnly)
	if err != nil {
		return errz.Wrapf(err, "failed to open %s", src.Handle)
	}
	srcMeta, err := grip.SourceMetadata(ctx, true)
	if err != nil {
		return errz.Wrapf(err, "failed to read %s source metadata", src.Handle)
	}
	if err = ru.Writers.Metadata.SourceMetadata(srcMeta, false); err != nil {
		return err
	}
	if emit {
		addr := src.Handle
		if table != "" {
			addr = src.Handle + "." + table
		}
		fmt.Fprintln(ru.Out, addr)
	}
	return nil
}

// isStdoutTTY returns true when the original stdout is a terminal. It
// checks ru.Stdout (the underlying fd), not the decorated ru.Out: in
// tests ru.Out is a buffer while os.Stdout may still be a TTY, so a
// fallback to os.Stdout would wrongly try to start the TUI.
func isStdoutTTY(ru *run.Run) bool {
	type fd interface{ Fd() uintptr }
	if f, ok := ru.Stdout.(fd); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

// noColorFor reports whether color is disabled via NO_COLOR
// (https://no-color.org). The caller also honors the merged
// --monochrome / -M option (see execExplore).
func noColorFor(ru *run.Run) bool {
	_ = ru
	return os.Getenv("NO_COLOR") != ""
}
