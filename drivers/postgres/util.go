package postgres

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/retry"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/xo/dburl"
	"strconv"
)

// REVISIT: DumpCmd and DumpAllCmd could be methods on driver.SQLDriver.

// TODO: Unify DumpCmd and DumpAllCmd, as they're almost identical, probably
// in the form:
//
//  DumpCmd(src *source.Source, all bool) (cmd []string, err error).

// ToolParams are parameters for DumpCmd and RestoreCmd.
// See: https://www.postgresql.org/docs/current/app-pgrestore.html.
type ToolParams struct {
	// Verbose indicates verbose output (progress).
	Verbose bool

	// NoOwner won't output commands to set ownership of objects; the source's
	// connection user will own all objects. This also sets the --no-acl flag.
	// Maybe NoOwner should be named "no security" or similar?
	NoOwner bool

	// File is the path to the dump file to restore from. If empty, stdin is used.
	File string

	// LongFlags indicates whether to use long flags, e.g. --no-owner instead of -O.
	LongFlags bool
}

// DumpCmd returns the shell command components to execute pg_dump for src.
// Example output (components concatenated with space):
//
//	pg_dump -Fc --no-acl -d 'postgres://alice:vNgR6R@db.acme.com:5432/sales?connect_timeout=10'
//
// Note that the returned cmd components may need to be shell-escaped if they're
// to be executed in the terminal or via a shell script.
func DumpCmd(src *source.Source, p *ToolParams) (cmd, env []string, err error) {
	// - https://www.postgresql.org/docs/9.6/app-pgdump.html
	// - https://cloud.google.com/sql/docs/postgres/import-export/import-export-dmp
	// - https://gist.github.com/vielhuber/96eefdb3aff327bdf8230d753aaee1e1

	cfg, err := getPoolConfig(src, true)
	if err != nil {
		return nil, env, err
	}

	flags := flagsShort
	if p.LongFlags {
		flags = flagsLong
	}

	cmd = []string{"pg_dump"}
	if p.Verbose {
		cmd = append(cmd, flags[flagVerbose])
	}
	cmd = append(cmd, flags[flagFormatCustomArchive])
	if p.NoOwner {
		// You might expect we'd add --no-owner, but if we're outputting a custom
		// archive (-Fc), then --no-owner is the default. From the pg_dump docs:
		//
		//  This option is ignored when emitting an archive (non-text) output file.
		//  For the archive formats, you can specify the option when you call pg_restore.
		//
		// If we ultimately allow non-archive formats, then we'll need to add
		// special handling for --no-owner.
		cmd = append(cmd, flags[flagNoACL])
	}

	cmd = append(cmd, flags[flagDBName], cfg.ConnString())
	return cmd, env, nil
}

// DumpAllCmd returns the shell command components to execute pg_dump for src.
// Example output (components concatenated with space):
//
//	pg_dump -Fc --no-acl -d 'postgres://alice:vNgR6R@db.acme.com:5432/sales?connect_timeout=10'
//
// Note that the returned cmd components may need to be shell-escaped if they're
// to be executed in the terminal or via a shell script.
func DumpAllCmd(src *source.Source, verbose bool) (cmd, env []string, err error) {
	// - https://www.postgresql.org/docs/9.6/app-pg-dumpall.html
	// - https://cloud.google.com/sql/docs/postgres/import-export/import-export-dmp

	cfg, err := getPoolConfig(src, true)
	if err != nil {
		return nil, env, err
	}

	env = []string{"PGPASSWORD=" + cfg.ConnConfig.Password}
	cmd = []string{"pg_dumpall"}
	if verbose {
		cmd = append(cmd, "-v")
	}

	cmd = append(cmd,
		"-w", // -w is --no-password
		"-O", // -O is --no-owner
		"-l", // -l is --database
		cfg.ConnConfig.Database,
		"-d", // -d is --dbname
		cfg.ConnString(),
	)
	return cmd, env, nil
}

