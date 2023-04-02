// Package cli implements sq's CLI. The spf13/cobra library
// is used, with some notable modifications. Although cobra
// provides excellent functionality, it has some issues.
// Most prominently, its documentation suggests reliance
// upon package-level constructs for initializing the
// command tree (bad for testing).
//
// Thus, this cmd package deviates from cobra's suggested
// usage pattern by eliminating all pkg-level constructs
// (which makes testing easier), and also replaces cobra's
// Command.RunE func signature with a signature that accepts
// as its first argument the RunContext type.
//
// RunContext is similar to context.Context (and contains
// an instance of that), but also encapsulates injectable
// resources such as config and logging.
//
// Update (Dec 2020): recent releases of cobra now support
// accessing Context from the cobra.Command. At some point
// it may make sense to revisit the way commands are
// constructed, to use this now-standard cobra mechanism.
//
// The entry point to this pkg is the Execute function.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/neilotoole/sq/libsq/core/lg"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/cli/buildinfo"

	"golang.org/x/exp/slog"

	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/csvw"
	"github.com/neilotoole/sq/cli/output/htmlw"
	"github.com/neilotoole/sq/cli/output/jsonw"
	"github.com/neilotoole/sq/cli/output/markdownw"
	"github.com/neilotoole/sq/cli/output/raww"
	"github.com/neilotoole/sq/cli/output/tablew"
	"github.com/neilotoole/sq/cli/output/xlsxw"
	"github.com/neilotoole/sq/cli/output/xmlw"
	"github.com/neilotoole/sq/drivers/csv"
	"github.com/neilotoole/sq/drivers/json"
	"github.com/neilotoole/sq/drivers/mysql"
	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/drivers/sqlserver"
	"github.com/neilotoole/sq/drivers/userdriver"
	"github.com/neilotoole/sq/drivers/userdriver/xmlud"
	"github.com/neilotoole/sq/drivers/xlsx"
	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

func init() { //nolint:gochecknoinits
	cobra.EnableCommandSorting = false
}

// errNoMsg is a sentinel error indicating that a command
// has failed, but that no error message should be printed.
// This is useful in the case where any error information may
// already have been printed as part of the command output.
var errNoMsg = errors.New("")

// Execute builds a RunContext using ctx and default
// settings, and invokes ExecuteWith.
func Execute(ctx context.Context, stdin *os.File, stdout, stderr io.Writer, args []string) error {
	rc, err := newDefaultRunContext(stdin, stdout, stderr)
	if err != nil {
		printError(rc, err)
		return err
	}

	defer rc.Close() // ok to call rc.Close on nil rc

	ctx = lg.NewContext(ctx, rc.Log)
	return ExecuteWith(ctx, rc, args)
}

