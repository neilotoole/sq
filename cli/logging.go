package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"golang.org/x/exp/slog"
)

// defaultLogging returns a log (and its associated closer) if
// logging has been enabled via envars.
func defaultLogging() (*slog.Logger, slog.Handler, *cleanup.Cleanup, error) {
	truncate, _ := strconv.ParseBool(os.Getenv(config.EnvarLogTruncate))

	logFilePath, ok := os.LookupEnv(config.EnvarLogPath)
	if !ok || logFilePath == "" || strings.TrimSpace(logFilePath) == "" {
		return lg.Discard(), nil, nil, nil
	}

	// Let's try to create the dir holding the logfile... if it already exists,
	// then os.MkdirAll will just no-op
	parent := filepath.Dir(logFilePath)
	err := os.MkdirAll(parent, 0o750)
	if err != nil {
		return lg.Discard(), nil, nil, errz.Wrapf(err, "failed to create parent dir of log file %s", logFilePath)
	}

	fileFlag := os.O_APPEND
	if truncate {
		fileFlag = os.O_TRUNC
	}

	logFile, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|fileFlag, 0o600)
	if err != nil {
		return lg.Discard(), nil, nil, errz.Wrapf(err, "unable to open log file: %s", logFilePath)
	}
	clnup := cleanup.New().AddE(logFile.Close)

	replace := func(groups []string, a slog.Attr) slog.Attr {
		// We want source to be "pkg/file.go".
		if a.Key == slog.SourceKey {
			fp := a.Value.String()
			a.Value = slog.StringValue(filepath.Join(filepath.Base(filepath.Dir(fp)), filepath.Base(fp)))
		}
		return a
	}

	h := slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: replace,
	}.NewJSONHandler(logFile)

	return slog.New(h), h, clnup, nil
}
