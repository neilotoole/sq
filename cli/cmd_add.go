package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/options"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/stringz"
)

func newSrcAddCmd() (*cobra.Command, runFunc) {
	cmd := &cobra.Command{
		Use: "add [--driver=TYPE] [--handle=@HANDLE] LOCATION",
		Example: `  # add a Postgres source; will have generated handle @sakila_pg
  sq add 'postgres://user:pass@localhost/sakila?sslmode=disable'
  
  # same as above, but explicitly setting flags
  sq add --handle=@sakila_pg --driver=postgres 'postgres://user:pass@localhost/sakila?sslmode=disable'

  # same as above, but with short flags
  sq add -h @sakila_pg --d postgres 'postgres://user:pass@localhost/sakila?sslmode=disable'

  # add a SQL Server source; will have generated handle @sakila_mssql
  sq add 'sqlserver://user:pass@localhost?database=sakila' 
  
  # add a sqlite db
  sq add ./testdata/sqlite1.db

  # add an Excel spreadsheet, with options
  sq add ./testdata/test1.xlsx --opts=header=true
  
  # add a CSV source, with options
  sq add ./testdata/person.csv --opts='header=true'

  # add a CSV source from a server (will be downloaded)
  sq add https://sq.io/testdata/actor.csv
`,
		Short: "Add data source",
		Long: `Add data source specified by LOCATION and optionally identified by @HANDLE.
The format of LOCATION varies, but is generally a DB connection string, a
file path, or a URL.

  DRIVER://USER:PASS@HOST:PORT/DBNAME
  /path/to/local/file.ext
  https://sq.io/data/test1.xlsx

If flag --handle is omitted, sq will generate a handle based
on LOCATION and the source driver type.

If flag --driver is omitted, sq will attempt to determine the
type from LOCATION via file suffix, content type, etc.. If the result
is ambiguous, specify the driver tye type via flag --driver.

Flag --opts sets source specific options. Generally opts are relevant
to document source types (such as a CSV file). The most common
use is to specify that the document has a header row:

  sq add actor.csv --opts=header=true

Available source driver types can be listed via "sq drivers".

At a minimum, the following drivers are bundled:

  sqlite3      SQLite3
  postgres     Postgres
  sqlserver    Microsoft SQL Server
  xlsx         Microsoft Excel XLSX
  mysql        MySQL
  csv          Comma-Separated Values
  tsv          Tab-Separated Values
`,
	}

	cmd.Flags().StringP(flagDriver, flagDriverShort, "", flagDriverUsage)
	cmd.Flags().StringP(flagSrcOptions, "", "", flagSrcOptionsUsage)
	cmd.Flags().StringP(flagHandle, flagHandleShort, "", flagHandleUsage)
	return cmd, execSrcAdd
}

func execSrcAdd(rc *RunContext, cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errz.Errorf(msgInvalidArgs)
	}

	cfg := rc.Config

	loc := source.AbsLocation(strings.TrimSpace(args[0]))

	var err error
	var typ source.Type
	if cmd.Flags().Changed(flagDriver) {
		val, _ := cmd.Flags().GetString(flagDriver)
		typ = source.Type(strings.TrimSpace(val))
		if !rc.reg.HasProviderFor(typ) {
			return errz.Errorf("unsupported source driver type %q", val)
		}
	} else {
		typ, err = rc.files.Type(rc.Context, loc)
		if err != nil {
			return errz.Errorf("unable to determine source driver type: use --driver flag")
		}
	}

	var handle string
	if cmd.Flags().Changed(flagHandle) {
		handle, _ = cmd.Flags().GetString(flagHandle)
	} else {
		handle, err = source.SuggestHandle(typ, loc, cfg.Sources.Exists)
		if err != nil {
			return errz.Wrap(err, "unable to suggest a handle: use --handle flag")
		}
	}

	if stringz.InSlice(source.ReservedHandles(), handle) {
		return errz.Errorf("handle reserved for system use: %s", handle)
	}

	err = source.VerifyLegalHandle(handle)
	if err != nil {
		return err
	}

	if cfg.Sources.Exists(handle) {
		return errz.Errorf("source handle already exists: %s", handle)
	}

	var opts options.Options
	if cmd.Flags().Changed(flagSrcOptions) {
		val, _ := cmd.Flags().GetString(flagSrcOptions)
		val = strings.TrimSpace(val)
		if val != "" {
			opts, err = options.ParseOptions(val)
			if err != nil {
				return err
			}
		}
	}

	// Special handling for SQLite, because it's a file-based SQL DB
	// unlike the other SQL DBs sq supports so far.
	// Both of these forms are allowed:
	//
	//  sq add sqlite3:///path/to/sakila.db
	//  sq add /path/to/sakila.db
	//
	// The second form is particularly nice for bash completion etc.
	if typ == sqlite3.Type {
		if !strings.HasPrefix(loc, sqlite3.Prefix) {
			loc = sqlite3.Prefix + loc
		}
	}

	src, err := newSource(rc.Log, rc.registry(), typ, handle, loc, opts)
	if err != nil {
		return err
	}

	err = cfg.Sources.Add(src)
	if err != nil {
		return err
	}

	if cfg.Sources.Active() == nil {
		// If no current active data source, use this one.
		_, err = cfg.Sources.SetActive(src.Handle)
		if err != nil {
			return err
		}
	}

	drvr, err := rc.registry().DriverFor(src.Type)
	if err != nil {
		return err
	}

	// TODO: should we really be pinging this src right now?
	err = drvr.Ping(rc.Context, src)
	if err != nil {
		return errz.Wrapf(err, "failed to ping %s [%s]", src.Handle, src.RedactedLocation())
	}

	err = rc.ConfigStore.Save(rc.Config)
	if err != nil {
		return err
	}

	return rc.writers().srcw.Source(src)
}
