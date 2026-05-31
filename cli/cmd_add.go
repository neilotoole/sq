package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/drivers/csv"
	"github.com/neilotoole/sq/drivers/duckdb"
	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/core/secret/keyring"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/location"
)

func newSrcAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "add [--handle @HANDLE] LOCATION",
		RunE:              execSrcAdd,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeAddLocation,
		Example: `
When adding a data source, LOCATION is the only required arg.

  $ sq add ./actor.csv
  @actor  csv  actor.csv

Note that sq generated the handle "@actor". But you can explicitly specify
a handle.

  # Add a postgres source with handle "@sakila/pg"
  $ sq add --handle @sakila/pg postgres://user:pass@localhost/sakila

This handle format "@sakila/pg" includes a group, "sakila". Using a group
is entirely optional: it is a way to organize sources. For example:

  $ sq add --handle @dev/pg postgres://user:pass@dev.db.acme.com/sakila
  $ sq add --handle @prod/pg postgres://user:pass@prod.db.acme.com/sakila

The format of LOCATION is driver-specific, but is generally a DB connection
string, a file path, or a URL.

  DRIVER://USER:PASS@HOST:PORT/DBNAME?PARAM=VAL
  /path/to/local/file.ext
  https://sq.io/data/test1.xlsx

If LOCATION contains special shell characters, it's necessary to enclose
it in single quotes, or to escape the special character. For example,
note the "\?" in the unquoted location below.

  $ sq add postgres://user:pass@localhost/sakila\?sslmode=disable

A significant advantage of not quoting LOCATION is that sq provides extensive
shell completion when inputting the location value.

If flag --handle is omitted, sq will generate a handle based
on LOCATION and the source driver type.

It's a security hazard to expose the data source password via
the LOCATION string. If flag --password (-p) is set, sq prompt the
user for the password:

  $ sq add postgres://user@localhost/sakila -p
  Password: ****

However, if there's input on stdin, sq will read the password from
there instead of prompting the user:

  # Add a source, but read password from an environment variable
  $ export PASSWD='open:;"_Ses@me'
  $ sq add postgres://user@localhost/sakila -p <<< $PASSWD

  # Same as above, but instead read password from file
  $ echo 'open:;"_Ses@me' > password.txt
  $ sq add postgres://user@localhost/sakila -p < password.txt

There are various driver-specific options available. For example:

  $ sq add actor.csv --ingest.header=false --driver.csv.delim=colon

If flag --driver is omitted, sq will attempt to determine the
type from LOCATION via file suffix, content type, etc. If the result
is ambiguous, explicitly specify the driver type.

  $ sq add --driver=tsv ./mystery.data

Available source driver types can be listed via "sq driver ls". At a
minimum, the following drivers are bundled:

  sqlite3    SQLite
  postgres   PostgreSQL
  sqlserver  Microsoft SQL Server
  mysql      MySQL
  clickhouse ClickHouse
  oracle     Oracle Database (experimental)
  csv        Comma-Separated Values
  tsv        Tab-Separated Values
  json       JSON
  jsona      JSON Array: LF-delimited JSON arrays
  jsonl      JSON Lines: LF-delimited JSON objects
  xlsx       Microsoft Excel XLSX

DRIVER NOTES:

The clickhouse driver will automatically apply a default port if not
specified: 9000 for non-secure, or 9440 for secure (when "secure=true"
is in the connection string). This differs from the underlying clickhouse-go
library, which does not apply a default port.

If there isn't already an active source, the newly added source becomes the
active source (but the active group does not change). Otherwise you can
use flag --active to make the new source active.

More examples:

  # Add a source, but prompt user for password
  $ sq add postgres://user@localhost/sakila -p
  Password: ****

  # Explicitly set flags
  $ sq add --handle @sakila_pg --driver postgres postgres://user:pass@localhost/sakila

  # Same as above, but with short flags
  $ sq add -n @sakila_pg -d postgres postgres://user:pass@localhost/sakila

  # Specify some params (note escaped chars)
  $ sq add postgres://user:pass@localhost/sakila\?sslmode=disable\&application_name=sq

  # Specify some params, but use quoted string (no shell completion)
  $ sq add 'postgres://user:pass@localhost/sakila?sslmode=disable&application_name=sq''

  # Add a SQL Server source; will have generated handle @sakila
  $ sq add 'sqlserver://user:pass@localhost?database=sakila'

  # Add an Oracle source (experimental)
  $ sq add 'oracle://user:pass@localhost:1521/ORCLPDB1'

  # Add a SQLite DB, and immediately make it the active source
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
  $ sq add postgres://user:pass@db.offline.com/sakila --skip-verify`,
		Short: "Add data source",
		Long:  `Add data source specified by LOCATION, optionally identified by @HANDLE.`,
	}

	cmdMarkRequiresConfigLock(cmd)
	addTextFormatFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	addOptionFlag(cmd.Flags(), OptCompact)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)

	cmd.Flags().StringP(flag.AddDriver, flag.AddDriverShort, "", flag.AddDriverUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.AddDriver, completeDriverType))

	cmd.Flags().StringP(flag.Handle, flag.HandleShort, "", flag.HandleUsage)
	cmd.Flags().BoolP(flag.PasswordPrompt, flag.PasswordPromptShort, false, flag.PasswordPromptUsage)
	cmd.Flags().Bool(flag.Keyring, false, flag.KeyringUsage)
	cmd.Flags().Bool(flag.InlinePassword, false, flag.InlinePasswordUsage)
	cmd.MarkFlagsMutuallyExclusive(flag.Keyring, flag.InlinePassword)
	cmd.Flags().Bool(flag.SkipVerify, false, flag.SkipVerifyUsage)
	cmd.Flags().BoolP(flag.AddActive, flag.AddActiveShort, false, flag.AddActiveUsage)

	addOptionFlag(cmd.Flags(), driver.OptIngestHeader)
	addOptionFlag(cmd.Flags(), csv.OptEmptyAsNull)
	addOptionFlag(cmd.Flags(), csv.OptDelim)
	panicOn(cmd.RegisterFlagCompletionFunc(csv.OptDelim.Flag().Name, completeStrings(-1, csv.NamedDelims()...)))

	return cmd
}