// ExecuteWith invokes the cobra CLI framework, ultimately
// resulting in a command being executed. The caller must
// invoke rc.Close.
func ExecuteWith(ctx context.Context, rc *RunContext, args []string) error {
	log := lg.FromContext(ctx)
	log.Debug("EXECUTE", "args", strings.Join(args, " "))
	log.Debug("Build info", "build", buildinfo.Info())
	log.Debug("Config", "version", rc.Config.Version, "filepath", rc.ConfigStore.Location())

	ctx = WithRunContext(ctx, rc)

	rootCmd := newCommandTree(rc)
	var err error

	// The following is a workaround for the fact that cobra doesn't
	// currently (as of 2017) support executing the root command with
	// arbitrary args. That is to say, if you execute:
	//
	//   $ sq @sakila_sl3.actor
	//
	// then cobra will look for a command named "@sakila_sl3.actor",
	// and when it doesn't find such a command, it returns
	// an "unknown command" error.
	//
	// NOTE: This entire mechanism is ancient. Perhaps cobra
	//  now handles this situation?

	// We need to perform handling for autocomplete
	if len(args) > 0 && args[0] == "__complete" {
		if hasMatchingChildCommand(rootCmd, args[1]) {
			// If there is a matching child command, we let rootCmd
			// handle it, as per normal.
			rootCmd.SetArgs(args)
		} else {
			// There's no command matching the first argument to __complete.
			// Therefore, we assume that we want to perform completion
			// for the "slq" command (which is the pseudo-root command).
			effectiveArgs := append([]string{"__complete", "slq"}, args[1:]...)
			rootCmd.SetArgs(effectiveArgs)
		}
	} else {
		var cmd *cobra.Command
		cmd, _, err = rootCmd.Find(args)
		if err != nil {
			// This err will be the "unknown command" error.
			// cobra still returns cmd though. It should be
			// the root cmd.
			if cmd == nil || cmd.Name() != rootCmd.Name() {
				// Not sure if this can happen anymore? Can prob delete?
				panic(fmt.Sprintf("bad cobra cmd state: %v", cmd))
			}

			// If we have args [sq, arg1, arg2] then we redirect
			// to the "slq" command by modifying args to
			// look like: [query, arg1, arg2] -- noting that SetArgs
			// doesn't want the first args element.
			effectiveArgs := append([]string{"slq"}, args...)
			rootCmd.SetArgs(effectiveArgs)
		} else {
			if cmd.Name() == rootCmd.Name() {
				// Not sure why we have two paths to this, but it appears
				// that we've found the root cmd again, so again
				// we redirect to "slq" cmd.

				a := append([]string{"slq"}, args...)
				rootCmd.SetArgs(a)
			} else {
				// It's just a normal command like "sq ls" or such.

				// Explicitly set the args on rootCmd as this makes
				// cobra happy when this func is executed via tests.
				// Haven't explored the reason why.
				rootCmd.SetArgs(args)
			}
		}
	}

	// Execute rootCmd; cobra will find the appropriate
	// sub-command, and ultimately execute that command.
	err = rootCmd.ExecuteContext(ctx)
	if err != nil {
		printError(rc, err)
	}

	return err
}

// cobraMu exists because cobra relies upon package-level
// constructs. This does not sit well with parallel tests.
var cobraMu sync.Mutex

// newCommandTree builds sq's command tree, returning
// the root cobra command.
func newCommandTree(rc *RunContext) (rootCmd *cobra.Command) {
	cobraMu.Lock()
	defer cobraMu.Unlock()

	rootCmd = newRootCmd()
	rootCmd.DisableAutoGenTag = true
	rootCmd.SetOut(rc.Out)
	rootCmd.SetErr(rc.ErrOut)
	rootCmd.Flags().SortFlags = false

	// The --help flag must be explicitly added to rootCmd,
	// or else cobra tries to do its own (unwanted) thing.
	// The behavior of cobra in this regard seems to have
	// changed? This particular incantation currently does the trick.
	rootCmd.Flags().Bool(flagHelp, false, "Show sq help")

	helpCmd := addCmd(rc, rootCmd, newHelpCmd())
	rootCmd.SetHelpCommand(helpCmd)

	// From the end user's perspective, slqCmd is *effectively* the
	// root cmd. We need to perform some trickery to make it output help
	// such that "sq help" and "sq --help" output the same thing.
	slqCmd := newSLQCmd()
	slqCmd.SetHelpFunc(func(command *cobra.Command, i []string) {
		panicOn(rootCmd.Help())
	})

	addCmd(rc, rootCmd, slqCmd)
	addCmd(rc, rootCmd, newSQLCmd())

	addCmd(rc, rootCmd, newSrcCommand())
	addCmd(rc, rootCmd, newSrcAddCmd())
	addCmd(rc, rootCmd, newSrcListCmd())
	addCmd(rc, rootCmd, newSrcRemoveCmd())
	addCmd(rc, rootCmd, newScratchCmd())

	addCmd(rc, rootCmd, newInspectCmd())
	addCmd(rc, rootCmd, newPingCmd())

	addCmd(rc, rootCmd, newVersionCmd())

	driverCmd := addCmd(rc, rootCmd, newDriverCmd())
	addCmd(rc, driverCmd, newDriverListCmd())

	tblCmd := addCmd(rc, rootCmd, newTblCmd())
	addCmd(rc, tblCmd, newTblCopyCmd())
	addCmd(rc, tblCmd, newTblTruncateCmd())
	addCmd(rc, tblCmd, newTblDropCmd())

	addCmd(rc, rootCmd, newCompletionCmd())
	addCmd(rc, rootCmd, newManCmd())

	return rootCmd
}

