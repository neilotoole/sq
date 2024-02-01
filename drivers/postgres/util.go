package postgres

import (
	"context"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/retry"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/xo/dburl"
)

// REVISIT: DumpCmd and DumpAllCmd could be methods on driver.SQLDriver.

// TODO: Unify DumpCmd and DumpAllCmd, as they're almost identical, probably
// in the form:
//
//  DumpCmd(src *source.Source, all bool) (cmd []string, err error).

// DumpCmd returns the shell command components to execute pg_dump for src.
// Example output (components concatenated with space):
//
//	pg_dump -Fc --no-acl -d 'postgres://alice:vNgR6R@db.acme.com:5432/sales?connect_timeout=10'
//
// Note that the returned cmd components may need to be shell-escaped if they're
// to be executed in the terminal or via a shell script.
func DumpCmd(src *source.Source) (cmd, env []string, err error) {
	// - https://www.postgresql.org/docs/9.6/app-pgdump.html
	// - https://cloud.google.com/sql/docs/postgres/import-export/import-export-dmp
	// - https://gist.github.com/vielhuber/96eefdb3aff327bdf8230d753aaee1e1

	cfg, err := getPoolConfig(src, true)
	if err != nil {
		return nil, env, err
	}

	// You might expect we'd add --no-owner, but if we're outputting a custom
	// archive (-Fc), then --no-owner is the default. From the pg_dump docs:
	//
	//  This option is ignored when emitting an archive (non-text) output file.
	//  For the archive formats, you can specify the option when you call pg_restore.
	//
	// If we ultimately allow non-archive formats, then we'll need to add
	// special handling for --no-owner, e.g. making it an optional flag.
	//
	// Note that -d is "--db-name", which takes the connection string.
	cmd = []string{
		"pg_dump",
		"-Fc",
		"--no-acl",
		"-d",
		cfg.ConnString(),
	}
	return cmd, env, nil
}

// DumpAllCmd returns the shell command components to execute pg_dump for src.
// Example output (components concatenated with space):
//
//	pg_dump -Fc --no-acl -d 'postgres://alice:vNgR6R@db.acme.com:5432/sales?connect_timeout=10'
//
// Note that the returned cmd components may need to be shell-escaped if they're
// to be executed in the terminal or via a shell script.
func DumpAllCmd(src *source.Source) (cmd, env []string, err error) {
	// - https://www.postgresql.org/docs/9.6/app-pg-dumpall.html
	// - https://cloud.google.com/sql/docs/postgres/import-export/import-export-dmp

	cfg, err := getPoolConfig(src, true)
	if err != nil {
		return nil, env, err
	}

	env = []string{"PGPASSWORD=" + cfg.ConnConfig.Password}
	cmd = []string{
		"pg_dumpall",
		"-w",                          // -w is --no-password
		"-O",                          // -O is --no-owner
		"-l", cfg.ConnConfig.Database, // -l is --database
		"-d", cfg.ConnString(), // -d is --dbname
	}
	return cmd, env, nil
}

// getConnConfig builds the native postgres config from src.
//
// Deprecated: use getPoolConfig instead.
func getConnConfig(src *source.Source) (*pgconn.Config, error) { //nolint:unused
	poolCfg, err := getPoolConfig(src, false)
	if err != nil {
		return nil, err
	}

	return &poolCfg.ConnConfig.Config, nil
}

// getPoolConfig returns the native postgres [*pgxpool.Config] for src, applying
// src's fields, such as [source.Source.Catalog] as appropriate. If
// includeConnTimeout is true, then 'connect_timeout' is included in the
// returned config; this is provided as an option, because the connection
// timeout is sometimes better handled via [context.WithTimeout].
func getPoolConfig(src *source.Source, includeConnTimeout bool) (*pgxpool.Config, error) {
	poolCfg, err := pgxpool.ParseConfig(src.Location)
	if err != nil {
		return nil, errw(err)
	}

	if src.Catalog != "" && src.Catalog != poolCfg.ConnConfig.Database {
		// The catalog differs from the database in the connection string.
		// OOTB, Postgres doesn't support cross-database references. So,
		// we'll need to change the connection string to use the catalog
		// as the database. Note that we don't modify src.Location, but it's
		// not entirely clear if that's the correct approach. Are there any
		// downsides to modifying it (as long as the modified Location is not
		// persisted back to config)?
		var u *dburl.URL
		if u, err = dburl.Parse(src.Location); err != nil {
			return nil, errw(err)
		}

		u.Path = src.Catalog
		connStr := u.String()
		poolCfg, err = pgxpool.ParseConfig(connStr)
		if err != nil {
			return nil, errw(err)
		}
	}

	if includeConnTimeout {
		srcTimeout := driver.OptConnOpenTimeout.Get(src.Options)
		// Only set connect_timeout if it's non-zero and differs from the
		// already-configured value.
		// REVISIT: We should actually always set it, otherwise the user's
		// envar PGCONNECT_TIMEOUT may override it?

		if srcTimeout > 0 || poolCfg.ConnConfig.ConnectTimeout != srcTimeout {
			var u *dburl.URL
			if u, err = dburl.Parse(poolCfg.ConnString()); err != nil {
				return nil, errw(err)
			}

			q := u.Query()
			q.Set("connect_timeout", strconv.Itoa(int(srcTimeout.Seconds())))
			u.RawQuery = q.Encode()
			poolCfg, err = pgxpool.ParseConfig(u.String())
			if err != nil {
				return nil, errw(err)
			}
		}
	}

	return poolCfg, nil
}

// doRetry executes fn with retry on isErrTooManyConnections.
func doRetry(ctx context.Context, fn func() error) error {
	maxRetryInterval := driver.OptMaxRetryInterval.Get(options.FromContext(ctx))
	return retry.Do(ctx, maxRetryInterval, fn, isErrTooManyConnections)
}

// tblfmt formats a table name for use in a query. The arg can be a string,
// or a tablefq.T.
func tblfmt[T string | tablefq.T](tbl T) string {
	tfq := tablefq.From(tbl)
	return tfq.Render(stringz.DoubleQuote)
}

//nolint:unused
func buildDumpParts(src *source.Source) (envars, flags string, cfg *pgconn.Config, err error) {
	if cfg, err = getConnConfig(src); err != nil {
		return "", "", nil, err
	}

	if cfg.Password == "" {
		envars = "PGPASSWORD=''"
	} else {
		envars = "PGPASSWORD=" + stringz.ShellEscape(cfg.Password)
	}

	timeout := driver.OptConnOpenTimeout.Get(src.Options)
	if timeout > 0 {
		envars += " PGCONNECT_TIMEOUT=" + strconv.Itoa(int(timeout.Seconds()))
	}

	if cfg.Port != 0 && cfg.Port != 5432 {
		//  Don't include the port if we don't need to.
		flags += " -p " + strconv.Itoa(int(cfg.Port))
	}

	if cfg.User != "" {
		flags += " -U " + stringz.ShellEscape(cfg.User)
	}

	if cfg.Host != "" {
		flags += " -h " + cfg.Host
	}

	if cfg.Database != "" {
		flags += " " + cfg.Database
	}

	return strings.TrimSpace(envars), strings.TrimSpace(flags), cfg, nil
}
