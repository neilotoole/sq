package cli

import (
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output/format"
	"github.com/neilotoole/sq/cli/pprofile"
	_ "github.com/neilotoole/sq/drivers" // Load drivers
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   `sq QUERY`,
		Short: "sq",
		Long: `sq is a swiss-army knife for wrangling data.

  $ sq '@sakila_pg | .actor | where(.actor_id > 2) | .first_name, .last_name | .[0:10]'

Use sq to query Postgres, SQLite, SQLServer, MySQL, CSV, Excel, etc,
and output in text, JSON, CSV, Excel and so on, or write output to a
database table.

You can query using sq's own jq-like syntax, or in native SQL.

Use "sq inspect" to view schema metadata. Use the "sq tbl" commands
to copy, truncate and drop tables. Use "sq diff" to compare source
metadata and row data.

See docs and more: https://sq.io`,
		Example: `  # Add Postgres source.
  $ sq add postgres://user@localhost/sakila -p
  Password: ****

  # List available data sources.
  $ sq ls

  # Set active data source.
  $ sq src @sakila_pg

  # Get specified cols from table address in active data source.
  $ sq '.actor | .actor_id, .first_name, .last_name'

  # Ping a data source.
  $ sq ping @sakila_pg

  # View metadata (schema, stats etc) for data source.
  $ sq inspect @sakila_pg

  # View metadata for a table.
  $ sq inspect @sakila_pg.actor

  # Output all rows from 'actor' table in JSON.
  $ sq -j .actor

  # Output in text format (with header).
  $ sq -th .actor

  # Output in text format (no header).
  $ sq -tH .actor

  # Output to a HTML file.
  $ sq --html '@sakila_pg.actor' -o actor.html

  # Join across data sources.
  $ sq '@my1.person, @pg1.address | join(.uid) | .username, .email, .city'

  # Insert query results into a table in another data source.
  $ sq --insert=@pg1.person '@my1.person | .username, .email'

  # Execute a database-native SQL query, specifying the source.
  $ sq sql --src=@pg1 'SELECT uid, username, email FROM person LIMIT 2'

  # Copy a table (in the same source).
  $ sq tbl copy @sakila_pg.actor .actor2

  # Truncate table.
  $ sq tbl truncate @sakila_pg.actor2

  # Drop table.
  $ sq tbl drop @sakila_pg.actor2

  # Pipe an Excel file and output the first 10 rows from sheet1
  $ cat data.xlsx | sq '.sheet1 | .[0:10]'`,
	}

	cmd.Flags().SortFlags = false
	cmd.PersistentFlags().SortFlags = false
	cmd.CompletionOptions.DisableDescriptions = true

	// The --help flag must be explicitly added to rootCmd,
	// or else cobra tries to do its own (unwanted) thing.
	// The behavior of cobra in this regard seems to have
	// changed? This particular incantation currently does the trick.
	cmd.PersistentFlags().Bool(flag.Help, false, "Show help")

	addQueryCmdFlags(cmd)

	// --render-sql is an slq-only flag, but mirror it on the root cmd
	// so it shows up in `sq --help` (the slq subcommand is hidden, so
	// the slq registration alone isn't surfaced). `sq sql` still rejects
	// the flag because it's not added to the sql subcommand.
	cmd.Flags().Bool(flag.RenderSQL, false, flag.RenderSQLUsage)

	cmd.Flags().Bool(flag.Version, false, flag.VersionUsage)

	addOptionFlag(cmd.PersistentFlags(), OptMonochrome)
	addOptionFlag(cmd.PersistentFlags(), OptProgress)
	// --reveal is the canonical disclosure flag; --no-redact is its
	// deprecated alias. Neither is bound directly to OptSecretsReveal.
	// Both are free-standing pflags whose presence is detected in
	// getOptionsFromFlags and unioned into secrets.reveal=true.
	// Explicit false (--reveal=false, --no-redact=false) is a no-op:
	// the flags are positive opt-ins to disclosure, and a leftover
	// config or default value wins. To force redaction when
	// secrets.reveal is true in config, override it via
	// 'sq config set secrets.reveal false'.
	cmd.PersistentFlags().Bool(flag.Reveal, false, flag.RevealUsage)
	cmd.PersistentFlags().Bool(flag.NoRedact, false, flag.NoRedactUsage)
	// --expand resolves ${scheme:path} placeholders against the
	// registered resolvers (keyring, env, file). It applies to every
	// command that prints a source location: sq src, sq ls, sq inspect,
	// sq add and sq mv (post-action echo), and sq ping in JSON/YAML
	// output (the text/CSV ping output does not include Location).
	//
	// On the display-expansion step itself, per-source resolver failure
	// is lenient: the placeholder is left verbatim for that source and
	// the listing continues. Connection-time resolution is independent;
	// commands that have to connect (sq inspect, sq ping) will still
	// error if a missing secret prevents the connection. On
	// `sq config export`, --expand keeps its existing strict-abort
	// behavior because an export is a snapshot for transfer.
	//
	// Not bound to a config option: persisting --expand as a workflow
	// preference would resolve every placeholder on every display
	// command, defeating the reason the user put the placeholder there.
	// Like --reveal, explicit --expand=false is a no-op.
	cmd.PersistentFlags().Bool(flag.Expand, false, flag.ExpandUsage)
	addOptionFlag(cmd.PersistentFlags(), OptVerbose)
	addOptionFlag(cmd.PersistentFlags(), pprofile.OptMode)
	panicOn(cmd.RegisterFlagCompletionFunc(pprofile.OptMode.Flag().Name, completeStrings(
		-1,
		pprofile.Modes()...,
	)))

	// flag.Config can't use the option flag mechanism, because... well,
	// because it's the config flag, and it exists above the realm of
	// options. It's the flag that tells us where to find the config file,
	// thus it can't be an option stored in the config file.
	cmd.PersistentFlags().String(flag.Config, "", flag.ConfigUsage)

	addOptionFlag(cmd.PersistentFlags(), OptLogEnabled)
	panicOn(cmd.RegisterFlagCompletionFunc(OptLogEnabled.Flag().Name, completeBool))

	addOptionFlag(cmd.PersistentFlags(), OptLogFile)

	addOptionFlag(cmd.PersistentFlags(), OptLogLevel)
	panicOn(cmd.RegisterFlagCompletionFunc(OptLogLevel.Flag().Name, completeStrings(
		1,
		slog.LevelDebug.String(),
		slog.LevelInfo.String(),
		slog.LevelWarn.String(),
		slog.LevelError.String(),
	)))

	addOptionFlag(cmd.PersistentFlags(), OptLogFormat)
	panicOn(cmd.RegisterFlagCompletionFunc(OptLogFormat.Flag().Name, completeStrings(
		1,
		string(format.Text),
		string(format.JSON),
	)))

	addOptionFlag(cmd.PersistentFlags(), OptErrorFormat)
	addOptionFlag(cmd.PersistentFlags(), OptErrorStack)

	return cmd
}