// hasMatchingChildCommand returns true if s is a full or prefix
// match for any of cmd's children. For example, if cmd has
// children [inspect, ls, rm], then "insp" or "ls" would return true.
func hasMatchingChildCommand(cmd *cobra.Command, s string) bool {
	for _, child := range cmd.Commands() {
		if strings.HasPrefix(child.Name(), s) {
			return true
		}
	}
	return false
}

// addCmd adds the command returned by cmdFn to parentCmd.
func addCmd(rc *RunContext, parentCmd, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().SortFlags = false

	if cmd.Name() != "help" {
		// Don't add the --help flag to the help command.
		cmd.Flags().Bool(flagHelp, false, "help for "+cmd.Name())
	}

	cmd.DisableAutoGenTag = true

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		rc.Cmd = cmd
		rc.Args = args
		err := rc.init()
		return err
	}

	runE := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed(flagVersion) {
			// Bit of a hack: flag --version on any command
			// results in execVersion being invoked
			return execVersion(cmd, args)
		}

		return runE(cmd, args)
	}

	// We handle the errors ourselves (rather than let cobra do it)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	parentCmd.AddCommand(cmd)

	return cmd
}

type runContextKey struct{}

// WithRunContext returns ctx with rc added as a value.
func WithRunContext(ctx context.Context, rc *RunContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, runContextKey{}, rc)
}

// RunContextFrom extracts the RunContext added to ctx via WithRunContext.
func RunContextFrom(ctx context.Context) *RunContext {
	return ctx.Value(runContextKey{}).(*RunContext)
}

// RunContext is a container for injectable resources passed
// to all execX funcs. The Close method should be invoked when
// the RunContext is no longer needed.
type RunContext struct {
	// Stdin typically is os.Stdin, but can be changed for testing.
	Stdin *os.File

	// Out is the output destination.
	// If nil, default to stdout.
	Out io.Writer

	// ErrOut is the error output destination.
	// If nil, default to stderr.
	ErrOut io.Writer

	// Cmd is the command instance provided by cobra for
	// the currently executing command. This field will
	// be set before the command's runFunc is invoked.
	Cmd *cobra.Command

	// Log is the run's logger.
	Log *slog.Logger

	// Args is the arg slice supplied by cobra for
	// the currently executing command. This field will
	// be set before the command's runFunc is invoked.
	Args []string

	// Config is the run's config.
	Config *config.Config

	// ConfigStore is run's config store.
	ConfigStore config.Store

	initOnce sync.Once
	initErr  error

	// writers holds the various writer types that
	// the CLI uses to print output.
	writers *writers

	registry  *driver.Registry
	files     *source.Files
	databases *driver.Databases
	clnup     *cleanup.Cleanup
}

