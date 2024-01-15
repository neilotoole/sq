// Package run holds the run.Run construct, which encapsulates CLI state
// for a command execution.
package run

import (
	"context"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

type runKey struct{}

// NewContext returns ctx with ru added as a value.
func NewContext(ctx context.Context, ru *Run) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, runKey{}, ru)
}

// FromContext extracts the Run added to ctx via NewContext.
func FromContext(ctx context.Context) *Run {
	return ctx.Value(runKey{}).(*Run)
}

// Run is a container for injectable resources passed
// to all cobra exec funcs. The Close method should be invoked when
// the Run is no longer needed.
type Run struct {
	// Stdin typically is os.Stdin, but can be changed for testing.
	Stdin *os.File

	// Out is the output destination, typically os.Stdout.
	Out io.Writer

	// ErrOut is the error output destination, typically os.Stderr.
	ErrOut io.Writer

	// Cmd is the command instance provided by cobra for
	// the currently executing command. This field will
	// be set before the command's runFunc is invoked.
	Cmd *cobra.Command

	// Args is the arg slice supplied by cobra for
	// the currently executing command. This field will
	// be set before the command's runFunc is invoked.
	Args []string

	// Config is the run's config.
	Config *config.Config

	// ConfigStore manages config persistence.
	ConfigStore config.Store

	// OptionsRegistry is a registry of CLI options.Opt instances.
	OptionsRegistry *options.Registry

	// DriverRegistry is a registry of driver implementations.
	DriverRegistry *driver.Registry

	// Files manages file access.
	Files *source.Files

	// Grips mediates access to driver.Grip instances.
	Grips *driver.Grips

	// Writers holds the various writer types that
	// the CLI uses to print output.
	Writers *output.Writers

	// Cleanup holds cleanup functions, except log closing, which
	// is held by LogCloser.
	Cleanup *cleanup.Cleanup

	// LogCloser contains any log-closing action (such as closing
	// a log file). It may be nil. Execution of this function
	// should be more-or-less the final cleanup action performed by the CLI,
	// and absolutely must happen after all other cleanup actions.
	LogCloser func() error
}

// Close should be invoked to dispose of any open resources
// held by ru. If an error occurs during Close and ru.Log
// is not nil, that error is logged at WARN level before
// being returned. Note that Run.LogCloser must be invoked separately.
func (ru *Run) Close() error {
	if ru == nil {
		return nil
	}

	if ru.Cmd != nil {
		lg.FromContext(ru.Cmd.Context()).Debug("Closing run")
	}

	return errz.Wrap(ru.Cleanup.Run(), "close run")
}

// NewQueryContext returns a *libsq.QueryContext constructed from ru.
func NewQueryContext(ru *Run, args map[string]string) *libsq.QueryContext {
	return &libsq.QueryContext{
		Collection: ru.Config.Collection,
		Grips:      ru.Grips,
		Args:       args,
	}
}
