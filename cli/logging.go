package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/output/format"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/devlog"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/userlogdir"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

var (
	OptLogEnabled = options.NewBool(
		"log",
		nil,
		false,
		"Enable logging",
		"Enable logging.",
	)

	OptLogFile = options.NewString(
		"log.file",
		nil,
		getDefaultLogFilePath(),
		nil,
		"Log file path",
		`Path to log file. Empty value disables logging.`,
	)

	OptLogLevel = NewLogLevelOpt(
		"log.level",
		slog.LevelDebug,
		`Log level, one of: DEBUG, INFO, WARN, ERROR`,
		"Log level, one of: DEBUG, INFO, WARN, ERROR.",
	)

	OptLogFormat = format.NewOpt(
		"log.format",
		nil,
		format.Text,
		func(f format.Format) error {
			if f == format.Text || f == format.JSON {
				return nil
			}

			return errz.Errorf("option {log.format} allows only %q or %q", format.Text, format.JSON)
		},
		"Log output format (text or json)",
		fmt.Sprintf(
			`Log output format. Allowed formats are %q (human-friendly) or %q.`, format.Text, format.JSON,
		),
	)
)

// defaultLogging returns a *slog.Logger, its slog.Handler, and
// possibly a *cleanup.Cleanup, which the caller is responsible
// for invoking at the appropriate time. If an error is returned, the
// other returned values will be nil. If logging is not enabled,
// the returned values will also be nil.
func defaultLogging(ctx context.Context, osArgs []string, cfg *config.Config,
) (log *slog.Logger, h slog.Handler, closer func() error, err error) {
	bootLog := lg.FromContext(ctx)

	enabled := getLogEnabled(ctx, osArgs, cfg)
	if !enabled {
		return nil, nil, nil, nil
	}

	// First, get the log file path. It can come from flag, envar, or config.
	logFilePath := strings.TrimSpace(getLogFilePath(ctx, osArgs, cfg))
	if logFilePath == "" {
		bootLog.Debug("Logging: not enabled (log file path not set)")
		return nil, nil, nil, nil
	}

	lvl := getLogLevel(ctx, osArgs, cfg)

	// Allow for $HOME/sq.log etc.
	logFilePath = os.ExpandEnv(logFilePath)
	bootLog.Debug("Logging: enabled", lga.Path, logFilePath)

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

	// Determine if we're logging in dev mode (format.Text).
	devMode := OptLogFormat.Default() != format.JSON
	switch getLogFormat(ctx, osArgs, cfg) { //nolint:exhaustive
	case format.Text:
		devMode = true
	case format.JSON:
		devMode = false
	default:
		// Shouldn't happen
	}

	if devMode {
		h = devlog.NewHandler(logFile, lvl)
	} else {
		h = newJSONHandler(logFile, lvl)
	}

	return slog.New(h), h, closer, nil
}

func stderrLogger() (*slog.Logger, slog.Handler) {
	h := newJSONHandler(os.Stderr, slog.LevelDebug)
	return slog.New(h), h
}

func newJSONHandler(w io.Writer, lvl slog.Leveler) slog.Handler {
	h := &slog.HandlerOptions{
		AddSource:   true,
		Level:       lvl,
		ReplaceAttr: slogReplaceAttrs,
	}

	return slog.NewJSONHandler(w, h)
}

func slogReplaceAttrs(groups []string, a slog.Attr) slog.Attr {
	a = slogReplaceSource(groups, a)
	a = slogReplaceDuration(groups, a)
	a = slogReplaceHTTPResponse(groups, a)
	return a
}