// newDefaultRunContext returns a RunContext configured
// with standard values for logging, config, etc. This
// effectively is the bootstrap mechanism for sq.
//
// Note: This func always returns a RunContext, even if
// an error occurs during bootstrap of the RunContext (for
// example if there's a config error). We do this to provide
// enough framework so that such an error can be logged or
// printed per the normal mechanisms if at all possible.
func newDefaultRunContext(stdin *os.File, stdout, stderr io.Writer) (*RunContext, error) {
	rc := &RunContext{
		Stdin:  stdin,
		Out:    stdout,
		ErrOut: stderr,
	}

	log, clnup, loggingErr := defaultLogging()
	rc.Log = log
	rc.clnup = clnup

	cfg, cfgStore, configErr := defaultConfig()
	rc.ConfigStore = cfgStore
	rc.Config = cfg

	switch {
	case rc.clnup == nil:
		rc.clnup = cleanup.New()
	case rc.Config == nil:
		rc.Config = config.New()
	}

	if configErr != nil {
		// configErr is more important, return that first
		return rc, configErr
	}

	if loggingErr != nil {
		return rc, loggingErr
	}

	return rc, nil
}

// init is invoked by cobra prior to the command RunE being
// invoked. It sets up the registry, databases, writers and related
// fundamental components. Subsequent invocations of this method
// are no-op.
func (rc *RunContext) init() error {
	rc.initOnce.Do(func() {
		rc.initErr = rc.doInit()
	})

	return rc.initErr
}

// doInit performs the actual work of initializing rc.
// It must only be invoked once.
func (rc *RunContext) doInit() error {
	rc.clnup = cleanup.New()
	cfg := rc.Config
	log := rc.Log

	// If the --output=/some/file flag is set, then we need to
	// override rc.Out (which is typically stdout) to point it at
	// the output destination file.
	if cmdFlagChanged(rc.Cmd, flagOutput) {
		fpath, _ := rc.Cmd.Flags().GetString(flagOutput)
		fpath, err := filepath.Abs(fpath)
		if err != nil {
			return errz.Wrapf(err, "failed to get absolute path for --%s", flagOutput)
		}

		// Ensure the parent dir exists
		err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm)
		if err != nil {
			return errz.Wrapf(err, "failed to make parent dir for --%s", flagOutput)
		}

		f, err := os.Create(fpath)
		if err != nil {
			return errz.Wrapf(err, "failed to open file specified by flag --%s", flagOutput)
		}

		rc.clnup.AddC(f) // Make sure the file gets closed eventually
		rc.Out = f
	}

	rc.writers, rc.Out, rc.ErrOut = newWriters(rc.Cmd, rc.Config.Defaults, rc.Out, rc.ErrOut)

	var scratchSrcFunc driver.ScratchSrcFunc

	// scratchSrc could be nil, and that's ok
	scratchSrc := cfg.Sources.Scratch()
	if scratchSrc == nil {
		scratchSrcFunc = sqlite3.NewScratchSource
	} else {
		scratchSrcFunc = func(log *slog.Logger, name string) (src *source.Source, clnup func() error, err error) {
			return scratchSrc, nil, nil
		}
	}

	var err error
	rc.files, err = source.NewFiles(rc.Log)
	if err != nil {
		lg.WarnIfFuncError(rc.Log, "cleanup", rc.clnup.Run)
		return err
	}

	// Note: it's important that files.Close is invoked
	// after databases.Close (hence added to clnup first),
	// because databases could depend upon the existence of
	// files (such as a sqlite db file).
	rc.clnup.AddE(rc.files.Close)
	rc.files.AddTypeDetectors(source.DetectMagicNumber)

	rc.registry = driver.NewRegistry(log)
	rc.databases = driver.NewDatabases(log, rc.registry, scratchSrcFunc)
	rc.clnup.AddC(rc.databases)

	rc.registry.AddProvider(sqlite3.Type, &sqlite3.Provider{Log: log})
	rc.registry.AddProvider(postgres.Type, &postgres.Provider{Log: log})
	rc.registry.AddProvider(sqlserver.Type, &sqlserver.Provider{Log: log})
	rc.registry.AddProvider(mysql.Type, &mysql.Provider{Log: log})
	csvp := &csv.Provider{Log: log, Scratcher: rc.databases, Files: rc.files}
	rc.registry.AddProvider(csv.TypeCSV, csvp)
	rc.registry.AddProvider(csv.TypeTSV, csvp)
	rc.files.AddTypeDetectors(csv.DetectCSV, csv.DetectTSV)

	jsonp := &json.Provider{Log: log, Scratcher: rc.databases, Files: rc.files}
	rc.registry.AddProvider(json.TypeJSON, jsonp)
	rc.registry.AddProvider(json.TypeJSONA, jsonp)
	rc.registry.AddProvider(json.TypeJSONL, jsonp)
	rc.files.AddTypeDetectors(json.DetectJSON, json.DetectJSONA, json.DetectJSONL)

	rc.registry.AddProvider(xlsx.Type, &xlsx.Provider{Log: log, Scratcher: rc.databases, Files: rc.files})
	rc.files.AddTypeDetectors(xlsx.DetectXLSX)
	// One day we may have more supported user driver genres.
	userDriverImporters := map[string]userdriver.ImportFunc{
		xmlud.Genre: xmlud.Import,
	}

	for i, userDriverDef := range cfg.Ext.UserDrivers {
		userDriverDef := userDriverDef

		errs := userdriver.ValidateDriverDef(userDriverDef)
		if len(errs) > 0 {
			err := errz.Combine(errs...)
			err = errz.Wrapf(err, "failed validation of user driver definition [%d] (%q) from config",
				i, userDriverDef.Name)
			return err
		}

		importFn, ok := userDriverImporters[userDriverDef.Genre]
		if !ok {
			return errz.Errorf("unsupported genre %q for user driver %q specified via config",
				userDriverDef.Genre, userDriverDef.Name)
		}

		// For each user driver definition, we register a
		// distinct userdriver.Provider instance.
		udp := &userdriver.Provider{
			Log:       log,
			DriverDef: userDriverDef,
			ImportFn:  importFn,
			Scratcher: rc.databases,
			Files:     rc.files,
		}

		rc.registry.AddProvider(source.Type(userDriverDef.Name), udp)
		rc.files.AddTypeDetectors(udp.TypeDetectors()...)
	}

	return nil
}

