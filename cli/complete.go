package cli

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output/format"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/timez"
	"github.com/neilotoole/sq/libsq/source"
)

// OptShellCompletionTimeout determines how long to wait until for long-running
// shell completion operations (such as fetching table names from a DB) before
// giving up.
var OptShellCompletionTimeout = options.NewDuration(
	"shell-completion.timeout",
	"",
	0,
	time.Millisecond*500,
	"Shell completion timeout",
	`How long shell completion should wait before giving up. This can
become relevant when shell completion inspects a source's metadata, e.g. to
offer a list of tables in a source.`,
)

// completionFunc is a shell completion function.
type completionFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

var (
	_ completionFunc = completeDriverType
	_ completionFunc = completeSLQ
	_ completionFunc = (*handleTableCompleter)(nil).complete
)

// completeStrings completes from a slice of string.
func completeStrings(max int, a ...string) completionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if max > 0 && len(args) >= max {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return a, cobra.ShellCompDirectiveNoFileComp & cobra.ShellCompDirectiveKeepOrder
	}
}

// completeBool returns "true" and "false".
func completeBool(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return []string{"true", "false"}, cobra.ShellCompDirectiveNoFileComp
}

// completeHandle is a completionFunc that suggests handles.
// The max arg is the maximum number of completions. Set to 0
// for no limit.
func completeHandle(max int) completionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if max > 0 && len(args) >= max {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		ru := getRun(cmd)
		handles := ru.Config.Collection.Handles()
		handles = lo.Reject(handles, func(item string, index int) bool {
			return !strings.HasPrefix(item, toComplete)
		})

		slices.Sort(handles) // REVISIT: what's the logic for sorting or not?
		handles, _ = lo.Difference(handles, args)

		if ru.Config.Collection.Active() != nil {
			handles = append([]string{source.ActiveHandle}, handles...)
		}

		return handles, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeGroup is a completionFunc that suggests groups.
// The max arg is the maximum number of completions. Set to 0
// for no limit.
func completeGroup(max int) completionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if max > 0 && len(args) >= max {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		ru := getRun(cmd)
		groups := ru.Config.Collection.Groups()
		groups, _ = lo.Difference(groups, args)
		groups = lo.Uniq(groups)
		slices.Sort(groups)
		return groups, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeHandleOrGroup returns the matching list of handles+groups.
func completeHandleOrGroup(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	switch {
	case toComplete == "":
		items, _ := completeHandle(0)(cmd, args, toComplete)
		groups, _ := completeGroup(0)(cmd, args, toComplete)
		items = append(items, groups...)
		items = lo.Uniq(items)
		return items, cobra.ShellCompDirectiveNoFileComp
	case toComplete == "/":
		return []string{}, cobra.ShellCompDirectiveNoFileComp
	case toComplete[0] == '@':
		return completeHandle(0)(cmd, args, toComplete)
	case source.IsValidGroup(toComplete):
		return completeGroup(0)(cmd, args, toComplete)
	default:
		return nil, cobra.ShellCompDirectiveError
	}
}

// completeSLQ is a completionFunc that completes SLQ queries.
// The completion functionality is rudimentary: it only
// completes the "table select" segment (that is, the @HANDLE.NAME)
// segment.
func completeSLQ(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveError
	}

	c := &handleTableCompleter{}
	return c.complete(cmd, args, toComplete)
}

// completeDriverType is a completionFunc that suggests drivers.
func completeDriverType(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	ru := getRun(cmd)
	if ru.Grips == nil {
		if err := preRun(cmd, ru); err != nil {
			lg.Unexpected(logFrom(cmd), err)
			return nil, cobra.ShellCompDirectiveError
		}
	}

	drivers := ru.DriverRegistry.Drivers()
	types := make([]string, len(drivers))
	for i, driver := range ru.DriverRegistry.Drivers() {
		types[i] = string(driver.DriverMetadata().Type)
	}

	return types, cobra.ShellCompDirectiveNoFileComp
}

// completeOptKey is a completionFunc that completes keys for options.Opt.
// If flag.ConfigSrc is set on cmd, the returned completions are limited to
// Opt keys appropriate to that source. For example, if the source is Excel,
// then "driver.csv.delim" won't be offered.
func completeOptKey(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ru := getRun(cmd)
	keys := ru.OptionsRegistry.Keys()

	if cmdFlagChanged(cmd, flag.ConfigSrc) {
		// If using with --src, then we only want to show the opts
		// that apply to that source.
		handle, err := cmd.Flags().GetString(flag.ConfigSrc)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		src, err := ru.Config.Collection.Get(handle)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		opts := filterOptionsForSrc(src.Type, ru.OptionsRegistry.Opts()...)
		keys = lo.Map(opts, func(item options.Opt, index int) string {
			return item.Key()
		})

		if cmdFlagChanged(cmd, flag.ConfigDelete) {
			if len(src.Options) == 0 {
				// Nothing to delete
				return nil, cobra.ShellCompDirectiveError
			}

			// There are options to delete
			return src.Options.Keys(), cobra.ShellCompDirectiveDefault
		}
	}

	if cmdFlagChanged(cmd, flag.ConfigDelete) {
		// At this stage, we have to offer all opts, because the user
		// input could become: $ sq config set -D ingest.header --src @csv
		return ru.OptionsRegistry.Keys(), cobra.ShellCompDirectiveDefault
	}

	keys = lo.Filter(keys, func(item string, index int) bool {
		return strings.HasPrefix(item, toComplete)
	})

	if len(keys) == 0 && len(toComplete) > 0 {
		logFrom(cmd).Warn("Invalid option key", lga.Key, toComplete)
		return nil, cobra.ShellCompDirectiveError
	}

	return keys, cobra.ShellCompDirectiveNoFileComp
}

// completeOptValue is a completionFunc that completes values for options.Opt.
// It expects that args[0] is a valid Opt key.
func completeOptValue(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 1 {
		return nil, cobra.ShellCompDirectiveError
	}

	ru := getRun(cmd)
	opt := ru.OptionsRegistry.Get(args[0])
	if opt == nil {
		logFrom(cmd).Warn("Invalid option key", lga.Key, args[0])
		return nil, cobra.ShellCompDirectiveError
	}

	var a []string
	switch opt.(type) {
	case options.String:
		switch opt.Key() {
		case OptLogFile.Key():
			// We return the default directive, so that the shell will offer
			// regular ol' file completion.
			return a, cobra.ShellCompDirectiveDefault
		case OptDatetimeFormat.Key(), OptTimeFormat.Key(), OptDateFormat.Key():
			return timez.NamedLayouts(), cobra.ShellCompDirectiveNoFileComp
		}

	case LogLevelOpt:
		a = []string{slog.LevelDebug.String(), slog.LevelInfo.String(), slog.LevelWarn.String(), slog.LevelError.String()}
	case format.Opt:
		switch opt.Key() {
		case OptErrorFormat.Key(), OptLogFormat.Key():
			a = []string{string(format.Text), string(format.JSON)}
		default:
			a = stringz.Strings(format.All())
		}
	case options.Bool:
		a = []string{"true", "false"}
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	a = lo.Filter(a, func(item string, index int) bool {
		return strings.HasPrefix(item, toComplete)
	})

	return a, cobra.ShellCompDirectiveNoFileComp
}

// completeTblCopy is a completionFunc for the "tbl copy" command.
func completeTblCopy(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Example invocation:
	//
	//  sq tbl copy @sakila_sl3.actor .new_table
	//
	// Note that the second arg can only be a table name (and
	// not a @HANDLE.TABLE), and it must also be a _new_ table
	// (because we can't copy over an existing table), thus
	// we only suggest "." for the second arg, forcing the user
	// to supply the rest of that new table name.
	switch len(args) {
	case 0:
		c := &handleTableCompleter{onlySQL: true}
		return c.complete(cmd, args, toComplete)
	case 1:
		return []string{"."}, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	default:
		return nil, cobra.ShellCompDirectiveError
	}
}

// activeSchemaCompleter encapsulates completion for flag.ActiveSchema.
// The completionFunc is activeSchemaCompleter.complete.
//
// Example usage:
//
//	# Only schema
//	$ sq --src.schema information_schema '.tables'
//
//	# Using catalog.schema
//	$ sq --src.schema postgres.information_schema '.tables'
//
// Note that some drivers don't support the catalog mechanism (e.g. SQLite).
//
// The returned slice contains the names of the schemas in the source, followed
// by the names of the catalogs (suffixed with a period, e.g. "sakila.", so
// that the user can complete the catalog.schema input, e.g. "sakila.public").
// For example:
//
//	information_schema		<-- this a schema in the active source
//	pg_catalog
//	public
//	sakila.								<-- note the trailing period, this is a catalog
//	customers.
//	postgres.
//
// If toComplete already contains a period (e.g. "sakila."), then the
// returned slice contains only the matching catalog-qualified schemas,
// e.g. "sakila.public", "sakila.information_schema", etc.
//
// Note the field activeSchemaCompleter.activeSourceFunc. This func is used to
// determine the source to act against. This is configurable because some commands
// may honor a flag (flag.ActiveSrc), but a different flag (or even cmd args)
// could also be used. Func getActiveSourceViaFlag is one such func impl. When
// that is used, if the command has flag.ActiveSrc set, it is honored. Otherwise,
// the config's active source is used. For example:
//
//	$ sq --src @sakila/pg12 --src.schema postgres.information_schema '.tables'
//
// Note also: if the targeted source is not SQL (e.g. CSV), an error is returned.
type activeSchemaCompleter struct {
	// activeSourceFunc is a function that returns the active source.
	// Typically the active source comes from the config, but it can also
	// be supplied via other means, e.g. flag.ActiveSrc or a command arg.
	activeSourceFunc func(cmd *cobra.Command, args []string) (*source.Source, error)
}

func (c activeSchemaCompleter) complete(cmd *cobra.Command, args []string, toComplete string,
) ([]string, cobra.ShellCompDirective) {
	const baseDirective = cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveKeepOrder

	log, ru := logFrom(cmd), getRun(cmd)
	if err := preRun(cmd, ru); err != nil {
		lg.Unexpected(log, err)
		return nil, cobra.ShellCompDirectiveError
	}

	src, err := c.activeSourceFunc(cmd, args)
	if err != nil {
		lg.Unexpected(log, err)
		return nil, cobra.ShellCompDirectiveError
	}

	if src == nil {
		log.Debug("No active source, thus no completion for flag", lga.Flag, flag.ActiveSrc)
		return nil, cobra.ShellCompDirectiveError
	}

	// If toComplete contains a period, then we extract the part before
	// the period into inputCatalog.
	var inputCatalog string
	if toComplete != "" {
		if strings.ContainsRune(toComplete, '.') {
			// User has supplied a catalog.schema (or at least a "catalog.")
			parts := strings.Split(toComplete, ".")
			if len(parts) > 2 {
				return nil, cobra.ShellCompDirectiveError
			}
			inputCatalog = parts[0]
			if inputCatalog == "" {
				// User supplied input of the form ".schema" (leading period),
				// which is invalid.
				return nil, cobra.ShellCompDirectiveError
			}
			src.Catalog = inputCatalog
		}
	}

	if ok, _ := isSQLDriver(ru, src.Handle); !ok {
		// Not a SQL driver, completion is N/A.
		return nil, cobra.ShellCompDirectiveError
	}

	drvr, err := ru.DriverRegistry.SQLDriverFor(src.Type)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	// We don't want the user to wait around forever for
	// shell completion, so we set a timeout. Typically
	// this is something like 500ms.
	ctx, cancelFn := context.WithTimeout(cmd.Context(), OptShellCompletionTimeout.Get(ru.Config.Options))
	defer cancelFn()

	grip, err := ru.Grips.Open(ctx, src)
	if err != nil {
		lg.Unexpected(log, err)
		return nil, cobra.ShellCompDirectiveError
	}

	db, err := grip.DB(ctx)
	if err != nil {
		lg.Unexpected(log, err)
		return nil, cobra.ShellCompDirectiveError
	}

	defer lg.WarnIfCloseError(log, lgm.CloseDB, db)

	a, err := drvr.ListSchemas(ctx, db)
	if err != nil {
		lg.Unexpected(log, err)
		return nil, cobra.ShellCompDirectiveError
	}

	if len(a) == 0 {
		return nil, cobra.ShellCompDirectiveError
	}

	if inputCatalog != "" {
		// We have a catalog, so we need to prepend it to each
		// schema name.
		for i := range a {
			a[i] = inputCatalog + "." + a[i]
		}

		a = lo.Filter(a, func(item string, index int) bool {
			return strings.HasPrefix(item, toComplete)
		})

		return a, baseDirective
	}

	if drvr.Dialect().Catalog {
		var catalogs []string
		if catalogs, err = drvr.ListCatalogs(ctx, db); err != nil {
			// We continue even if an error occurs.
			log.Warn("List catalogs", lga.Err, err)
		}

		for i := range catalogs {
			a = append(a, catalogs[i]+".")
		}
	}

	a = lo.Filter(a, func(item string, index int) bool {
		return strings.HasPrefix(item, toComplete)
	})

	for i := range a {
		// If any of the completions has a trailing period (i.e. they've
		// only typed the catalog name), then we need cobra.ShellCompDirectiveNoSpace,
		// because the user has more typing to do to complete the catalog.schema.
		if strings.HasSuffix(a[i], ".") {
			return a, baseDirective | cobra.ShellCompDirectiveNoSpace
		}
	}

	return a, baseDirective
}

// getActiveSourceViaFlag returns the active source, either from the
// config or from flag.ActiveSrc. This function is intended for use
// with activeSchemaCompleter.activeSourceFunc.
func getActiveSourceViaFlag(cmd *cobra.Command, _ []string) (*source.Source, error) {
	if !cmdFlagChanged(cmd, flag.ActiveSrc) {
		// User has not supplied --src, so we'll use the
		// config's active source.
		return getRun(cmd).Config.Collection.Active(), nil
	}

	handle, err := cmd.Flags().GetString(flag.ActiveSrc)
	if err != nil {
		return nil, err
	}

	var src *source.Source
	if src, err = getRun(cmd).Config.Collection.Get(handle); err != nil {
		return nil, err
	}

	return src, nil
}

// getActiveSourceViaArgs returns the active source, either from the
// config or from the handle in the first cmd arg. This function is intended
// for use with activeSchemaCompleter.activeSourceFunc.
func getActiveSourceViaArgs(cmd *cobra.Command, args []string) (*source.Source, error) {
	if len(args) == 0 {
		// No args supplied, so we'll use the config's active source.
		return getRun(cmd).Config.Collection.Active(), nil
	}

	handle := args[0]
	return getRun(cmd).Config.Collection.Get(handle)
}

// handleTableCompleter encapsulates completion of a handle
// ("@sakila_sl3"), table (".actor"), or @HANDLE.TABLE
// ("@sakila_sl3.actor"). Its complete method is a completionFunc.
type handleTableCompleter struct {
	// onlySQL, when true, filters out non-SQL sources.
	onlySQL bool

	// handleRequired, when true, means that only @HANDLE.TABLE
	// suggestions are offered. That is, naked .TABLE suggestions
	// will not be offered.
	handleRequired bool

	// max indicates the maximum number of completions
	// to offer. Use 0 to indicate no limit. Frequently this
	// is set to 1 to if the command accepts only one argument.
	max int
}

// complete is the completionFunc for handleTableCompleter.
func (c *handleTableCompleter) complete(cmd *cobra.Command, args []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	ru := getRun(cmd)
	if err := preRun(cmd, ru); err != nil {
		lg.Unexpected(logFrom(cmd), err)
		return nil, cobra.ShellCompDirectiveError
	}

	// We don't want the user to wait around forever for
	// shell completion, so we set a timeout. Typically
	// this is something like 500ms.
	ctx, cancelFn := context.WithTimeout(cmd.Context(), OptShellCompletionTimeout.Get(ru.Config.Options))
	defer cancelFn()

	if c.max > 0 && len(args) >= c.max {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	if toComplete == "" {
		if c.handleRequired {
			return c.completeHandle(ctx, ru, args, toComplete)
		}
		return c.completeEither(ctx, ru, args, toComplete)
	}

	// There's some input. We expect the input to be of the
	// form "@handle" or ".table". That is, the input should
	// start with either '@' or '.'.
	switch toComplete[0] {
	default:
		// User input was something other than '@' or '.'
		return nil, cobra.ShellCompDirectiveError
	case '@':
		return c.completeHandle(ctx, ru, args, toComplete)
	case '.':
		if c.handleRequired {
			return nil, cobra.ShellCompDirectiveError
		}
		return c.completeTableOnly(ctx, ru, args, toComplete)
	}
}

// completeTableOnly returns suggestions given input beginning with
// a period. Effectively this is completion for tables in the
// active src.
func (c *handleTableCompleter) completeTableOnly(ctx context.Context, ru *run.Run, _ []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	activeSrc := ru.Config.Collection.Active()
	if activeSrc == nil {
		lg.FromContext(ctx).Error("Active source is nil")
		return nil, cobra.ShellCompDirectiveError
	}

	if c.onlySQL {
		isSQL, err := isSQLDriver(ru, activeSrc.Handle)
		if err != nil {
			lg.Unexpected(lg.FromContext(ctx), err)
			return nil, cobra.ShellCompDirectiveError
		}
		if !isSQL {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}

	tables, err := getTableNamesForHandle(ctx, ru, activeSrc.Handle)
	if err != nil {
		lg.Unexpected(lg.FromContext(ctx), err)
		return nil, cobra.ShellCompDirectiveError
	}

	var suggestions []string
	for _, table := range tables {
		if strings.HasPrefix(table, toComplete[1:]) {
			suggestions = append(suggestions, "."+table)
		}
	}

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// completeHandle returns suggestions given input beginning with
// a '@'. The returned suggestions could be @HANDLE, or @HANDLE.TABLE.
func (c *handleTableCompleter) completeHandle(ctx context.Context, ru *run.Run, _ []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	// We're dealing with a handle.

	// But we could be dealing with just the handle ("@sakila_sl3")
	// or a @HANDLE.TABLE ("@sakila_sl3.actor").
	if strings.ContainsRune(toComplete, '.') {
		if strings.Count(toComplete, ".") > 1 {
			// Can only have one period
			return nil, cobra.ShellCompDirectiveError
		}

		// It's a handle with a full handle and at least a
		// partial table name, such as "@sakila_sl3.fil"
		handle, partialTbl, err := source.ParseTableHandle(strings.TrimSuffix(toComplete, "."))
		if err != nil {
			lg.Unexpected(lg.FromContext(ctx), err)
			return nil, cobra.ShellCompDirectiveError
		}

		if c.onlySQL {
			var isSQL bool
			isSQL, err = isSQLDriver(ru, handle)
			if err != nil {
				lg.Unexpected(lg.FromContext(ctx), err)
				return nil, cobra.ShellCompDirectiveError
			}

			if !isSQL {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
		}

		tables, err := getTableNamesForHandle(ctx, ru, handle)
		if err != nil {
			lg.Unexpected(lg.FromContext(ctx), err)
			return nil, cobra.ShellCompDirectiveError
		}

		var suggestions []string
		for _, table := range tables {
			if strings.HasPrefix(table, partialTbl) {
				suggestions = append(suggestions, handle+"."+table)
			}
		}

		return suggestions, cobra.ShellCompDirectiveNoFileComp
	}

	handles := ru.Config.Collection.Handles()
	handles = append([]string{source.ActiveHandle}, handles...)
	// Else, we're dealing with just a handle so far
	var matchingHandles []string
	for _, handle := range handles {
		if strings.HasPrefix(handle, toComplete) {
			if c.onlySQL {
				isSQL, err := isSQLDriver(ru, handle)
				if err != nil {
					lg.Unexpected(lg.FromContext(ctx), err)
					return nil, cobra.ShellCompDirectiveError
				}
				if !isSQL {
					continue
				}
			}

			matchingHandles = append(matchingHandles, handle)
		}
	}

	switch len(matchingHandles) {
	default:
		return matchingHandles, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	case 0:
		return nil, cobra.ShellCompDirectiveNoFileComp
	case 1:
		// Only one handle match, so we will present that complete
		// handle, plus a suggestion (@HANDLE.TABLE) for each of the tables
		// for that handle
	}

	tables, err := getTableNamesForHandle(ctx, ru, matchingHandles[0])
	if err != nil {
		// This means that we aren't able to get metadata for this source.
		// This could be because the source is temporarily offline. The
		// best we can do is just to return the handle, without the tables.
		lg.WarnIfError(lg.FromContext(ctx), "Get metadata", err)
		return matchingHandles, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	}

	suggestions := []string{matchingHandles[0]}
	for _, table := range tables {
		suggestions = append(suggestions, matchingHandles[0]+"."+table)
	}

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// completeEither returns a union of all handles plus the tables from the active source.
func (c *handleTableCompleter) completeEither(ctx context.Context, ru *run.Run,
	_ []string, _ string,
) ([]string, cobra.ShellCompDirective) {
	var suggestions []string

	// There's no input yet.
	// Therefore we want to return a union of all handles
	// plus the tables from the active source.
	activeSrc := ru.Config.Collection.Active()
	if activeSrc != nil {
		var activeSrcTables []string
		isSQL, err := isSQLDriver(ru, activeSrc.Handle)
		if err != nil {
			lg.Unexpected(lg.FromContext(ctx), err)
			return nil, cobra.ShellCompDirectiveError
		}

		if !c.onlySQL || isSQL {
			activeSrcTables, err = getTableNamesForHandle(ctx, ru, activeSrc.Handle)
			if err != nil {
				// This can happen if the active source is offline.
				// Log the error, but continue below, because we still want to
				// list the handles.
				lg.FromContext(ctx).Warn("completion: failed to get table metadata from active source",
					lga.Err, err, lga.Src, activeSrc)
			}
		}

		for _, table := range activeSrcTables {
			suggestions = append(suggestions, "."+table)
		}
	}

	for _, src := range ru.Config.Collection.Sources() {
		if c.onlySQL {
			isSQL, err := isSQLDriver(ru, src.Handle)
			if err != nil {
				lg.Unexpected(lg.FromContext(ctx), err)
				return nil, cobra.ShellCompDirectiveError
			}
			if !isSQL {
				continue
			}
		}

		suggestions = append(suggestions, src.Handle)
	}

	return suggestions, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

func isSQLDriver(ru *run.Run, handle string) (bool, error) {
	src, err := ru.Config.Collection.Get(handle)
	if err != nil {
		return false, err
	}

	driver, err := ru.DriverRegistry.DriverFor(src.Type)
	if err != nil {
		return false, err
	}

	return driver.DriverMetadata().IsSQL, nil
}

func getTableNamesForHandle(ctx context.Context, ru *run.Run, handle string) ([]string, error) {
	src, err := ru.Config.Collection.Get(handle)
	if err != nil {
		return nil, err
	}

	grip, err := ru.Grips.Open(ctx, src)
	if err != nil {
		return nil, err
	}

	// TODO: We shouldn't have to load the full metadata just to get
	// the table names. driver.SQLDriver should have a method ListTables.
	md, err := grip.SourceMetadata(ctx, false)
	if err != nil {
		return nil, err
	}

	return md.TableNames(), nil
}
