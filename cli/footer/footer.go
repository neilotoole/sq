// Package footer renders interactive stderr notices after successful commands.
package footer

import (
	"context"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/cobraz"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/termz"
)

const envNoUpdateNotifier = "SQ_NO_UPDATE_NOTIFIER"

// Notice is one interactive stderr footer line (or block).
type Notice interface {
	// Eligible returns whether this notice applies to the finished command.
	Eligible(ctx context.Context, ru *run.Run, err error) bool
	// Write renders the notice to w. width is the stderr terminal width in
	// columns (0 = unknown; implementors may skip right-align).
	Write(w io.Writer, pr *output.Printing, ru *run.Run, width int) error
}

var notices []Notice

func init() { //nolint:gochecknoinits
	Register(updateNotice{})
}

// Register adds a footer notice. Typically called from init().
func Register(n Notice) {
	notices = append(notices, n)
}

// Render writes eligible notices to stderr when the session is interactive.
func Render(ctx context.Context, ru *run.Run, err error) {
	if ru == nil || !interactiveSession(ru, err) {
		return
	}

	w, pr := stderrWriterAndPrinting(ru)
	if w == nil {
		return
	}

	width := terminalWidthFromWriter(ru.Stderr)
	wrote := false
	for _, n := range notices {
		if !n.Eligible(ctx, ru, err) {
			continue
		}
		if !wrote && termz.IsTerminal(ru.Stderr) {
			_, _ = io.WriteString(w, "\n")
			wrote = true
		}
		if writeErr := n.Write(w, pr, ru, width); writeErr != nil {
			return
		}
	}
}

func stderrWriterAndPrinting(ru *run.Run) (io.Writer, *output.Printing) {
	if ru.ErrOut != nil && ru.Writers != nil && ru.Writers.PrErr != nil {
		return ru.ErrOut, ru.Writers.PrErr
	}
	if ru.Stderr != nil {
		pr := output.NewPrinting()
		if !termz.IsColorTerminal(ru.Stderr) {
			pr.EnableColor(false)
		}
		return ru.Stderr, pr
	}
	return nil, nil
}

func interactiveSession(ru *run.Run, err error) bool {
	if footerSuppressingError(err) {
		return false
	}
	if os.Getenv(envNoUpdateNotifier) == "1" {
		return false
	}
	if !termz.IsTerminal(ru.Stdout) || !termz.IsTerminal(ru.Stderr) {
		return false
	}
	if cmdFlagChanged(ru.Cmd, flag.FileOutput) {
		return false
	}
	if shellCompletionInvocation(ru) {
		return false
	}
	return true
}

func shellCompletionInvocation(ru *run.Run) bool {
	if len(ru.Args) > 0 {
		switch ru.Args[0] {
		case cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd, cobraz.ShellCompGenScriptsCmd:
			return true
		}
	}
	if ru.Cmd != nil && ru.Cmd.Name() == cobraz.ShellCompGenScriptsCmd {
		return true
	}
	return false
}

// footerSuppressingError reports whether err should block interactive footers.
func footerSuppressingError(err error) bool {
	if err == nil {
		return false
	}
	return !errz.IsUsageError(err)
}

func cmdFlagChanged(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}
	f := cmd.Flag(name)
	if f == nil {
		return false
	}
	return f.Changed
}
