package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/neilotoole/sq/drivers/csv"

	"github.com/neilotoole/sq/cli/flag"

	"github.com/neilotoole/sq/cli/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
)

func newSrcAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "add [--handle @HANDLE] LOCATION",
		RunE: execSrcAdd,
		Args: cobra.ExactArgs(1),
		Example: `
When adding a data source, LOCATION is the only required arg.

  $ sq add ./actor.csv
  @actor  csv  actor.csv

Note that sq generated the handle "@actor". But you can explicitly specify
a handle.

  # Add a postgres source with handle "@sakila/pg"
  $ sq add --handle @sakila/pg 'postgres://user:pass@localhost/sakila'

This handle format "@sakila/pg" includes a group, "sakila". Using a group
is entirely optional: it is a way to organize sources. For example:

  $ sq add --handle @dev/pg 'postgres://user:pass@dev.db.example.com/sakila'
  $ sq add --handle @prod/pg 'postgres://user:pass@prod.db.acme.com/sakila'

The format of LOCATION is driver-specific, but is generally a DB connection
string, a file path, or a URL.

  DRIVER://USER:PASS@HOST:PORT/DBNAME
  /path/to/local/file.ext
  https://sq.io/data/test1.xlsx

If flag --handle is omitted, sq will generate a handle based
on LOCATION and the source driver type.

It's a security hazard to expose the data source password via
the LOCATION string. If flag --password (-p) is set, sq prompt the
user for the password:

  $ sq add 'postgres://user@localhost/sakila' -p
  Password: ****

However, if there's input on stdin, sq will read the password from
there instead of prompting the user:

  # Add a source, but read password from an environment variable
  $ export PASSWD='open:;"_Ses@me'
  $ sq add 'postgres://user@localhost/sakila' -p <<< $PASSWD

  # Same as above, but instead read password from file
  $ echo 'open:;"_Ses@me' > password.txt
  $ sq add 'postgres://user@localhost/sakila' -p < password.txt

There are various driver-specific options available. For example:

  $ sq add actor.csv --ingest.header=false --driver.csv.delim=colon

If flag --driver is omitted, sq will attempt to determine the
type from LOCATION via file suffix, content type, etc.. If the result
is ambiguous, explicitly specify the driver type.

  $ sq add --driver=tsv ./mystery.data

Available source driver types can be listed via "sq driver ls". At a
minimum, the following drivers are bundled:

  sqlite3    SQLite
  postgres   PostgreSQL
  sqlserver  Microsoft SQL Server / Azure SQL Edge
  mysql      MySQL
  csv        Comma-Separated Values
  tsv        Tab-Separated Values
  json       JSON
  jsona      JSON Array: LF-delimited JSON arrays
  jsonl      JSON Lines: LF-delimited JSON objects
  xlsx       Microsoft Excel XLSX

If there isn't already an active source, the newly added source becomes the
active source (but the active group does not change). Otherwise you can
use flag --active to make the new source active.

More examples:

  # Add a source, but prompt user for password
  $ sq add 'postgres://user@localhost/sakila' -p
  Password: ****

  # Explicitly set flags
  $ sq add --handle @sakila_pg --driver postgres 'postgres://user:pass@localhost/sakila'

  # Same as above, but with short flags
  $ sq add -n @sakila_pg -d postgres 'postgres://user:pass@localhost/sakila'

  # Add a SQL Server source; will have generated handle @sakila
  $ sq add 'sqlserver://user:pass@localhost?database=sakila'

  # Add a sqlite db, and immediately make it the active source
  $ sq add ./testdata/sqlite1.db --active

  # Add an Excel spreadsheet, with options
  $ sq add ./testdata/test1.xlsx --ingest.header=true

  # Add a CSV source, with options
  $ sq add ./testdata/person.csv --ingest.header=true

  # Add a CSV source from a URL (will be downloaded)
  $ sq add https://sq.io/testdata/actor.csv

  # Add a source, and make it the active source (and group)
  $ sq add ./actor.csv --handle @csv/actor

  # Add a currently unreachable source
  $ sq add 'postgres://user:pass@db.offline.com/sakila' --skip-verify`,
		Short: "Add data source",
		Long:  `Add data source specified by LOCATION, optionally identified by @HANDLE.`,
	}

	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.Compact, flag.CompactShort, false, flag.CompactUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)

	cmd.Flags().StringP(flag.AddDriver, flag.AddDriverShort, "", flag.AddDriverUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.AddDriver, completeDriverType))

	cmd.Flags().StringP(flag.Handle, flag.HandleShort, "", flag.HandleUsage)
	cmd.Flags().BoolP(flag.PasswordPrompt, flag.PasswordPromptShort, false, flag.PasswordPromptUsage)
	cmd.Flags().Bool(flag.SkipVerify, false, flag.SkipVerifyUsage)
	cmd.Flags().BoolP(flag.AddActive, flag.AddActiveShort, false, flag.AddActiveUsage)

	cmd.Flags().Bool(flag.IngestHeader, false, flag.IngestHeaderUsage)

	cmd.Flags().Bool(flag.CSVEmptyAsNull, true, flag.CSVEmptyAsNullUsage)
	cmd.Flags().String(flag.CSVDelim, flag.CSVDelimDefault, flag.CSVDelimUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.CSVDelim, completeStrings(-1, csv.NamedDelims()...)))

	return cmd
}