// Close should be invoked to dispose of any open resources
// held by rc. If an error occurs during Close and rc.Log
// is not nil, that error is logged at WARN level before
// being returned.
func (rc *RunContext) Close() error {
	if rc == nil {
		return nil
	}

	err := rc.clnup.Run()
	if err != nil && rc.Log != nil {
		rc.Log.Warn("failed to close RunContext", lga.Err, err)
	}

	return err
}

// writers is a container for the various output writers.
type writers struct {
	fm *output.Formatting

	recordw  output.RecordWriter
	metaw    output.MetadataWriter
	srcw     output.SourceWriter
	errw     output.ErrorWriter
	pingw    output.PingWriter
	versionw output.VersionWriter
}

// newWriters returns a writers instance configured per defaults and/or
// flags from cmd. The returned out2/errOut2 values may differ
// from the out/errOut args (e.g. decorated to support colorization).
func newWriters(cmd *cobra.Command, defaults config.Defaults, out,
	errOut io.Writer,
) (w *writers, out2, errOut2 io.Writer) {
	var fm *output.Formatting
	fm, out2, errOut2 = getWriterFormatting(cmd, out, errOut)

	// Package tablew has writer impls for each of the writer interfaces,
	// so we use its writers as the baseline. Later we check the format
	// flags and set the various writer fields depending upon which
	// writers the format implements.
	w = &writers{
		fm:       fm,
		recordw:  tablew.NewRecordWriter(out2, fm),
		metaw:    tablew.NewMetadataWriter(out2, fm),
		srcw:     tablew.NewSourceWriter(out2, fm),
		pingw:    tablew.NewPingWriter(out2, fm),
		errw:     tablew.NewErrorWriter(errOut2, fm),
		versionw: tablew.NewVersionWriter(out2, fm),
	}

	// Invoke getFormat to see if the format was specified
	// via config or flag.
	format := getFormat(cmd, defaults)

	switch format { //nolint:exhaustive
	default:
		// No format specified, use JSON
		w.recordw = jsonw.NewStdRecordWriter(out2, fm)
		w.metaw = jsonw.NewMetadataWriter(out2, fm)
		w.srcw = jsonw.NewSourceWriter(out2, fm)
		w.errw = jsonw.NewErrorWriter(errOut2, fm)
		w.versionw = jsonw.NewVersionWriter(out2, fm)
		w.pingw = jsonw.NewPingWriter(out2, fm)

	case config.FormatTable:
	// Table is the base format, already set above, no need to do anything.

	case config.FormatTSV:
		w.recordw = csvw.NewRecordWriter(out2, fm.ShowHeader, csvw.Tab)
		w.pingw = csvw.NewPingWriter(out2, csvw.Tab)

	case config.FormatCSV:
		w.recordw = csvw.NewRecordWriter(out2, fm.ShowHeader, csvw.Comma)
		w.pingw = csvw.NewPingWriter(out2, csvw.Comma)

	case config.FormatXML:
		w.recordw = xmlw.NewRecordWriter(out2, fm)

	case config.FormatXLSX:
		w.recordw = xlsxw.NewRecordWriter(out2, fm.ShowHeader)

	case config.FormatRaw:
		w.recordw = raww.NewRecordWriter(out2)

	case config.FormatHTML:
		w.recordw = htmlw.NewRecordWriter(out2)

	case config.FormatMarkdown:
		w.recordw = markdownw.NewRecordWriter(out2)

	case config.FormatJSONA:
		w.recordw = jsonw.NewArrayRecordWriter(out2, fm)

	case config.FormatJSONL:
		w.recordw = jsonw.NewObjectRecordWriter(out2, fm)
	}

	return w, out2, errOut2
}

