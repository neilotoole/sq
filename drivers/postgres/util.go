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

// DumpCmd returns the shell command execute pg_dump for src.
// The command is suitable for execution in a shell.
//
// TODO: This could be a method on driver.SQLDriver?
func DumpCmd(src *source.Source) (cmd string, err error) {
	// - https://www.postgresql.org/docs/9.6/app-pgdump.html
	// - https://cloud.google.com/sql/docs/postgres/import-export/import-export-dmp

	envars, flags, _, err := buildDumpParts(src)
	if err != nil {
		return "", err
	}

	// --dbname=dbname
	// Specifies the name of the database to connect to. This is equivalent to specifying dbname as the first non-option argument on the command line. The dbname can be a connection string.
	// If so, connection string parameters will override any conflicting command line options.
	// DOH ^^

	cmd = envars
	if cmd != "" {
		cmd += " "
	}

	// You might think we'd add --no-owner, but if we're outputting a custom
	// archive (-Fc), then --no-owner is the default. From the pg_dump docs:
	//
	//  This option is ignored when emitting an archive (non-text) output file.
	//  For the archive formats, you can specify the option when you call pg_restore.
	cmd += "pg_dump -Fc --no-acl"
	if flags != "" {
		cmd += " " + flags
	}

	return cmd, nil
}

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

// DumpAllCmd returns the shell command execute pg_dumpall for src.
// The command is suitable for execution in a shell.
//
// TODO: This could be a method on driver.SQLDriver?
func DumpAllCmd(src *source.Source) (cmd string, err error) {
	// - https://www.postgresql.org/docs/9.6/app-pg-dumpall.html
	// - https://cloud.google.com/sql/docs/postgres/import-export/import-export-dmp

	envars, flags, _, err := buildDumpParts(src)
	if err != nil {
		return "", err
	}

	cmd = envars
	if cmd != "" {
		cmd += " "
	}

	// You might think we'd add --no-owner, but if we're outputting a custom
	// archive (-Fc), then --no-owner is the default. From the pg_dump docs:
	//
	//  This option is ignored when emitting an archive (non-text) output file.
	//  For the archive formats, you can specify the option when you call pg_restore.
	cmd += "pg_dump -Fc --no-acl"
	if flags != "" {
		cmd += " " + flags
	}

	return cmd, nil
}

// getConnConfig builds the native postgres config from src.
func getConnConfig(src *source.Source) (*pgconn.Config, error) {
	poolCfg, err := getPoolConfig(src)
	if err != nil {
		return nil, err
	}

	return &poolCfg.ConnConfig.Config, nil
}

func getPoolConfig(src *source.Source) (*pgxpool.Config, error) {
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
