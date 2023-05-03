package cli

import (
	"github.com/neilotoole/sq/cli/flag"
	"github.com/spf13/cobra"

	// Import the providers package to initialize provider implementations.
	_ "github.com/neilotoole/sq/drivers"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   `sq QUERY`,
		Short: "sq",
		Long: `sq is a swiss-army knife for wrangling data.

Use sq to query Postgres, SQLite, SQLServer, MySQL, CSV, Excel, etc,
and output in text, JSON, CSV, Excel and so on, or
write output to a database table.

You can query using sq's own jq-like syntax, or in native SQL.

Use "sq inspect" to view schema metadata. Use the "sq tbl" commands
to copy, truncate and drop tables.

See docs and more: https://sq.io`,
		Example: `  # pipe an Excel file and output the first 10 rows from sheet1
  $ cat data.xlsx | sq '.sheet1 | .[0:10]'

  # add Postgres source identified by handle @sakila_pg
  $ sq add --handle=@sakila_pg 'postgres://user:pass@localhost:5432/sakila'

  # add SQL Server source; will have generated handle @sakila_mssql
  $ sq add 'sqlserver://user:pass@localhost?database=sakila'

  # list available data sources
  $ sq ls

  # ping all data sources
  $ sq ping all

  # set active data source
  $ sq src @sakila_pg

  # get specified cols from table address in active data source
  $ sq '.address |  .address_id, .city, .country'

  # get metadata (schema, stats etc) for data source
  $ sq inspect @sakila_pg

  # get metadata for a table
  $ sq inspect @pg1.person

  # output in JSON
  $ sq -j '.person | .uid, .username, .email'

  # output in text format (with header)
  $ sq -th '.person | .uid, .username, .email'

  # output in text format (no header)
  $ sq -t '.person | .uid, .username, .email'

  # output to a HTML file
  $ sq --html '@sakila_sl3.actor' -o actor.html

  # join across data sources
  $ sq '@my1.person, @pg1.address | join(.uid) | .username, .email, .city'

  # insert query results into a table in another data source
  $ sq --insert=@pg1.person '@my1.person | .username, .email'

  # execute a database-native SQL query, specifying the source
  $ sq sql --src=@pg1 'SELECT uid, username, email FROM person LIMIT 2'

  # copy a table (in the same source)
  $ sq tbl copy @sakila_sl3.actor .actor2

  # truncate tables
  $ sq tbl truncate @sakila_sl3.actor2

  # drop table
  $ sq tbl drop @sakila_sl3.actor2`,
	}

	// The --help flag must be explicitly added to rootCmd,
	// or else cobra tries to do its own (unwanted) thing.
	// The behavior of cobra in this regard seems to have
	// changed? This particular incantation currently does the trick.
	cmd.PersistentFlags().Bool(flag.Help, false, "Show help")

	addQueryCmdFlags(cmd)
	cmd.Flags().Bool(flag.Version, false, flag.VersionUsage)

	cmd.PersistentFlags().BoolP(flag.Text, flag.TextShort, false, flag.TextUsage)
	cmd.PersistentFlags().BoolP(flag.Header, flag.HeaderShort, true, flag.HeaderUsage)
	cmd.PersistentFlags().BoolP(flag.NoHeader, flag.NoHeaderShort, false, flag.NoHeaderUsage)
	cmd.MarkFlagsMutuallyExclusive(flag.Header, flag.NoHeader)
	cmd.PersistentFlags().BoolP(flag.Monochrome, flag.MonochromeShort, false, flag.MonochromeUsage)
	cmd.PersistentFlags().BoolP(flag.Verbose, flag.VerboseShort, false, flag.VerboseUsage)
	cmd.PersistentFlags().String(flag.Config, "", flag.ConfigUsage)
	return cmd
}