// getWriterFormatting returns a Formatting instance and
// colorable or non-colorable writers. It is permissible
// for the cmd arg to be nil.
func getWriterFormatting(cmd *cobra.Command, out, errOut io.Writer) (fm *output.Formatting, out2, errOut2 io.Writer) {
	fm = output.NewFormatting()

	if cmdFlagChanged(cmd, flagPretty) {
		fm.Pretty, _ = cmd.Flags().GetBool(flagPretty)
	}

	if cmdFlagChanged(cmd, flagVerbose) {
		fm.Verbose, _ = cmd.Flags().GetBool(flagVerbose)
	}

	if cmdFlagChanged(cmd, flagHeader) {
		fm.ShowHeader, _ = cmd.Flags().GetBool(flagHeader)
	}

	// TODO: Should get this default value from config
	colorize := true

	if cmdFlagChanged(cmd, flagOutput) {
		// We're outputting to a file, thus no color.
		colorize = false
	} else if cmdFlagChanged(cmd, flagMonochrome) {
		if mono, _ := cmd.Flags().GetBool(flagMonochrome); mono {
			colorize = false
		}
	}

	if !colorize {
		color.NoColor = true // TODO: shouldn't rely on package-level var
		fm.EnableColor(false)
		out2 = out
		errOut2 = errOut
		return fm, out2, errOut2
	}

	// We do want to colorize
	if !isColorTerminal(out) {
		// But out can't be colorized.
		color.NoColor = true
		fm.EnableColor(false)
		out2, errOut2 = out, errOut
		return fm, out2, errOut2
	}

	// out can be colorized.
	color.NoColor = false
	fm.EnableColor(true)
	out2 = colorable.NewColorable(out.(*os.File))

	// Check if we can colorize errOut
	if isColorTerminal(errOut) {
		errOut2 = colorable.NewColorable(errOut.(*os.File))
	} else {
		// errOut2 can't be colorized, but since we're colorizing
		// out, we'll apply the non-colorable filter to errOut.
		errOut2 = colorable.NewNonColorable(errOut)
	}

	return fm, out2, errOut2
}