func execSrcAdd(cmd *cobra.Command, args []string) error {
	rc := RunContextFrom(cmd.Context())
	cfg := rc.Config

	loc := source.AbsLocation(strings.TrimSpace(args[0]))
	var err error
	var typ source.DriverType

	if cmdFlagChanged(cmd, flag.AddDriver) {
		val, _ := cmd.Flags().GetString(flag.AddDriver)
		typ = source.DriverType(strings.TrimSpace(val))
	} else {
		typ, err = rc.files.DriverType(cmd.Context(), loc)
		if err != nil {
			return err
		}
		if typ == source.TypeNone {
			return errz.Errorf("unable to determine driver type: use --driver flag")
		}
	}

	if rc.driverReg.ProviderFor(typ) == nil {
		return errz.Errorf("unsupported driver type {%s}", typ)
	}

	var handle string
	if cmdFlagChanged(cmd, flag.Handle) {
		handle, _ = cmd.Flags().GetString(flag.Handle)
	} else {
		handle, err = source.SuggestHandle(rc.Config.Collection, typ, loc)
		if err != nil {
			return errz.Wrap(err, "unable to suggest a handle: use --handle flag")
		}
	}

	if stringz.InSlice(source.ReservedHandles(), handle) {
		return errz.Errorf("handle reserved for system use: %s", handle)
	}

	if err = source.ValidHandle(handle); err != nil {
		return err
	}

	if cfg.Collection.IsExistingSource(handle) {
		return errz.Errorf("source handle already exists: %s", handle)
	}

	if typ == sqlite3.Type {
		// Special handling for SQLite, because it's a file-based DB.
		loc, err = sqlite3.MungeLocation(loc)
		if err != nil {
			return err
		}
	}

	// If the -p flag is set, sq looks for password input on stdin,
	// or sq prompts the user.
	if cmdFlagTrue(cmd, flag.PasswordPrompt) {
		var passwd []byte
		if passwd, err = readPassword(cmd.Context(), rc.Stdin, rc.Out, rc.writers.Printing); err != nil {
			return err
		}

		if loc, err = source.LocationWithPassword(loc, string(passwd)); err != nil {
			return err
		}
	}

	o, err := getSrcOptionsFromFlags(cmd.Flags(), rc.OptionsRegistry, typ)
	if err != nil {
		return err
	}

	src, err := newSource(
		cmd.Context(),
		rc.driverReg,
		typ,
		handle,
		loc,
		o,
	)
	if err != nil {
		return err
	}

	if err = cfg.Collection.Add(src); err != nil {
		return err
	}

	if cfg.Collection.Active() == nil || cmdFlagTrue(cmd, flag.AddActive) {
		// If no current active data source, use this one, OR if
		// flagAddActive is true.
		if _, err = cfg.Collection.SetActive(src.Handle, false); err != nil {
			return err
		}

		// However, we do not set the active group to be the new src's group.
		// In UX testing, it led to confused users.
	}

	drvr, err := rc.driverReg.DriverFor(src.Type)
	if err != nil {
		return err
	}

	if !cmdFlagTrue(cmd, flag.SkipVerify) {
		// Typically we want to ping the source before adding it.
		// But, sometimes not, for example if a source is temporarily offline.
		if err = drvr.Ping(cmd.Context(), src); err != nil {
			return err
		}
	}

	if err = rc.ConfigStore.Save(cmd.Context(), rc.Config); err != nil {
		return err
	}

	if src, err = rc.Config.Collection.Get(src.Handle); err != nil {
		return err
	}

	return rc.writers.Source.Source(rc.Config.Collection, src)
}

// readPassword reads a password from stdin pipe, or if nothing on stdin,
// it prints a prompt to stdout, and then accepts input (which must be
// followed by a return).
func readPassword(ctx context.Context, stdin *os.File, stdout io.Writer, pr *output.Printing) ([]byte, error) {
	resultCh := make(chan []byte)
	errCh := make(chan error)

	// Check if there is something to read on STDIN.
	stat, err := stdin.Stat()
	if err != nil {
		// Shouldn't happen
		return nil, errz.Err(err)
	}
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return nil, err
		}

		b = bytes.TrimSuffix(b, []byte("\n"))
		return b, nil
	}

	// Run this is a goroutine so that we can handle ctrl-c.
	go func() {
		buf := &bytes.Buffer{}
		fmt.Fprint(buf, "Password: ")
		pr.Faint.Fprint(buf, "[ENTER]")
		fmt.Fprint(buf, " ")
		stdout.Write(buf.Bytes())

		b, err := term.ReadPassword(int(stdin.Fd()))
		// Regardless of whether there's an error, we print
		// newline for presentation.
		fmt.Fprintln(stdout)
		if err != nil {
			errCh <- errz.Err(err)
			return
		}

		resultCh <- b
	}()

	select {
	case <-ctx.Done():
		// Print newline so that cancel msg is printed on its own line.
		fmt.Fprintln(stdout)
		return nil, errz.Err(ctx.Err())
	case err := <-errCh:
		return nil, err
	case b := <-resultCh:
		return b, nil
	}
}
