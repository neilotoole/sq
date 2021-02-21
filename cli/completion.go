package cli

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/libsq/source"
)

// completionFunc is a shell completion function.
type completionFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

var (
	_ completionFunc = completeHandle
	_ completionFunc = new(handleTableCompleter).complete
	_ completionFunc = completeDriverType
)

// completeHandle is a completionFunc that suggests handles.
func completeHandle(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	rc := RunContextFrom(cmd.Context())

	handles := rc.Config.Sources.Handles()

	return handles, cobra.ShellCompDirectiveNoFileComp
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

// completeHandleOrTable is a completionFunc that suggests
// a handle ("@sakila_sl3"), table (".actor"),
// or handle.table ("@sakila_sl3.actor").
func completeHandleOrTable(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	rc := RunContextFrom(cmd.Context())
	ctx, cancelFn := context.WithTimeout(cmd.Context(), rc.Config.Defaults.ShellCompletionTimeout)
	defer cancelFn()

	handles := rc.Config.Sources.Handles()

	if toComplete == "" {
		// There's no input yet.
		// Therefore we want to return a union of all handles
		// plus the tables from the active source.
		activeSrc := rc.Config.Sources.Active()
		if activeSrc == nil {
			rc.Log.Debug("Active source is nil")
			return nil, cobra.ShellCompDirectiveError
		}

		tables, err := getTableNames(ctx, rc, activeSrc.Handle)
		if err != nil {
			rc.Log.Error(err)
			return nil, cobra.ShellCompDirectiveError
		}

		suggestions := handles
		for _, table := range tables {
			suggestions = append(suggestions, "."+table)
		}

		return suggestions, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	}

	// There's some input. We expect the input to be of the
	// the form "@handle" or ".table". That is, the input should
	// start with either '@' or '.'.
	switch toComplete[0] {
	default:
		// User input was something other than '@' or '.'
		return nil, cobra.ShellCompDirectiveError
	case '@':
		// We're dealing with a handle.

		// We could be dealing with a just the handle ("@sakila_sl3")
		// or a handle.table ("@sakila_sl3.actor").
		if strings.ContainsRune(toComplete, '.') {
			// It's a handle with at least a partial table,
			// such as "@sakila_sl3.fil"
			handle, partialTbl, err := source.ParseTableHandle(toComplete)
			if err != nil {
				rc.Log.Error(err)
				return nil, cobra.ShellCompDirectiveError
			}

			tables, err := getTableNames(ctx, rc, handle)
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

		// Else, we're dealing with a handle
		var matchingHandles []string
		for _, handle := range handles {
			if strings.HasPrefix(handle, toComplete) {
				matchingHandles = append(matchingHandles, handle)
			}
		}

		switch len(matchingHandles) {
		case 0:
			return nil, cobra.ShellCompDirectiveNoFileComp
		default:
			return matchingHandles, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
		case 1:
			// Only one handle match, so we will present that complete
			// handle, plus a suggestion for each of the tables
			// for that handle
		}

		tables, err := getTableNames(ctx, rc, matchingHandles[0])
		if err != nil {
			rc.Log.Error(err)
			return nil, cobra.ShellCompDirectiveError
		}

		suggestions := []string{matchingHandles[0]}
		for _, table := range tables {
			suggestions = append(suggestions, matchingHandles[0]+"."+table)
		}

		return suggestions, cobra.ShellCompDirectiveNoFileComp
	case '.':
		// We're dealing with a table, from the active source.
		// Continues below
	}

	activeSrc := rc.Config.Sources.Active()
	if activeSrc == nil {
		rc.Log.Debug("Active source is nil")
		return nil, cobra.ShellCompDirectiveError
	}
	tables, err := getTableNames(ctx, rc, activeSrc.Handle)
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

	return suggestions, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

// handleTableCompleter encapsulates completion of a handle
// ("@sakila_sl3"), table (".actor"), or handle.table
// ("@sakila_sl3.actor"). Its complete method is a completionFunc.
type handleTableCompleter struct {
	// onlySQL, when true, filters out non-SQL sources.
	onlySQL bool
}

func (c *handleTableCompleter) complete(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	rc := RunContextFrom(cmd.Context())
	if err := rc.init(); err != nil {
		rc.Log.Error(err)
		return nil, cobra.ShellCompDirectiveError
	}

	ctx, cancelFn := context.WithTimeout(cmd.Context(), rc.Config.Defaults.ShellCompletionTimeout)
	defer cancelFn()

	if toComplete == "" {
		return c.completeEither(ctx, rc, args, toComplete)
	}

	// There's some input. We expect the input to be of the
	// the form "@handle" or ".table". That is, the input should
	// start with either '@' or '.'.
	switch toComplete[0] {
	default:
		// User input was something other than '@' or '.'
		return nil, cobra.ShellCompDirectiveError
	case '@':
		return c.completeHandle(ctx, rc, args, toComplete)
	case '.':
		return c.completeTableOnly(ctx, rc, args, toComplete)
	}
}

func (c *handleTableCompleter) completeTableOnly(ctx context.Context, rc *RunContext, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	activeSrc := rc.Config.Sources.Active()
	if activeSrc == nil {
		rc.Log.Debug("Active source is nil")
		return nil, cobra.ShellCompDirectiveError
	}

	if c.onlySQL {
		isSQL, err := handleIsSQL(rc, activeSrc.Handle)
		if err != nil {
			rc.Log.Error(err)
			return nil, cobra.ShellCompDirectiveError
		}
		if !isSQL {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}

	tables, err := getTableNames(ctx, rc, activeSrc.Handle)
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

	return suggestions, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

func (c *handleTableCompleter) completeHandle(ctx context.Context, rc *RunContext, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// We're dealing with a handle.

	// But we could be dealing with just the handle ("@sakila_sl3")
	// or a handle.table ("@sakila_sl3.actor").
	if strings.ContainsRune(toComplete, '.') {
		// It's a handle with a full handle and at least a
		// partial table name, such as "@sakila_sl3.fil"
		handle, partialTbl, err := source.ParseTableHandle(toComplete)
		if err != nil {
			rc.Log.Error(err)
			return nil, cobra.ShellCompDirectiveError
		}

		if c.onlySQL {
			isSQL, err := handleIsSQL(rc, handle)
			if err != nil {
				rc.Log.Error(err)
				return nil, cobra.ShellCompDirectiveError
			}

			if !isSQL {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
		}

		tables, err := getTableNames(ctx, rc, handle)
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
				isSQL, err := handleIsSQL(rc, handle)
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
	case 0:
		return nil, cobra.ShellCompDirectiveNoFileComp
	default:
		return matchingHandles, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	case 1:
		// Only one handle match, so we will present that complete
		// handle, plus a suggestion for each of the tables
		// for that handle
	}

	tables, err := getTableNames(ctx, rc, matchingHandles[0])
	if err != nil {
		rc.Log.Error(err)
		return nil, cobra.ShellCompDirectiveError
	}

	suggestions := []string{matchingHandles[0]}
	for _, table := range tables {
		suggestions = append(suggestions, matchingHandles[0]+"."+table)
	}

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func (c *handleTableCompleter) completeEither(ctx context.Context, rc *RunContext, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// There's no input yet.
	// Therefore we want to return a union of all handles
	// plus the tables from the active source.
	activeSrc := rc.Config.Sources.Active()
	if activeSrc == nil {
		rc.Log.Debug("Active source is nil")
		return nil, cobra.ShellCompDirectiveError
	}

	var activeSrcTables []string
	isSQL, err := handleIsSQL(rc, activeSrc.Handle)
	if err != nil {
		rc.Log.Error(err)
		return nil, cobra.ShellCompDirectiveError
	}

	if !c.onlySQL || isSQL {
		activeSrcTables, err = getTableNames(ctx, rc, activeSrc.Handle)
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
			isSQL, err = handleIsSQL(rc, src.Handle)
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

func handleIsSQL(rc *RunContext, handle string) (bool, error) {
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

func getTableNames(ctx context.Context, rc *RunContext, handle string) ([]string, error) {
	if err := rc.init(); err != nil {
		return nil, err
	}

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
