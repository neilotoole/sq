package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slog"
)

// defaultLogging returns a *slog.Logger, its slog.Handler, and
// possibly a *cleanup.Cleanup, which the caller is responsible
// for invoking at the appropriate time. If an error is returned, the
// other returned values will be nil. If logging is not enabled,
// all returned values will be nil.
func defaultLogging(ctx context.Context) (log *slog.Logger, h slog.Handler, closer func() error, err error) {
	logFilePath, ok := os.LookupEnv(config.EnvarLogPath)
	if !ok || strings.TrimSpace(logFilePath) == "" {
		lg.FromContext(ctx).Debug("Logging: not enabled via envar", lga.Key, config.EnvarLogPath)
		return lg.Discard(), nil, nil, nil
	}

	lg.FromContext(ctx).Debug("Logging: enabled via envar", lga.Key, config.EnvarLogPath, lga.Val, logFilePath)

	// Let's try to create the dir holding the logfile... if it already exists,
	// then os.MkdirAll will just no-op
	parent := filepath.Dir(logFilePath)
	err = os.MkdirAll(parent, 0o750)
	if err != nil {
		return nil, nil, nil, errz.Wrapf(err, "logging: failed to create parent dir of log file %s", logFilePath)
	}

	logFile, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, nil, errz.Wrapf(err, "logging: unable to open log file: %s", logFilePath)
	}
	closer = logFile.Close

	h = newJSONHandler(logFile)
	return slog.New(h), h, closer, nil
}

func stderrLogger() (*slog.Logger, slog.Handler) {
	h := newJSONHandler(os.Stderr)
	return slog.New(h), h
}

func newJSONHandler(w io.Writer) slog.Handler {
	return slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: slogReplaceAttrs,
	}.NewJSONHandler(w)
}

func slogReplaceAttrs(groups []string, a slog.Attr) slog.Attr {
	a = slogReplaceSource(groups, a)
	a = slogReplaceDuration(groups, a)
	return a
}

// slogReplaceSource overrides the default slog.SourceKey attr
// to print "pkg/file.go" instead.
func slogReplaceSource(_ []string, a slog.Attr) slog.Attr {
	// We want source to be "pkg/file.go".
	if a.Key == slog.SourceKey {
		fp := a.Value.String()
		a.Value = slog.StringValue(filepath.Join(filepath.Base(filepath.Dir(fp)), filepath.Base(fp)))
	}
	return a
}

// slogReplaceDuration prints the friendly version of duration.
func slogReplaceDuration(_ []string, a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindDuration {
		a.Value = slog.StringValue(a.Value.Duration().String())
	}
	return a
}

// logFrom is a convenience function for getting a *slog.Logger from a
// *cobra.Command or context.Context.
// If no logger present, lg.Discard() is returned.
func logFrom(cmd *cobra.Command) *slog.Logger {
	if cmd == nil {
		return lg.Discard()
	}

	ctx := cmd.Context()
	if ctx == nil {
		return lg.Discard()
	}

	log := lg.FromContext(ctx)
	if log == nil {
		return lg.Discard()
	}

	return log
}
