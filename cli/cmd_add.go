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
		Use:   "add [--handle=@HANDLE] [--password] [--driver=TYPE][--opts=a=b] LOCATION",
		RunE:  execSrcAdd,
		Short: "Add data source",
		Long: `Add data source specified by LOCATION and optionally identified by @HANDLE.
The format of LOCATION varies, but is generally a DB connection string, a
file path, or a URL.

  DRIVER://USER:PASS@HOST:PORT/DBNAME
  /path/to/local/file.ext
  https://sq.io/data/test1.xlsx

If flag --handle is omitted, sq will generate a handle based
on LOCATION and the source driver type.

If flag --password (-p) is set, the user will be prompted for
the data source password, or it will be read from stdin.

  $ sq add 'postgres://user@localhost/sakila' -p
  Password: ****

If flag --driver is omitted, sq will attempt to determine the
type from LOCATION via file suffix, content type, etc.. If the result
is ambiguous, specify the driver type via flag --driver.

Flag --opts sets source-specific options. Generally opts are relevant
to document source types (such as a CSV file). The most common
use is to specify that the document has a header row:

  $ sq add actor.csv --opts=header=true

Available source driver types can be listed via "sq driver ls".

At a minimum, the following drivers are bundled:

  sqlite3    SQLite                               
  postgres   PostgreSQL                           
  sqlserver  Microsoft SQL Server                 
  mysql      MySQL                                
  csv        Comma-Separated Values               
  tsv        Tab-Separated Values                 
  json       JSON                                 
  jsona      JSON Array: LF-delimited JSON arrays 
  jsonl      JSON Lines: LF-delimited JSON objects
  xlsx       Microsoft Excel XLSX                  
`,

		Example: `  # add a Postgres source; will have generated handle @sakila_pg
  $ sq add 'postgres://user:pass@localhost/sakila'
  
  # Add a source, but prompt user for password
  $ sq add 'postgres://user@localhost/sakila' -p
  Password: ****

  # Add a source, but read password from an environment variable
  $ export PASSWORD='open:;"_Ses@me'
  $ sq add 'postgres://user@localhost/sakila' -p <<< $PASSWORD

  # Same as above, but read password from file
  $ echo 'open:;"_Ses@me' > password.txt
  $ sq add 'postgres://user@localhost/sakila' -p < password.txt

  # Explicitly set flags
  $ sq add --handle=@sakila_pg --driver=postgres 'postgres://user:pass@localhost/sakila'

  # Same as above, but with short flags
  $ sq add -h @sakila_pg --d postgres 'postgres://user:pass@localhost/sakila'

  # Add a SQL Server source; will have generated handle @sakila_mssql or similar
  $ sq add 'sqlserver://user:pass@localhost?database=sakila' 
  
  # Add a sqlite db
  $ sq add ./testdata/sqlite1.db

  # Add an Excel spreadsheet, with options
  $ sq add ./testdata/test1.xlsx --opts=header=true
  
  # Add a CSV source, with options
  $ sq add ./testdata/person.csv --opts=header=true

  # Add a CSV source from a URL (will be downloaded)
  $ sq add https://sq.io/testdata/actor.csv
`,
	}

	cmd.Flags().StringP(flagDriver, flagDriverShort, "", flagDriverUsage)
	_ = cmd.RegisterFlagCompletionFunc(flagDriver, completeDriverType)
	cmd.Flags().StringP(flagSrcOptions, "", "", flagSrcOptionsUsage)
	cmd.Flags().StringP(flagHandle, flagHandleShort, "", flagHandleUsage)
	cmd.Flags().BoolP(flagPasswordPrompt, flagPasswordPromptShort, false, flagPasswordPromptUsage)
	return cmd
}

func execSrcAdd(cmd *cobra.Command, args []string) error {
	rc := RunContextFrom(cmd.Context())
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
		return errz.Errorf("unsupported source type %q", typ)
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
	//  $ sq add sqlite3:///path/to/sakila.db
	//  $ sq add /path/to/sakila.db
	//
	// The second form is particularly nice for bash completion etc.
	if typ == sqlite3.Type {
		if !strings.HasPrefix(loc, sqlite3.Prefix) {
			loc = sqlite3.Prefix + loc
		}
	}

	// If the -p flag is set, we look for password input in
	// one of two ways:
	//
	//  $ sq add postgres://sakila@localhost/sakila < passwd.txt
	if cmdFlagTrue(cmd, flagPasswordPrompt) {
		passwd, err := readPassword(cmd.Context(), rc.Stdin, rc.Out, rc.writers.fm)
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

	if cfg.Sources.Active() == nil {
		// If no current active data source, use this one.
		_, err = cfg.Sources.SetActive(src.Handle)
		if err != nil {
			return err
		}
	}

	drvr, err := rc.registry.DriverFor(src.Type)
	if err != nil {
		return err
	}

	// TODO: should we really be pinging this src right now?
	err = drvr.Ping(cmd.Context(), src)
	if err != nil {
		return errz.Wrapf(err, "failed to ping %s [%s]", src.Handle, src.RedactedLocation())
	}

	err = rc.ConfigStore.Save(rc.Config)
	if err != nil {
		return err
	}

	return rc.writers.srcw.Source(src)
}

// readPassword reads a password from stdin pipe, or if nothing on stdin,
// it prints a prompt to stdout, and then accepts input (which must be
// followed by a return).
func readPassword(ctx context.Context, stdin *os.File, stdout io.Writer, fm *output.Formatting) ([]byte, error) {
	resultCh := make(chan []byte, 0)
	errCh := make(chan error, 0)

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
