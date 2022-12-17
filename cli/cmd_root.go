package cli

import (
	"github.com/spf13/cobra"

	// Import the providers package to initialize provider implementations
	_ "github.com/neilotoole/sq/drivers"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   `sq QUERY`,
		Short: "sq",
		Long: `sq is a swiss army knife for data.

Use sq to query Postgres, SQLite, SQLServer, MySQL, CSV, TSV
and Excel, and output in text, JSON, CSV, Excel, HTML, etc., or
output to a database table.

You can query using sq's own jq-like syntax, or in native SQL.

Execute "sq completion --help" for instructions to install shell completion.

More at https://sq.io
`,
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

  # output in table format (with header)
  $ sq -th '.person | .uid, .username, .email'

  # output in table format (no header)
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
  $ sq tbl drop @sakila_sl3.actor2
`,
	}

	addQueryCmdFlags(cmd)
	cmd.Flags().Bool(flagVersion, false, flagVersionUsage)
	cmd.PersistentFlags().BoolP(flagMonochrome, flagMonochromeShort, false, flagMonochromeUsage)
	cmd.PersistentFlags().BoolP(flagVerbose, flagVerboseShort, false, flagVerboseUsage)
	return cmd
}
