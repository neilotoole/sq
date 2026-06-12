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
	"github.com/neilotoole/sq/libsq/core/ioz"
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
	cmd.Flags().String(flag.AddStore, "", flag.AddStoreUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.AddStore,
		completeStrings(1, flag.AddStoreInline, flag.AddStoreKeyring)))
	cmd.Flags().Bool(flag.SkipVerify, false, flag.SkipVerifyUsage)
	cmd.Flags().BoolP(flag.AddActive, flag.AddActiveShort, false, flag.AddActiveUsage)

	addOptionFlag(cmd.Flags(), driver.OptIngestHeader)
	addOptionFlag(cmd.Flags(), csv.OptEmptyAsNull)
	addOptionFlag(cmd.Flags(), csv.OptDelim)
	panicOn(cmd.RegisterFlagCompletionFunc(csv.OptDelim.Flag().Name, completeStrings(-1, csv.NamedDelims()...)))

	return cmd
}

func execSrcAdd(cmd *cobra.Command, args []string) (err error) {
	ctx := cmd.Context()
	ru := run.FromContext(ctx)
	cfg := ru.Config

	// keyringRollbackID is set non-empty by applyPassword when (and only
	// when) --store keyring mints a fresh keyring entry. If anything in
	// this function later returns an error before the config is
	// persisted, the deferred cleanup deletes that orphan. After
	// ConfigStore.Save succeeds, the placeholder is committed to YAML
	// and we MUST NOT delete the entry — doing so would leave a
	// dangling reference. The id is cleared at that point.
	var keyringRollbackID string
	defer func() {
		if err != nil && keyringRollbackID != "" {
			if delErr := keyring.NewStore().Delete(ctx, keyringRollbackID); delErr != nil {
				// Rollback failed: the user may have an orphan keyring
				// entry they can't easily find (orphan-listing is
				// pending — see #715). Log so the failure is at least
				// recoverable from debug output.
				lg.FromContext(ctx).Warn("Failed to roll back keyring entry on sq add error",
					lga.Path, keyringRollbackID, lga.Err, delErr)
			}
		}
	}()

	loc := location.Abs(strings.TrimSpace(args[0]))
	loc = wrapBareSecretURI(loc)
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
	// at an external store. sq does not own the value: --store (which
	// selects where sq writes the secret) is meaningless here. Reject
	// early with a clear message.
	//
	// Detect placeholders via ExtractRefs rather than a "${"-substring
	// scan so that a literal "${" in the URL (e.g. an escaped
	// "$${env:X}" or a password containing "${") does not flip the
	// branch. ExtractRefs returns an error for malformed placeholders;
	// surface that immediately rather than letting downstream resolve
	// produce a less-clear parse error.
	refs, err := secret.ExtractRefs(loc)
	if err != nil {
		return errz.Wrapf(err, "parse placeholders in location")
	}
	hasPlaceholder := len(refs) > 0
	if hasPlaceholder {
		if cmdFlagChanged(cmd, flag.AddStore) {
			return errz.Errorf("--%s is not supported when the location is a ${...} placeholder", flag.AddStore)
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

	if err = checkFileLocationEscape(loc, hasPlaceholder); err != nil {
		return err
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
		if loc, err = mungeLocationForType(ctx, typ, loc); err != nil {
			return err
		}

		// The pre-detection escape check covers bare file paths; munged
		// file-DB locations (sqlite3:///path, duckdb:///path) carry the
		// '$$' bytes inside a DSN, so check the extracted path here.
		// Especially important for SQLite, which CREATES missing files
		// on open: without this, the ping would silently create and
		// open an empty DB at the interpreted path.
		if fpath, isFileDB := fileDBPath(typ, loc); isFileDB {
			if err = checkPathEscape(fpath); err != nil {
				return err
			}
		}

		// Apply password: store in keyring or inline, per flags and config.
		// keyringRollbackID is set when --store keyring mints a new entry;
		// the deferred cleanup at the top of execSrcAdd undoes it on err.
		if loc, keyringRollbackID, err = applyPassword(ctx, cmd, ru, loc); err != nil {
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

	drvr, err := ru.DriverRegistry.DriverFor(src.Type)
	if err != nil {
		return err
	}

	// Detection may rewrite src.Location (e.g. appending ?tls=true
	// for a TLS-only rqlite endpoint), so it runs before the source
	// is added to the collection and before Ping verifies it.
	if err = detectConnParamsForAdd(ctx, cmd, drvr, src); err != nil {
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

	if !cmdFlagIsSetTrue(cmd, flag.SkipVerify) {
		// Typically we want to ping the source before adding it.
		// But, sometimes not, for example if a source is temporarily offline.
		// Resolve secret placeholders first: with --store keyring we just
		// wrote the full DSN to keyring and src.Location holds a
		// ${keyring:...} placeholder; the driver can't ping that literal
		// string. Same expansion is needed for ${env:...}, ${file:...},
		// or any composition form.
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
	// Placeholder is now persisted to YAML; the keyring entry it
	// references is no longer rollback-eligible — deleting it would
	// leave a dangling ${keyring:<id>} in the saved config.
	keyringRollbackID = ""

	if src, err = ru.Config.Collection.Get(src.Handle); err != nil {
		return err
	}

	return ru.Writers.Source.Added(ru.Config.Collection, src)
}

// resolveSecretStore returns the secret-storage mode to use for the
// current sq add invocation: either flag.AddStoreInline or
// flag.AddStoreKeyring. The --store flag wins; if absent, the
// secrets.store config option provides the default ("inline" out of
// the box). An invalid --store value yields a clear error.
func resolveSecretStore(cmd *cobra.Command, opts options.Options) (string, error) {
	if cmdFlagChanged(cmd, flag.AddStore) {
		v, _ := cmd.Flags().GetString(flag.AddStore)
		switch v {
		case flag.AddStoreInline, flag.AddStoreKeyring:
			return v, nil
		default:
			return "", errz.Errorf("invalid --%s value %q: must be %q or %q",
				flag.AddStore, v, flag.AddStoreInline, flag.AddStoreKeyring)
		}
	}
	return secret.OptSecretsStore.Get(opts), nil
}

// applyPassword handles secret storage for sq add. It reads any
// prompted or piped password, then either writes the full DSN to the
// OS keyring (replacing src.Location with a bare "${keyring:<id>}"
// placeholder, per Form B) or splices the password inline into loc.
// The mode is determined by the --store flag, falling back to the
// secrets.store config option.
//
// Returns (newLoc, keyringID, err). keyringID is non-empty only when
// the --store keyring path executed and a fresh keyring entry was
// minted; callers must roll it back via keyring.Delete if subsequent
// steps before the config is persisted fail. For the inline path,
// keyringID is "" and there is nothing to clean up.
func applyPassword(ctx context.Context, cmd *cobra.Command, ru *run.Run, loc string) (
	newLoc, keyringID string, err error,
) {
	var store string
	store, err = resolveSecretStore(cmd, options.FromContext(ctx))
	if err != nil {
		return loc, "", err
	}
	useKeyring := store == flag.AddStoreKeyring
	storeExplicit := cmdFlagChanged(cmd, flag.AddStore)

	// Read password from stdin/prompt if -p was passed.
	var passwd []byte
	if cmdFlagIsSetTrue(cmd, flag.PasswordPrompt) {
		if passwd, err = readPassword(ctx, ru.Stdin, ru.Out, ru.Writers.PrOut); err != nil {
			return loc, "", err
		}
	}

	// Route to keyring only when there's actually a secret to store —
	// either an inline password in the URL, or one supplied via -p —
	// OR when the user explicitly asked for it via --store keyring.
	// Otherwise (config default of keyring + a source with no
	// credentials, e.g. ./data.csv or postgres://alice@db/sakila),
	// fall through to inline. The user's intent is "add this source";
	// the keyring default shouldn't reject passwordless adds.
	if useKeyring && (storeExplicit || len(passwd) > 0 || urlHasPassword(loc)) {
		return applyKeyring(ctx, ru, loc, passwd)
	}

	if len(passwd) > 0 {
		// Inline path: splice the prompted/piped password into the URL.
		// The password is a literal, but the stored location is a
		// placeholder template in which '$$' means a literal '$', so
		// escape it; the connect path (ResolveSourceSecrets) unescapes.
		// Escape before WithPassword: '$' is never percent-encoded in
		// userinfo, so the '$$' pairs survive URL encoding intact. The
		// keyring path above needs no escaping: keyring slots hold
		// literal values that Registry.Expand splices raw.
		if loc, err = location.WithPassword(loc, secret.Escape(string(passwd))); err != nil {
			return loc, "", err
		}
	}
	return loc, "", nil
}

// mungeLocationForType applies driver-specific location munging for
// the file-based DB types (SQLite, DuckDB); other types pass through
// unchanged. Only meaningful for non-placeholder locations: a
// placeholder location is opaque at add time, and gets the same
// munging at connect time via driver.ResolveSourceSecrets.
func mungeLocationForType(ctx context.Context, typ drivertype.Type, loc string) (string, error) {
	munged, err := location.MungeForDriver(typ, loc)
	if err != nil {
		return "", err
	}
	if munged != loc {
		lg.FromContext(ctx).Debug("Munged location", lga.Before, loc, lga.After, munged)
	}
	return munged, nil
}

// checkFileLocationEscape rejects a typed file location whose '$$'
// escaping is almost certainly unintended; see checkPathEscape.
// Placeholder locations and non-file locations (DSNs) pass through;
// driver-prefixed file-DB paths (sqlite3:, duckdb:) are covered
// separately post-munge via fileDBPath.
func checkFileLocationEscape(loc string, hasPlaceholder bool) error {
	if hasPlaceholder || location.TypeOf(secret.Unescape(loc)) != location.TypeFile {
		return nil
	}
	return checkPathEscape(loc)
}

// checkPathEscape errors when a regular file exists at the typed
// (template) path but not at the template-interpreted path. The
// stored location is a placeholder template: '$$' means a literal
// '$', so the connect path interprets a typed path like
// 'data$$file.csv' as 'data$file.csv'. When the literal file exists
// and the interpreted one doesn't, the user meant the literal file
// and forgot to escape; erroring now names the real path, rather than
// failing later with an error citing a path the user never typed.
func checkPathEscape(typedPath string) error {
	interpreted := secret.Unescape(typedPath)
	if interpreted == typedPath {
		return nil
	}
	if ioz.IsPathToRegularFile(typedPath) && !ioz.IsPathToRegularFile(interpreted) {
		return errz.Errorf(
			"location contains %q, which sq interprets as an escaped literal '$': "+
				"a file exists at %s, but the interpreted path %s does not",
			"$$", typedPath, interpreted)
	}
	return nil
}

// fileDBPath returns the file path component of a munged file-DB
// location (sqlite3:///path, duckdb:///path) and true, or ("", false)
// for other driver types and for non-file forms (e.g. duckdb
// ":memory:").
func fileDBPath(typ drivertype.Type, loc string) (string, bool) {
	src := &source.Source{Type: typ, Location: loc}
	if typ == drivertype.SQLite {
		p, err := sqlite3.PathFromLocation(src)
		return p, err == nil
	}
	if typ == drivertype.DuckDB {
		p, err := duckdb.PathFromLocation(src)
		return p, err == nil
	}
	return "", false
}

// wrapBareSecretURI normalizes location forms that look like a URL but
// whose scheme belongs to an external secret resolver. Today that means
// 1Password: "op://vault/item/field" (the literal "Copy Secret Reference"
// output from the op CLI and 1Password's app UI) is rewritten as
// "${op://vault/item/field}" so the rest of the add flow treats it as
// a placeholder. The bare form is unambiguous because "op" is not a sq
// database driver scheme.
//
// Other resolver schemes (env, file, keyring) don't have a URL-style
// "scheme://" form so there is nothing to normalize for them: their bare
// natural forms ("VAR", "/path/to/file", "abc") are not URL-shaped.
func wrapBareSecretURI(loc string) string {
	if strings.HasPrefix(loc, "op://") {
		return "${" + loc + "}"
	}
	return loc
}

// resolveDriverType determines the driver type for a `sq add` invocation.
// Three branches, in priority order:
//
//   - --driver was passed: use it verbatim, no detection or resolution.
//   - loc contains a ${...} placeholder: resolve it via the secret
//     registry and detect the driver from the resolved URL. The resolved
//     value is used only for inspection here; it is never persisted to
//     YAML (loc itself is what lands as the Location).
//   - otherwise: detect driver from loc directly.
//
// --driver and --skip-verify are orthogonal. --driver suppresses the
// add-time inference resolve (this function's job); --skip-verify
// suppresses the post-add ping (handled in execSrcAdd). The resolve
// failure hint below disambiguates them — users sometimes assume
// --skip-verify alone covers both, but it does not.
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
			return "", errz.Wrapf(err,
				"resolve %s (pass --%s to skip resolution; --%s alone only skips the post-add ping)",
				location.Redact(loc), flag.AddDriver, flag.SkipVerify)
		}
		// Driver inference from a resolved URL is scheme-based —
		// location.Parse handles every URL-shaped DSN sq supports. We
		// deliberately do NOT route through ru.Files.DetectType for
		// URL DSNs: DetectType binds its `loc` argument to a logger
		// attribute and embeds it in fallback error messages, both
		// of which would leak the plaintext resolved DSN (including
		// password) into debug logs and stderr.
		typ, err := driverTypeFromResolved(ctx, ru, handle, resolved)
		if err != nil || typ == drivertype.None {
			// Error and placeholder path are safe to echo (caller typed
			// them); the resolved value is not, so it is never included.
			return "", errz.Errorf(
				"could not infer driver from %s: resolved value is not a DSN. "+
					"Either store a full DSN in the secret (e.g. postgres://alice:pw@db/sakila), "+
					"compose the placeholder into a DSN (e.g. postgres://alice:%s@db/sakila), "+
					"or pass --%s",
				location.Redact(loc), location.Redact(loc), flag.AddDriver)
		}
		return typ, nil
	}

	// Detect from the template-interpreted bytes ('$$' reduced to '$'),
	// not the raw typed bytes: extension-based detection is unaffected,
	// but the byte-sniffing fallback opens the path, and it must read
	// the same file the connect path will open.
	interpreted := secret.Unescape(loc)
	typ, err := ru.Files.DetectType(ctx, handle, interpreted)
	if err != nil {
		if interpreted != loc {
			// Detection ran on the interpreted bytes; surface the typed
			// form too, or the error cites a path the user never typed.
			// Redact: loc may be a DSN with inline credentials.
			return "", errz.Wrapf(err,
				"location %s contains %q, which sq interpreted as an escaped literal '$'",
				location.Redact(loc), "$$")
		}
		return "", err
	}
	if typ == drivertype.None {
		return "", errz.Errorf("unable to determine driver type: use --%s flag", flag.AddDriver)
	}
	return typ, nil
}

// driverTypeFromResolved infers a driver type from a placeholder's
// resolved value WITHOUT leaking the value into logs or error
// messages. URL-shaped DSNs (postgres://..., mysql://..., etc.)
// are classified via location.Parse alone — DriverType is set by
// scheme without any logging-side-effect path. Non-URL resolved
// values (file paths) fall through to Files.DetectType, which is
// safe because file paths don't carry credentials.
//
// Errors returned to callers must be generic; in particular,
// callers must NOT wrap them with the resolved string. location.Parse's
// own errors may include the bad string, so they're swallowed here.
func driverTypeFromResolved(
	ctx context.Context, ru *run.Run, handle, resolved string,
) (drivertype.Type, error) {
	fields, err := location.Parse(resolved)
	if err != nil {
		// Don't propagate err — it may contain the resolved DSN.
		return drivertype.None, errz.New("could not parse resolved location")
	}
	if fields.DriverType != drivertype.None {
		return fields.DriverType, nil
	}
	// Non-URL: file path or similar. DetectType binds loc to its
	// logger, but for file paths (the only resolved values reaching
	// this branch) loc is a filesystem path, not a credential.
	return ru.Files.DetectType(ctx, handle, resolved)
}

// applyKeyring writes the resolved DSN to the OS keyring at a fresh
// opaque ID and returns "${keyring:<id>}" as the new Location.
//
// Preconditions enforced here (per the Form B contract):
//   - loc must be a URL with scheme+host. --store keyring on a file
//     path or other non-URL loc would create an orphan keyring entry
//     with a nonsensical stored value, so reject early.
//   - A password must be available. Either:
//     (a) loc already has one in its userinfo, in which case the URL is
//     stored verbatim; or
//     (b) passwd is non-empty (the caller has read -p / piped stdin),
//     in which case it is spliced into loc's userinfo before storage.
//
// No prompting fallback. An earlier version called readPassword
// silently when neither (a) nor (b) held; that path hung non-interactive
// callers and produced incomplete keyring entries on /dev/null-style
// stdin. Require the user to opt into prompting via the explicit -p
// flag instead.
// applyKeyring returns (newLoc, mintedID, err). On success, mintedID
// is the keyring path the caller is responsible for rolling back if
// any downstream step before the config is persisted fails — leaving
// the entry behind would orphan it. On error, mintedID is "" and the
// caller has nothing to clean up.
func applyKeyring(ctx context.Context, _ *run.Run, loc string, passwd []byte) (newLoc, mintedID string, err error) {
	if !isURLLocation(loc) {
		return loc, "", errz.Errorf("--%s %s requires a URL location (got %q)",
			flag.AddStore, flag.AddStoreKeyring, location.Redact(loc))
	}
	if len(passwd) == 0 && !urlHasPassword(loc) {
		return loc, "", errz.Errorf(
			"--%s %s requires a password: embed it in the URL or pass --%s",
			flag.AddStore, flag.AddStoreKeyring, flag.PasswordPrompt,
		)
	}

	// The typed loc is a placeholder template ('$$' means a literal
	// '$'; zero refs guaranteed by the caller), but the keyring slot
	// holds a literal that Registry.Expand splices raw at connect.
	// Convert to literal form now, before the raw literal password is
	// spliced in below.
	loc = secret.Unescape(loc)

	if len(passwd) > 0 {
		var spliced string
		spliced, err = location.WithPassword(loc, string(passwd))
		if err != nil {
			return loc, "", err
		}
		loc = spliced
	}

	kr := keyring.NewStore()
	var id string
	id, err = kr.NewID(ctx)
	if err != nil {
		return loc, "", errz.Wrap(err, "mint keyring id")
	}

	if err = kr.Set(ctx, id, loc); err != nil {
		return loc, "", errz.Wrap(err, "write to keyring")
	}

	return "${keyring:" + id + "}", id, nil
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

// detectConnParamsForAdd runs the driver's ConnParamDetector (if
// implemented) against the resolved source and merges any detected
// params into src.Location. It is a no-op when --skip-verify is set
// or when the location contains secret placeholders. The function
// name carries the add-time coupling deliberately: the interface is
// a general capability, but the location rewrite is only legitimate
// here, while the source is being established and nothing yet
// references it.
func detectConnParamsForAdd(ctx context.Context, cmd *cobra.Command,
	drvr driver.Driver, src *source.Source,
) error {
	if cmdFlagIsSetTrue(cmd, flag.SkipVerify) {
		return nil
	}
	detector, ok := drvr.(driver.ConnParamDetector)
	if !ok {
		return nil
	}

	// Placeholder-bearing locations are skipped: detected params
	// would have to be merged into a stored form whose URL structure
	// is opaque (e.g. a bare "${keyring:abc}"), which would require
	// composition-aware rewriting of the stored value. Such sources
	// get the standard connection-error hints instead. This mirrors
	// ValidateSource, which also skips grammar checks for placeholders.
	refs, err := secret.ExtractRefs(src.Location)
	if err != nil {
		return err
	}
	if len(refs) > 0 {
		lg.FromContext(ctx).Debug("Conn param detection skipped: placeholder location",
			lga.Src, src.Handle)
		return nil
	}

	probeSrc, err := driver.ResolveSourceSecrets(ctx, src)
	if err != nil {
		return err
	}
	params, err := detector.DetectConnParams(ctx, probeSrc)
	if err != nil {
		return err
	}
	params = filterToAdvertisedParams(ctx, drvr, src, params)
	if len(params) == 0 {
		return nil
	}

	mergedLoc, err := location.MergeQuery(src.Location, params)
	if err != nil {
		return err
	}
	src.Location = mergedLoc
	lg.FromContext(ctx).Debug("Conn param detection rewrote location",
		lga.Src, src.Handle, lga.Loc, src.RedactedLocation())
	return nil
}

// filterToAdvertisedParams enforces the ConnParamDetector contract
// clause that detected keys must be a subset of the keys advertised
// by SQLDriver.ConnParams. A violation is a driver bug: the
// offending keys are dropped with a warning rather than failing the
// user's add.
func filterToAdvertisedParams(ctx context.Context, drvr driver.Driver,
	src *source.Source, params url.Values,
) url.Values {
	if len(params) == 0 {
		return params
	}
	sqlDrvr, ok := drvr.(driver.SQLDriver)
	if !ok {
		// A non-SQL driver advertises no conn params, so no detected
		// key can be validated against the subset invariant: drop
		// them all rather than persisting unvetted params.
		lg.FromContext(ctx).Warn(
			"Detected conn params from non-SQL driver; dropping all",
			lga.Src, src.Handle)
		return url.Values{}
	}
	advertised := sqlDrvr.ConnParams()
	out := url.Values{}
	for k, vs := range params {
		if _, ok := advertised[k]; !ok {
			lg.FromContext(ctx).Warn(
				"Detected conn param not advertised by driver; dropping",
				lga.Src, src.Handle, lga.Key, k)
			continue
		}
		out[k] = vs
	}
	return out
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
