package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/neilotoole/sq/cli/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/options"
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
  $ sq add -h @sakila/pg 'postgres://user:pass@localhost/sakila'

This handle format "@sakila/pg" includes a group, "sakila". Using a group
is entirely optional: it is a way to organize sources. For example:

  $ sq add -h @dev/pg 'postgres://user:pass@dev.db.example.com/sakila'
  $ sq add -h @prod/pg 'postgres://user:pass@prod.db.acme.com/sakila'

The format of LOCATION is driver-specific,but is generally a DB connection
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

Flag --opts sets source-specific options. Generally, opts are relevant
to document source types (such as a CSV file). The most common
use is to specify that the document has a header row:

  $ sq add actor.csv --opts=header=true

Use query string encoding for multiple options, e.g. "--opts a=b&x=y".

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
active source (and its group becomes the active group). Otherwise you can
use flag --active to make the new source (and its group) active.

More examples:

  # Add a source, but prompt user for password
  $ sq add 'postgres://user@localhost/sakila' -p
  Password: ****

  # Explicitly set flags
  $ sq add --handle=@sakila_pg --driver=postgres 'postgres://user:pass@localhost/sakila'

  # Same as above, but with short flags
  $ sq add -h @sakila_pg --d postgres 'postgres://user:pass@localhost/sakila'

  # Add a SQL Server source; will have generated handle @sakila_mssql or similar
  $ sq add 'sqlserver://user:pass@localhost?database=sakila' 

  # Add a sqlite db, and immediately make it the active source
  $ sq add --active ./testdata/sqlite1.db

  # Add an Excel spreadsheet, with options
  $ sq add ./testdata/test1.xlsx --opts=header=true
  
  # Add a CSV source, with options
  $ sq add ./testdata/person.csv --opts=header=true

  # Add a CSV source from a URL (will be downloaded)
  $ sq add https://sq.io/testdata/actor.csv

  # Add a source, and make it the active source (and group)
  $ sq add ./actor.csv -h @csv/actor`,
		Short: "Add data source",
		Long:  `Add data source specified by LOCATION, optionally identified by @HANDLE.`,
	}

	cmd.Flags().StringP(flagDriver, flagDriverShort, "", flagDriverUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flagDriver, completeDriverType))
	cmd.Flags().StringP(flagSrcOptions, "", "", flagSrcOptionsUsage)
	cmd.Flags().StringP(flagHandle, flagHandleShort, "", flagHandleUsage)
	cmd.Flags().BoolP(flagPasswordPrompt, flagPasswordPromptShort, false, flagPasswordPromptUsage)
	cmd.Flags().Bool(flagSkipVerify, false, flagSkipVerifyUsage)
	cmd.Flags().BoolP(flagJSON, flagJSONShort, false, flagJSONUsage)
	cmd.Flags().BoolP(flagAddActive, flagAddActiveShort, false, flagAddActiveUsage)
	return cmd
}

func execSrcAdd(cmd *cobra.Command, args []string) error {
	rc := RunContextFrom(cmd.Context())
	cfg := rc.Config

	loc := source.AbsLocation(strings.TrimSpace(args[0]))
	var err error
	var typ source.Type

	if cmdFlagChanged(cmd, flagDriver) {
		val, _ := cmd.Flags().GetString(flagDriver)
		typ = source.Type(strings.TrimSpace(val))
	} else {
		typ, err = rc.files.Type(cmd.Context(), loc)
		if err != nil {
			return err
		}
		if typ == source.TypeNone {
			return errz.Errorf("unable to determine source type: use --driver flag")
		}
	}

	if rc.registry.ProviderFor(typ) == nil {
		return errz.Errorf("unsupported source type {%s}", typ)
	}

	var handle string
	if cmdFlagChanged(cmd, flagHandle) {
		handle, _ = cmd.Flags().GetString(flagHandle)
	} else {
		handle, err = source.SuggestHandle(rc.Config.Sources, typ, loc)
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

	if cfg.Sources.Exists(handle) {
		return errz.Errorf("source handle already exists: %s", handle)
	}

	var opts options.Options
	if cmdFlagChanged(cmd, flagSrcOptions) {
		val, _ := cmd.Flags().GetString(flagSrcOptions)
		val = strings.TrimSpace(val)
		if val != "" {
			opts, err = options.ParseOptions(val)
			if err != nil {
				return err
			}
		}
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
	if cmdFlagTrue(cmd, flagPasswordPrompt) {
		var passwd []byte
		passwd, err = readPassword(cmd.Context(), rc.Stdin, rc.Out, rc.writers.fm)
		if err != nil {
			return err
		}

		loc, err = source.LocationWithPassword(loc, string(passwd))
		if err != nil {
			return err
		}
	}

	src, err := newSource(rc.Log, rc.registry, typ, handle, loc, opts)
	if err != nil {
		return err
	}

	err = cfg.Sources.Add(src)
	if err != nil {
		return err
	}

	if cfg.Sources.Active() == nil || cmdFlagTrue(cmd, flagAddActive) {
		// If no current active data source, use this one, OR if
		// flagAddActive is true.
		if _, err = cfg.Sources.SetActive(src.Handle, false); err != nil {
			return err
		}

		// Likewise with the group.
		if err = cfg.Sources.SetActiveGroup(src.Group()); err != nil {
			return err
		}
	}

	drvr, err := rc.registry.DriverFor(src.Type)
	if err != nil {
		return err
	}

	if !cmdFlagTrue(cmd, flagSkipVerify) {
		// Typically we want to ping the source before adding it.
		if err = drvr.Ping(cmd.Context(), src); err != nil {
			return err
		}
	}

	if err = rc.ConfigStore.Save(rc.Config); err != nil {
		return err
	}

	return rc.writers.srcw.Source(src)
}

// readPassword reads a password from stdin pipe, or if nothing on stdin,
// it prints a prompt to stdout, and then accepts input (which must be
// followed by a return).
func readPassword(ctx context.Context, stdin *os.File, stdout io.Writer, fm *output.Formatting) ([]byte, error) {
	resultCh := make(chan []byte)
	errCh := make(chan error)

	// Check if there is something to read on STDIN.
	stat, _ := stdin.Stat()
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
		fm.Faint.Fprint(buf, "[ENTER]")
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