func getFormat(cmd *cobra.Command, defaults config.Defaults) config.Format {
	var format config.Format

	switch {
	// cascade through the format flags in low-to-high order of precedence.
	case cmdFlagChanged(cmd, flagTSV):
		format = config.FormatTSV
	case cmdFlagChanged(cmd, flagCSV):
		format = config.FormatCSV
	case cmdFlagChanged(cmd, flagXLSX):
		format = config.FormatXLSX
	case cmdFlagChanged(cmd, flagXML):
		format = config.FormatXML
	case cmdFlagChanged(cmd, flagRaw):
		format = config.FormatRaw
	case cmdFlagChanged(cmd, flagHTML):
		format = config.FormatHTML
	case cmdFlagChanged(cmd, flagMarkdown):
		format = config.FormatMarkdown
	case cmdFlagChanged(cmd, flagTable):
		format = config.FormatTable
	case cmdFlagChanged(cmd, flagJSONL):
		format = config.FormatJSONL
	case cmdFlagChanged(cmd, flagJSONA):
		format = config.FormatJSONA
	case cmdFlagChanged(cmd, flagJSON):
		format = config.FormatJSON
	default:
		// no format flag, use the config value
		format = defaults.Format
	}
	return format
}

// defaultLogging returns a log (and its associated closer) if
// logging has been enabled via envars.
func defaultLogging() (*slog.Logger, *cleanup.Cleanup, error) {
	truncate, _ := strconv.ParseBool(os.Getenv(envarLogTruncate))

	logFilePath, ok := os.LookupEnv(envarLogPath)
	if !ok || logFilePath == "" || strings.TrimSpace(logFilePath) == "" {
		return lg.Discard(), nil, nil
	}

	// Let's try to create the dir holding the logfile... if it already exists,
	// then os.MkdirAll will just no-op
	parent := filepath.Dir(logFilePath)
	err := os.MkdirAll(parent, 0o750)
	if err != nil {
		return lg.Discard(), nil, errz.Wrapf(err, "failed to create parent dir of log file %s", logFilePath)
	}

	flag := os.O_APPEND
	if truncate {
		flag = os.O_TRUNC
	}

	logFile, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|flag, 0o600)
	if err != nil {
		return lg.Discard(), nil, errz.Wrapf(err, "unable to open log file %q", logFilePath)
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

	opts := slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: replace,
	}

	log := slog.New(opts.NewJSONHandler(logFile))

	return log, clnup, nil
}

// defaultConfig loads sq config from the default location
// (~/.config/sq/sq.yml) or the location specified in envars.
func defaultConfig() (*config.Config, config.Store, error) {
	cfgDir, ok := os.LookupEnv(envarConfigDir)
	if !ok {
		// envar not set, let's use the default
		home, err := homedir.Dir()
		if err != nil {
			// TODO: we should be able to run without the homedir... revisit this
			return nil, nil, errz.Wrap(err, "unable to get user home dir for config purposes")
		}

		cfgDir = filepath.Join(home, ".config", "sq")
	}

	cfgPath := filepath.Join(cfgDir, "sq.yml")
	extDir := filepath.Join(cfgDir, "ext")
	cfgStore := &config.YAMLFileStore{Path: cfgPath, ExtPaths: []string{extDir}}

	if !cfgStore.FileExists() {
		cfg := config.New()
		return cfg, cfgStore, nil
	}

	// file does exist, let's try to load it
	cfg, err := cfgStore.Load()
	if err != nil {
		return nil, nil, err
	}

	return cfg, cfgStore, nil
}