func execSrcAdd(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	ru := run.FromContext(ctx)
	cfg := ru.Config

	loc := location.Abs(strings.TrimSpace(args[0]))
	var err error
	var typ drivertype.Type

	var handle string
	if cmdFlagChanged(cmd, flag.Handle) {
		handle, _ = cmd.Flags().GetString(flag.Handle)
	} else {
		handle, err = source.SuggestHandle(ru.Config.Collection, typ, loc)
		if err != nil {
			return errz.Wrap(err, "unable to suggest a handle: use --handle flag")
		}
	}

	if err = source.ValidHandle(handle); err != nil {
		return err
	}

	if stringz.InSlice(source.ReservedHandles(), handle) {
		return errz.Errorf("handle reserved for system use: %s", handle)
	}

	if cfg.Collection.IsExistingSource(handle) {
		return errz.Errorf("source handle already exists: %s", handle)
	}

	// A placeholder Location (e.g. "${env:DSN}" or "${op://...}") points
	// at an external store. sq does not own the value: --keyring (which
	// writes) and --inline-password (which expects an inline secret) are
	// both meaningless here. Reject early with a clear message.
	hasPlaceholder := strings.Contains(loc, "${")
	if hasPlaceholder {
		if cmdFlagIsSetTrue(cmd, flag.Keyring) {
			return errz.Errorf("--%s is not supported when the location is a ${...} placeholder", flag.Keyring)
		}
		if cmdFlagIsSetTrue(cmd, flag.InlinePassword) {
			return errz.Errorf("--%s is not supported when the location is a ${...} placeholder", flag.InlinePassword)
		}
		// Resolve relative ${file:...} paths against the current working
		// directory. The file resolver itself only accepts absolute paths
		// (or ~/...) — but at add time we know where the user is and can
		// capture that intent before persisting the placeholder. Other
		// schemes pass through unchanged.
		if loc, err = secret.RewritePlaceholders(ctx, loc, absolutizeFilePath); err != nil {
			return err
		}
	}

	if typ, err = resolveDriverType(ctx, ru, cmd, handle, loc, hasPlaceholder); err != nil {
		return err
	}

	if ru.DriverRegistry.ProviderFor(typ) == nil {
		return errz.Errorf("unsupported driver type {%s}", typ)
	}

	// File-path munging and password application only make sense for
	// non-placeholder Locations. A placeholder is opaque to sq at this
	// stage: nothing to munge, no inline password to relocate.
	if !hasPlaceholder {
		if typ == drivertype.SQLite {
			locBefore := loc
			// Special handling for SQLite, because it's a file-based DB.
			loc, err = sqlite3.MungeLocation(loc)
			if err != nil {
				return err
			}

			lg.FromContext(ctx).Debug("Munged sqlite loc", lga.Before, locBefore, lga.After, loc)
		}

		if typ == drivertype.DuckDB {
			locBefore := loc
			// Special handling for DuckDB, because it's a file-based DB.
			loc, err = duckdb.MungeLocation(loc)
			if err != nil {
				return err
			}

			lg.FromContext(ctx).Debug("Munged duckdb loc", lga.Before, locBefore, lga.After, loc)
		}

		// Apply password: store in keyring or inline, per flags and config.
		if loc, err = applyPassword(ctx, cmd, ru, loc); err != nil {
			return err
		}
	}

	o, err := getSrcOptionsFromFlags(cmd.Flags(), ru.OptionsRegistry, typ)
	if err != nil {
		return err
	}

	src, err := newSource(
		ctx,
		ru.DriverRegistry,
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

	if cfg.Collection.Active() == nil || cmdFlagIsSetTrue(cmd, flag.AddActive) {
		// If no current active data source, use this one, OR if
		// flagAddActive is true.
		if _, err = cfg.Collection.SetActive(src.Handle, false); err != nil {
			return err
		}

		// However, we do not set the active group to be the new src's group.
		// In UX testing, it led to confused users.
	}

	drvr, err := ru.DriverRegistry.DriverFor(src.Type)
	if err != nil {
		return err
	}

	if !cmdFlagIsSetTrue(cmd, flag.SkipVerify) {
		// Typically we want to ping the source before adding it.
		// But, sometimes not, for example if a source is temporarily offline.
		// Resolve secret placeholders first: with --keyring we just wrote
		// the password to keyring and src.Location holds a ${keyring:...}
		// placeholder; the driver can't ping that literal string.
		var pingSrc *source.Source
		pingSrc, err = driver.ResolveSourceSecrets(ctx, src)
		if err != nil {
			return err
		}
		if err = drvr.Ping(ctx, pingSrc); err != nil {
			return err
		}
	}

	if err = ru.ConfigStore.Save(ctx, ru.Config); err != nil {
		return err
	}

	if src, err = ru.Config.Collection.Get(src.Handle); err != nil {
		return err
	}

	return ru.Writers.Source.Added(ru.Config.Collection, src)
}

// applyPassword handles password storage for sq add. It reads any prompted or
// piped password, then either stores it in the OS keyring (replacing the loc
// password field with a ${keyring:...} placeholder) or splices it inline into
// loc. The storage mode is determined by --keyring / --inline-password flags,
// falling back to the secrets.default config option.
func applyPassword(ctx context.Context, cmd *cobra.Command, ru *run.Run, loc string) (string, error) {
	opts := options.FromContext(ctx)
	var useKeyring bool
	switch {
	case cmdFlagIsSetTrue(cmd, flag.Keyring):
		useKeyring = true
	case cmdFlagIsSetTrue(cmd, flag.InlinePassword):
		useKeyring = false
	default:
		useKeyring = secret.OptSecretsDefault.Get(opts) == "keyring"
	}

	// Read password from stdin/prompt if -p was passed.
	var passwd []byte
	var err error
	if cmdFlagIsSetTrue(cmd, flag.PasswordPrompt) {
		if passwd, err = readPassword(ctx, ru.Stdin, ru.Out, ru.Writers.PrOut); err != nil {
			return loc, err
		}
	}

	if useKeyring {
		return applyKeyring(ctx, ru, loc, passwd)
	}

	if len(passwd) > 0 {
		// Inline path: splice the prompted/piped password into the URL.
		if loc, err = location.WithPassword(loc, string(passwd)); err != nil {
			return loc, err
		}
	}
	return loc, nil
}

// resolveDriverType determines the driver type for a `sq add` invocation.
// Three branches, in priority order:
//
//   - --driver was passed: use it verbatim, no detection or resolution.
//   - loc contains a ${...} placeholder: resolve it via the secret
//     registry and detect the driver from the resolved URL. The resolved
//     value is used only for inspection here; it is never persisted to
//     YAML (loc itself is what lands as the Location).
//   - otherwise: today's behavior — detect driver from loc directly.
func resolveDriverType(
	ctx context.Context, ru *run.Run, cmd *cobra.Command,
	handle, loc string, hasPlaceholder bool,
) (drivertype.Type, error) {
	if cmdFlagChanged(cmd, flag.AddDriver) {
		val, _ := cmd.Flags().GetString(flag.AddDriver)
		return drivertype.Type(strings.TrimSpace(val)), nil
	}

	if hasPlaceholder {
		resolved, err := ru.SecretRegistry.Expand(ctx, loc)
		if err != nil {
			return "", errz.Wrapf(err, "resolve %s (or pass --%s to skip resolution)", loc, flag.AddDriver)
		}
		typ, err := ru.Files.DetectType(ctx, handle, resolved)
		if err != nil {
			return "", errz.Wrapf(err, "detect driver from resolved location (or pass --%s)", flag.AddDriver)
		}
		if typ == drivertype.None {
			return "", errz.Errorf("could not infer driver from resolved location: use --%s flag", flag.AddDriver)
		}
		return typ, nil
	}

	typ, err := ru.Files.DetectType(ctx, handle, loc)
	if err != nil {
		return "", err
	}
	if typ == drivertype.None {
		return "", errz.Errorf("unable to determine driver type: use --%s flag", flag.AddDriver)
	}
	return typ, nil
}

// applyKeyring writes the resolved DSN to the OS keyring at a fresh
// opaque ID and returns "${keyring:<id>}" as the new Location.
//
// Preconditions enforced here (per the Form B contract):
//   - loc must be a URL with scheme+host. --keyring on a file path or
//     other non-URL loc would create an orphan keyring entry with a
//     nonsensical stored value, so reject early.
//   - A password must be available. Either:
//     (a) loc already has one in its userinfo, in which case the URL is
//     stored verbatim; or
//     (b) passwd is non-empty (the caller has read -p / piped stdin),
//     in which case it is spliced into loc's userinfo before storage.
//
// No prompting fallback. The original applyKeyringPassword used to call
// readPassword silently when neither (a) nor (b) was true; that path
// hung non-interactive callers and produced incomplete keyring entries
// on /dev/null-style stdin. Require the user to opt into prompting via
// the explicit -p flag instead.
func applyKeyring(ctx context.Context, _ *run.Run, loc string, passwd []byte) (string, error) {
	if !isURLLocation(loc) {
		return loc, errz.Errorf("--%s requires a URL location (got %q)", flag.Keyring, loc)
	}
	if len(passwd) == 0 && !urlHasPassword(loc) {
		return loc, errz.Errorf(
			"--%s requires a password: embed it in the URL or pass --%s",
			flag.Keyring, flag.PasswordPrompt,
		)
	}

	if len(passwd) > 0 {
		spliced, err := location.WithPassword(loc, string(passwd))
		if err != nil {
			return loc, err
		}
		loc = spliced
	}

	kr := keyring.New()
	id, err := kr.NewID(ctx)
	if err != nil {
		return loc, errz.Wrap(err, "mint keyring id")
	}

	if err = kr.Set(ctx, id, loc); err != nil {
		return loc, errz.Wrap(err, "write to keyring")
	}

	return "${keyring:" + id + "}", nil
}

// isURLLocation reports whether loc parses as a URL with a non-empty
// scheme and host. File paths, sqlite/Excel/CSV paths, and any other
// non-URL Location returns false.
func isURLLocation(loc string) bool {
	u, err := url.Parse(loc)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// absolutizeFilePath is a secret.RewritePlaceholders callback: for
// the "file" scheme it returns filepath.Abs(path) when path is bare
// relative (e.g. "./pg.dsn", "pg.dsn", "../shared/pw"). Paths that
// are already usable by the file resolver — absolute, ~/-prefixed,
// or the file:/// URI form — pass through unchanged so user intent
// is preserved (e.g. "~/" stays portable across users). Other
// schemes are no-ops: their path bodies have semantics this helper
// has no business interpreting.
//
// The expansion uses os.Getwd at add time and is captured once;
// later moves of the user's working directory don't affect a source
// already added.
func absolutizeFilePath(_ context.Context, scheme, path string) (string, error) {
	if scheme != "file" || path == "" {
		return path, nil
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		return path, nil
	}
	if strings.HasPrefix(path, "//") {
		// Covers ${file:///abs/...} URI-empty-authority sugar and the
		// rejected file://host/... remote form. Either way, leave for
		// the file resolver to interpret.
		return path, nil
	}
	if filepath.IsAbs(path) {
		return path, nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, errz.Wrapf(err, "absolutize relative file path %q", path)
	}
	return abs, nil
}

// urlHasPassword reports whether loc parses as a URL with a non-empty
// password in its userinfo component. Non-URL locs return false.
func urlHasPassword(loc string) bool {
	u, err := url.Parse(loc)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User == nil {
		return false
	}
	_, has := u.User.Password()
	return has
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

	// Input is read in a goroutine so that we can handle ctrl-c.
	go func() {
		buf := &bytes.Buffer{}
		fmt.Fprint(buf, "Password: ")
		pr.Faint.Fprint(buf, "[ENTER]")
		fmt.Fprint(buf, " ")
		_, _ = stdout.Write(buf.Bytes())

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