// RestoreCmd returns the shell command components to execute pg_restore for src.
// Example output (components concatenated with space):
//
// Note that the returned cmd components may need to be shell-escaped if they're
// to be executed in the terminal or via a shell script.
func RestoreCmd(src *source.Source, p *ToolParams) (cmd, env []string, err error) {
	// - https://www.postgresql.org/docs/9.6/app-pgrestore.html
	// - https://www.postgresql.org/docs/9.6/app-pgdump.html
	// - https://cloud.google.com/sql/docs/postgres/import-export/import-export-dmp
	// - https://gist.github.com/vielhuber/96eefdb3aff327bdf8230d753aaee1e1

	flags := flagsShort
	if p.LongFlags {
		flags = flagsLong
	}

	cfg, err := getPoolConfig(src, true)
	if err != nil {
		return nil, env, err
	}

	cmd = []string{"pg_restore"}
	if p.Verbose {
		cmd = append(cmd, flags[flagVerbose])
	}
	if p.NoOwner {
		// NoOwner sets both --no-owner and --no-acl. Maybe these should
		// be separate options.
		cmd = append(cmd, flags[flagNoACL], flags[flagNoOwner]) // -O is --no-owner
	}

	cmd = append(cmd,
		flags[flagClean],    // -c is --clean, meaning clean/drop db objects before restore
		flags[flagIfExists], // ignore errors if objects don't exist, e.g. when dropping
		flags[flagCreate],   // -C is --create
		flags[flagDBName],
		cfg.ConnString())

	if p.File != "" {
		cmd = append(cmd, p.File)
	}

	return cmd, env, nil
}

// RestoreAllCmd returns the shell command components to execute pg_restore for src.
// Example output (components concatenated with space):
//
//	pg_dump -Fc --no-acl -d 'postgres://alice:vNgR6R@db.acme.com:5432/sales?connect_timeout=10'
//
// Note that the returned cmd components may need to be shell-escaped if they're
// to be executed in the terminal or via a shell script.
//
// FIXME: maybe delete this?
func RestoreAllCmd(src *source.Source, verbose bool) (cmd, env []string, err error) {
	// - https://www.postgresql.org/docs/9.6/app-pgrestore.html
	// - https://www.postgresql.org/docs/9.6/app-pgdump.html
	// - https://cloud.google.com/sql/docs/postgres/import-export/import-export-dmp
	// - https://gist.github.com/vielhuber/96eefdb3aff327bdf8230d753aaee1e1

	cfg, err := getPoolConfig(src, true)
	if err != nil {
		return nil, env, err
	}

	cmd = []string{"pg_restore"}
	if verbose {
		cmd = append(cmd, "-v")
	}
	cmd = append(cmd,
		"--no-acl",
		"-c", // -c is --clean, meaning clean/drop db objects before restore
		"-C", // -C is --create
		"-O", // -O is --no-owner
		"-d", // -d is --dbname (conn string)
		cfg.ConnString())
	return cmd, env, nil
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

// flags for pg_dump and pg_restore programs.
const (
	flagNoOwner             = "--no-owner"
	flagVerbose             = "--verbose"
	flagNoACL               = "--no-acl"
	flagCreate              = "--create"
	flagDBName              = "--dbname"
	flagFormatCustomArchive = "--format=custom"
	flagIfExists            = "--if-exists"
	flagClean               = "--clean"
	flagNoPassword          = "--no-password"
)

var flagsLong = map[string]string{
	flagNoOwner:             flagNoOwner,
	flagVerbose:             flagVerbose,
	flagNoACL:               flagNoACL,
	flagCreate:              flagCreate,
	flagDBName:              flagDBName,
	flagIfExists:            flagIfExists,
	flagFormatCustomArchive: flagFormatCustomArchive,
	flagClean:               flagClean,
	flagNoPassword:          flagNoPassword,
}

var flagsShort = map[string]string{
	flagNoOwner:             "-O",
	flagVerbose:             "-v",
	flagNoACL:               "-x",
	flagCreate:              "-C",
	flagClean:               "-c",
	flagDBName:              "-d",
	flagFormatCustomArchive: "-Fc",
	flagIfExists:            "--if-exists",
	flagNoPassword:          "-w",
}