// printError is the centralized function for printing
// and logging errors. This func has a lot of (possibly needless)
// redundancy; ultimately err will print if non-nil (even if
// rc or any of its fields are nil).
func printError(rc *RunContext, err error) {
	log := lg.Discard()
	if rc != nil && rc.Log != nil {
		log = rc.Log
	}

	if err == nil {
		log.Warn("printError called with nil error")
		return
	}

	if errors.Is(err, errNoMsg) {
		// errNoMsg is a sentinel err that sq doesn't want to print
		return
	}

	switch {
	default:
	case errors.Is(err, context.Canceled):
		err = errz.New("canceled")
	case errors.Is(err, context.DeadlineExceeded):
		err = errz.New("timeout")
	}

	var cmd *cobra.Command
	if rc != nil {
		cmd = rc.Cmd

		cmdName := "unknown"
		if cmd != nil {
			cmdName = cmd.Name()
		}

		lg.Error(log, "nil command", err, lga.Cmd, cmdName)

		wrtrs := rc.writers
		if wrtrs != nil && wrtrs.errw != nil {
			// If we have an errorWriter, we print to it
			// and return.
			wrtrs.errw.Error(err)
			return
		}

		// Else we don't have an errorWriter, so we fall through
	}

	// If we get this far, something went badly wrong in bootstrap
	// (probably the config is corrupt).
	// At this point, we could just print err to os.Stderr and be done.
	// However, our philosophy is to always provide the ability
	// to output errors in json if possible. So, even though cobra
	// may not have initialized and our own config may be borked, we
	// will still try to determine if the user wants the error
	// in json, specified via flags (by directly using the pflag
	// package) or via sq config's default output format.

	// getWriterFormatting works even if cmd is nil
	fm, _, errOut := getWriterFormatting(cmd, os.Stdout, os.Stderr)

	if bootstrapIsFormatJSON(rc) {
		// The user wants JSON, either via defaults or flags.
		jw := jsonw.NewErrorWriter(errOut, fm)
		jw.Error(err)
		return
	}

	// The user didn't want JSON, so we just print to stderr.
	if isColorTerminal(os.Stderr) {
		fm.Error.Fprintln(os.Stderr, "sq: "+err.Error())
	} else {
		fmt.Fprintln(os.Stderr, "sq: "+err.Error())
	}
}

// cmdFlagChanged returns true if cmd is non-nil and
// has the named flag and that flag been changed.
func cmdFlagChanged(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}

	flag := cmd.Flag(name)
	if flag == nil {
		return false
	}

	return flag.Changed
}

// cmdFlagTrue returns true if flag name has been changed
// and the flag value is true.
func cmdFlagTrue(cmd *cobra.Command, name string) bool {
	if !cmdFlagChanged(cmd, name) {
		return false
	}

	b, err := cmd.Flags().GetBool(name)
	if err != nil {
		panic(err) // Should never happen
	}

	return b
}

// bootstrapIsFormatJSON is a last-gasp attempt to check if the user
// supplied --json=true on the command line, to determine if a
// bootstrap error (hopefully rare) should be output in JSON.
func bootstrapIsFormatJSON(rc *RunContext) bool {
	// If no RunContext, assume false
	if rc == nil {
		return false
	}

	defaultFormat := config.FormatTable
	if rc.Config != nil {
		defaultFormat = rc.Config.Defaults.Format
	}

	// If args were provided, create a new flag set and check
	// for the --json flag.
	if len(rc.Args) > 0 {
		flags := pflag.NewFlagSet("bootstrap", pflag.ContinueOnError)

		jsonFlag := flags.BoolP(flagJSON, flagJSONShort, false, flagJSONUsage)
		err := flags.Parse(rc.Args)
		if err != nil {
			return false
		}

		// No --json flag, return true if the config file default is JSON
		if jsonFlag == nil {
			return defaultFormat == config.FormatJSON
		}

		return *jsonFlag
	}

	// No args, return true if the config file default is JSON
	return defaultFormat == config.FormatJSON
}

func panicOn(err error) {
	if err != nil {
		panic(err)
	}
}