// slogReplaceSource overrides the default slog.SourceKey attr
// to print "pkg/file.go" instead.
func slogReplaceSource(_ []string, a slog.Attr) slog.Attr {
	// We want source to be "pkg/file.go:42".
	if a.Key == slog.SourceKey {
		source, ok := a.Value.Any().(*slog.Source)
		if ok && source != nil {
			val := filepath.Join(filepath.Base(filepath.Dir(source.File)), filepath.Base(source.File))
			val += ":" + strconv.Itoa(source.Line)
			a.Value = slog.StringValue(val)
		}
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

// slogReplaceDuration prints the friendly version of duration.
func slogReplaceHTTPResponse(_ []string, a slog.Attr) slog.Attr {
	resp, ok := a.Value.Any().(*http.Response)
	if !ok {
		return a
	}

	v := httpz.ResponseLogValue(resp)
	a.Value = v
	return a
}

// getLogEnabled determines if logging is enabled based on flags, envars, or config.
// Any error is logged to the ctx logger.
func getLogEnabled(ctx context.Context, osArgs []string, cfg *config.Config) bool {
	bootLog := lg.FromContext(ctx)
	var enabled bool

	flg := OptLogEnabled.Flag().Name
	val, ok, err := getBootstrapFlagValue(flg, "", OptLogEnabled.Usage(), osArgs)
	if err != nil {
		bootLog.Warn("Reading log 'enabled' from flag", lga.Flag, flg, lga.Err, err)
	}
	if ok {
		bootLog.Debug("Using log 'enabled' specified via flag", lga.Flag, flg, lga.Val, val)

		enabled, err = stringz.ParseBool(val)
		if err != nil {
			bootLog.Error(
				"Reading bool flag",
				lga.Flag, flg,
				lga.Val, val,
			)
			// When in doubt, enable logging?
			return true
		}

		return enabled
	}

	val, ok = os.LookupEnv(config.EnvarLogEnabled)
	if ok {
		bootLog.Debug(
			"Using log 'enabled' specified via envar",
			lga.Env, config.EnvarLogEnabled,
			lga.Val, val,
		)

		enabled, err = stringz.ParseBool(val)
		if err != nil {
			bootLog.Error(
				"Reading bool envar",
				lga.Env, config.EnvarLogEnabled,
				lga.Val, val,
			)

			// When in doubt, enable logging?
			return true
		}

		return enabled
	}

	var o options.Options
	if cfg != nil {
		o = cfg.Options
	}

	enabled = OptLogEnabled.Get(o)
	bootLog.Debug("Using log 'enabled' specified via config", lga.Key, OptLogEnabled.Key(), lga.Val, enabled)
	return enabled
}

// getLogLevel gets the log level, based on flags, envars, or config.
// Any error is logged to the ctx logger.
func getLogLevel(ctx context.Context, osArgs []string, cfg *config.Config) slog.Level {
	bootLog := lg.FromContext(ctx)

	flg := OptLogLevel.Flag().Name
	val, ok, err := getBootstrapFlagValue(flg, "", OptLogLevel.Usage(), osArgs)
	if err != nil {
		bootLog.Warn("Reading log level from flag", lga.Flag, flg, lga.Err, err)
	}
	if ok {
		bootLog.Debug("Using log level specified via flag", lga.Flag, flg, lga.Val, val)

		lvl := new(slog.Level)
		if err = lvl.UnmarshalText([]byte(val)); err == nil {
			return *lvl
		}
		bootLog.Error("Invalid log level specified via flag",
			lga.Flag, flg, lga.Val, val, lga.Err, err)
	}

	val, ok = os.LookupEnv(config.EnvarLogLevel)
	if ok {
		bootLog.Debug("Using log level specified via envar",
			lga.Env, config.EnvarLogLevel, lga.Val, val)

		lvl := new(slog.Level)
		if err = lvl.UnmarshalText([]byte(val)); err != nil {
			return *lvl
		}
		bootLog.Error("Invalid log level specified by envar",
			lga.Env, config.EnvarLogLevel, lga.Val, val, lga.Err, err)
	}

	var o options.Options
	if cfg != nil {
		o = cfg.Options
	}

	lvl := OptLogLevel.Get(o)
	bootLog.Debug("Using log level specified via config", lga.Key, OptLogLevel.Key(), lga.Val, lvl)
	return lvl
}

// getLogEnabled gets the log format based on flags, envars, or config.
// Any error is logged to the ctx logger. The returned value is guaranteed
// to be one of format.Text or format.JSON.
func getLogFormat(ctx context.Context, osArgs []string, cfg *config.Config) format.Format {
	bootLog := lg.FromContext(ctx)

	flg := OptLogFormat.Flag().Name
	val, ok, err := getBootstrapFlagValue(flg, "", OptLogFormat.Usage(), osArgs)
	if err != nil {
		bootLog.Warn("Error reading log format from flag", lga.Flag, flg, lga.Err, err)
	}
	if ok {
		bootLog.Debug("Using log format specified via flag", lga.Flag, flg, lga.Val, val)

		f := new(format.Format)
		if err = f.UnmarshalText([]byte(val)); err == nil {
			switch *f { //nolint:exhaustive
			case format.Text, format.JSON:
				return *f
			default:
			}
		}
		bootLog.Error("Invalid log format specified via flag",
			lga.Flag, flg, lga.Val, val, lga.Err, err)
	}

	val, ok = os.LookupEnv(config.EnvarLogFormat)
	if ok {
		bootLog.Debug("Using log level specified via envar",
			lga.Env, config.EnvarLogFormat, lga.Val, val)

		f := new(format.Format)
		if err = f.UnmarshalText([]byte(val)); err == nil {
			switch *f { //nolint:exhaustive
			case format.Text, format.JSON:
				return *f
			default:
			}
		}
		bootLog.Error("Invalid log format specified by envar",
			lga.Env, config.EnvarLogLevel, lga.Val, val, lga.Err, err)
	}

	var o options.Options
	if cfg != nil {
		o = cfg.Options
	}

	f := OptLogFormat.Get(o)
	bootLog.Debug("Using log format specified via config", lga.Key, OptLogFormat.Key(), lga.Val, f)
	return f
}

// getLogFilePath gets the log file path, based on flags, envars, or config.
// If a log file is not specified (and thus logging is disabled), empty string
// is returned.
func getLogFilePath(ctx context.Context, osArgs []string, cfg *config.Config) string {
	bootLog := lg.FromContext(ctx)

	flg := OptLogFile.Flag().Name
	fp, ok, err := getBootstrapFlagValue(flg, "", OptLogFile.Usage(), osArgs)
	if err != nil {
		bootLog.Warn("Reading log file from flag", lga.Flag, flg, lga.Err, err)
	}
	if ok {
		bootLog.Debug("Log file specified via flag", lga.Flag, flg, lga.Path, fp)
		return fp
	}

	fp, ok = os.LookupEnv(config.EnvarLogPath)
	if ok {
		bootLog.Debug("Log file specified via envar", lga.Env, config.EnvarLogPath, lga.Path, fp)
		return fp
	}

	var o options.Options
	if cfg != nil {
		o = cfg.Options
	}

	fp = OptLogFile.Get(o)
	bootLog = bootLog.With(lga.Key, OptLogFile.Key(), lga.Path, fp)

	if !o.IsSet(OptLogFile) {
		bootLog.Debug("Log file not explicitly set in config; using default")
		return fp
	}

	if fp == "" {
		bootLog.Debug(`Log file explicitly set to "" in config; logging disabled`)
	}

	bootLog.Debug("Log file specified via config")
	return fp
}

// getDefaultLogFilePath returns the OS-dependent log file path,
// or an empty string if it can't be determined. The file (and its
// parent dir) may not exist.
func getDefaultLogFilePath() string {
	p, err := userlogdir.UserLogDir()
	if err != nil {
		return ""
	}

	return filepath.Join(p, "sq", "sq.log")
}

var _ options.Opt = LogLevelOpt{}

// NewLogLevelOpt returns a new LogLevelOpt instance.
func NewLogLevelOpt(key string, defaultVal slog.Level, usage, help string) LogLevelOpt {
	opt := options.NewBaseOpt(key, nil, usage, help)
	return LogLevelOpt{BaseOpt: opt, defaultVal: defaultVal}
}

// LogLevelOpt is an options.Opt for slog.Level.
type LogLevelOpt struct {
	options.BaseOpt
	defaultVal slog.Level
}

// Process implements options.Processor. It converts matching
// string values in o into slog.Level. If no match found,
// the input arg is returned unchanged. Otherwise, a clone is
// returned.
func (op LogLevelOpt) Process(o options.Options) (options.Options, error) {
	if o == nil {
		return nil, nil //nolint:nilnil
	}

	key := op.Key()
	v, ok := o[key]
	if !ok || v == nil {
		return o, nil
	}

	if _, isLevel := v.(slog.Level); isLevel {
		return o, nil
	}

	lvl, err := op.convert(v)
	if err != nil {
		return nil, err
	}

	o = o.Clone()
	o[key] = lvl
	return o, nil
}

// convert coerces v into a slog.Level. A level reaches the config only as a
// string or an int, so Get needs this to avoid silently returning op's default
// for an Options that has not been through Registry.Process. See #1209.
func (op LogLevelOpt) convert(v any) (slog.Level, error) {
	key := op.Key()
	switch x := v.(type) {
	case slog.Level:
		return x, nil
	case int:
		return slog.Level(x), nil
	case string:
		var lvl slog.Level
		if err := lvl.UnmarshalText([]byte(x)); err != nil {
			return 0, errz.Wrapf(err, "option {%s} is not a valid {%T}", key, lvl)
		}
		return lvl, nil
	default:
		return 0, errz.Errorf("option {%s} should be {%T} or {%T} but got {%T}: %v",
			key, slog.LevelDebug, "", x, x)
	}
}

// Get returns op's value in o. If o is nil, or no value
// is set, op's default value is returned.
func (op LogLevelOpt) Get(o options.Options) slog.Level {
	if o == nil {
		return op.defaultVal
	}

	v, ok := o[op.Key()]
	if !ok || v == nil {
		return op.defaultVal
	}

	lvl, err := op.convert(v)
	if err != nil {
		return op.defaultVal
	}

	return lvl
}

// GetAny implements options.Opt.
func (op LogLevelOpt) GetAny(o options.Options) any {
	return op.Get(o)
}

// DefaultAny implements options.Opt.
func (op LogLevelOpt) DefaultAny() any {
	return op.defaultVal
}
