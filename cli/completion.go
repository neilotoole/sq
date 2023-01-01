package cli

import (
	"context"
	"strings"

	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/libsq/source"
)

// completionFunc is a shell completion function.
type completionFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

var (
	_ completionFunc = completeDriverType
	_ completionFunc = completeSLQ
	_ completionFunc = new(handleTableCompleter).complete
)

// completeHandle is a completionFunc that suggests handles.
// The max arg is the maximum number of completions. Set to 0
// for no limit.
func completeHandle(max int) completionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if max > 0 && len(args) >= max {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		rc := RunContextFrom(cmd.Context())
		handles := rc.Config.Sources.Handles()

		handles, _ = lo.Difference(handles, args)

		return handles, cobra.ShellCompDirectiveNoFileComp
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
func completeDriverType(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	rc := RunContextFrom(cmd.Context())
	if rc.databases == nil {
		err := rc.init()
		if err != nil {
			rc.Log.Error(err)
			return nil, cobra.ShellCompDirectiveError
		}
	}

	drivers := rc.registry.Drivers()
	types := make([]string, len(drivers))
	for i, driver := range rc.registry.Drivers() {
		types[i] = string(driver.DriverMetadata().Type)
	}

	return types, cobra.ShellCompDirectiveNoFileComp
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
func (c *handleTableCompleter) complete(cmd *cobra.Command, args []string, toComplete string) ([]string,
	cobra.ShellCompDirective,
) {
	rc := RunContextFrom(cmd.Context())
	if err := rc.init(); err != nil {
		rc.Log.Error(err)
		return nil, cobra.ShellCompDirectiveError
	}

	// We don't want the user to wait around forever for
	// shell completion, so we set a timeout. Typically
	// this is something like 500ms.
	ctx, cancelFn := context.WithTimeout(cmd.Context(), rc.Config.Defaults.ShellCompletionTimeout)
	defer cancelFn()

	if c.max > 0 && len(args) >= c.max {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	if toComplete == "" {
		if c.handleRequired {
			return c.completeHandle(ctx, rc, args, toComplete)
		}
		return c.completeEither(ctx, rc, args, toComplete)
	}

	// There's some input. We expect the input to be of the
	// form "@handle" or ".table". That is, the input should
	// start with either '@' or '.'.
	switch toComplete[0] {
	default:
		// User input was something other than '@' or '.'
		return nil, cobra.ShellCompDirectiveError
	case '@':
		return c.completeHandle(ctx, rc, args, toComplete)
	case '.':
		if c.handleRequired {
			return nil, cobra.ShellCompDirectiveError
		}
		return c.completeTableOnly(ctx, rc, args, toComplete)
	}
}

// completeTableOnly returns suggestions given input beginning with
// a period. Effectively this is completion for tables in the
// active src.
//
//nolint:unparam
func (c *handleTableCompleter) completeTableOnly(ctx context.Context, rc *RunContext, args []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	activeSrc := rc.Config.Sources.Active()
	if activeSrc == nil {
		rc.Log.Error("Active source is nil")
		return nil, cobra.ShellCompDirectiveError
	}

	if c.onlySQL {
		isSQL, err := handleIsSQLDriver(rc, activeSrc.Handle)
		if err != nil {
			rc.Log.Error(err)
			return nil, cobra.ShellCompDirectiveError
		}
		if !isSQL {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}

	tables, err := getTableNamesForHandle(ctx, rc, activeSrc.Handle)
	if err != nil {
		rc.Log.Error(err)
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
//
//nolint:unparam
func (c *handleTableCompleter) completeHandle(ctx context.Context, rc *RunContext, args []string,
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
			rc.Log.Error(err)
			return nil, cobra.ShellCompDirectiveError
		}

		if c.onlySQL {
			var isSQL bool
			isSQL, err = handleIsSQLDriver(rc, handle)
			if err != nil {
				rc.Log.Error(err)
				return nil, cobra.ShellCompDirectiveError
			}

			if !isSQL {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
		}

		tables, err := getTableNamesForHandle(ctx, rc, handle)
		if err != nil {
			rc.Log.Error(err)
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

	handles := rc.Config.Sources.Handles()
	// Else, we're dealing with just a handle so far
	var matchingHandles []string
	for _, handle := range handles {
		if strings.HasPrefix(handle, toComplete) {
			if c.onlySQL {
				isSQL, err := handleIsSQLDriver(rc, handle)
				if err != nil {
					rc.Log.Error(err)
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

	tables, err := getTableNamesForHandle(ctx, rc, matchingHandles[0])
	if err != nil {
		// This means that we aren't able to get metadata for this source.
		// This could be because the source is temporarily offline. The
		// best we can do is just to return the handle, without the tables.
		rc.Log.Warn(err)
		return matchingHandles, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	}

	suggestions := []string{matchingHandles[0]}
	for _, table := range tables {
		suggestions = append(suggestions, matchingHandles[0]+"."+table)
	}

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// completeEither returns a union of all handles plus the tables from the active source.
//
//nolint:unparam
func (c *handleTableCompleter) completeEither(ctx context.Context, rc *RunContext, args []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	// There's no input yet.
	// Therefore we want to return a union of all handles
	// plus the tables from the active source.
	activeSrc := rc.Config.Sources.Active()
	if activeSrc == nil {
		rc.Log.Error("Active source is nil")
		return nil, cobra.ShellCompDirectiveError
	}

	var activeSrcTables []string
	isSQL, err := handleIsSQLDriver(rc, activeSrc.Handle)
	if err != nil {
		rc.Log.Error(err)
		return nil, cobra.ShellCompDirectiveError
	}

	if !c.onlySQL || isSQL {
		activeSrcTables, err = getTableNamesForHandle(ctx, rc, activeSrc.Handle)
		if err != nil {
			rc.Log.Error(err)
			return nil, cobra.ShellCompDirectiveError
		}
	}

	var suggestions []string
	for _, table := range activeSrcTables {
		suggestions = append(suggestions, "."+table)
	}

	for _, src := range rc.Config.Sources.Items() {
		if c.onlySQL {
			isSQL, err = handleIsSQLDriver(rc, src.Handle)
			if err != nil {
				rc.Log.Error(err)
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

func handleIsSQLDriver(rc *RunContext, handle string) (bool, error) {
	src, err := rc.Config.Sources.Get(handle)
	if err != nil {
		return false, err
	}

	driver, err := rc.registry.DriverFor(src.Type)
	if err != nil {
		return false, err
	}

	return driver.DriverMetadata().IsSQL, nil
}

func getTableNamesForHandle(ctx context.Context, rc *RunContext, handle string) ([]string, error) {
	src, err := rc.Config.Sources.Get(handle)
	if err != nil {
		return nil, err
	}

	db, err := rc.databases.Open(ctx, src)
	if err != nil {
		return nil, err
	}

	md, err := db.SourceMetadata(ctx)
	if err != nil {
		return nil, err
	}

	return md.TableNames(), nil
}
