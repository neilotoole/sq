package postgres

import (
	"github.com/neilotoole/sq/libsq/source"
)

// REVISIT: DumpCatalogCmd and DumpClusterCmd could be methods on driver.SQLDriver.

// TODO: Unify DumpCatalogCmd and DumpClusterCmd, as they're almost identical, probably
// in the form:
//  DumpCatalogCmd(src *source.Source, all bool) (cmd []string, err error).

// ToolParams are parameters for postgres tools such as pg_dump and pg_restore.
//
// - https://www.postgresql.org/docs/9.6/app-pgdump.html
// - https://www.postgresql.org/docs/9.6/app-pgrestore.html.
// - https://www.postgresql.org/docs/9.6/app-pg-dumpall.html
// - https://cloud.google.com/sql/docs/postgres/import-export/import-export-dmp
//
// Not every flag is applicable to all tools.
type ToolParams struct {
	// File is the path to the dump file.
	File string

	// Verbose indicates verbose output (progress).
	Verbose bool

	// NoOwner won't output commands to set ownership of objects; the source's
	// connection user will own all objects. This also sets the --no-acl flag.
	// Maybe NoOwner should be named "no security" or similar?
	NoOwner bool

	// LongFlags indicates whether to use long flags, e.g. --no-owner instead
	// of -O.
	LongFlags bool
}

func (p *ToolParams) flag(name string) string {
	if p.LongFlags {
		return flagsLong[name]
	}
	return flagsShort[name]
}

// DumpCatalogCmd returns the shell command to execute pg_dump for src.
// Example output:
//
//	pg_dump -Fc -d postgres://alice:vNgR6R@db.acme.com:5432/sales sales.dump
//
// Reference:
//
//   - https://www.postgresql.org/docs/9.6/app-pgdump.html
//   - https://www.postgresql.org/docs/9.6/app-pgrestore.html
//
// See also: [RestoreCatalogCmd].
func DumpCatalogCmd(src *source.Source, p *ToolParams) (cmd, env []string, err error) {
	// - https://www.postgresql.org/docs/9.6/app-pgdump.html
	// - https://cloud.google.com/sql/docs/postgres/import-export/import-export-dmp
	// - https://gist.github.com/vielhuber/96eefdb3aff327bdf8230d753aaee1e1

	cfg, err := getPoolConfig(src, true)
	if err != nil {
		return nil, env, err
	}

	cmd = []string{"pg_dump"}
	if p.Verbose {
		cmd = append(cmd, p.flag(flagVerbose))
	}
	cmd = append(cmd, p.flag(flagFormatCustomArchive))
	if p.NoOwner {
		// You might expect we'd add --no-owner, but if we're outputting a custom
		// archive (-Fc), then --no-owner is the default. From the pg_dump docs:
		//
		//  This option is ignored when emitting an archive (non-text) output file.
		//  For the archive formats, you can specify the option when you call pg_restore.
		//
		// If we ultimately allow non-archive formats, then we'll need to add
		// special handling for --no-owner.
		cmd = append(cmd, p.flag(flagNoACL))
	}
	cmd = append(cmd, p.flag(flagDBName), cfg.ConnString())
	if p.File != "" {
		cmd = append(cmd, p.flag(flagFile), p.File)
	}
	return cmd, env, nil
}

// RestoreCatalogCmd returns the shell command to restore a pg catalog (db) from
// a dump produced by pg_dump ([DumpClusterCmd]). Example command:
//
//	pg_restore -d postgres://alice:vNgR6R@db.acme.com:5432/sales sales.dump
//
// Reference:
//
//   - https://www.postgresql.org/docs/9.6/app-pgrestore.html
//   - https://www.postgresql.org/docs/9.6/app-pgdump.html
//
// See also: [DumpCatalogCmd].
func RestoreCatalogCmd(src *source.Source, p *ToolParams) (cmd, env []string, err error) {
	// - https://cloud.google.com/sql/docs/postgres/import-export/import-export-dmp
	// - https://gist.github.com/vielhuber/96eefdb3aff327bdf8230d753aaee1e1

	cfg, err := getPoolConfig(src, true)
	if err != nil {
		return nil, env, err
	}

	cmd = []string{"pg_restore"}
	if p.Verbose {
		cmd = append(cmd, p.flag(flagVerbose))
	}
	if p.NoOwner {
		// NoOwner sets both --no-owner and --no-acl. Maybe these should
		// be separate options.
		cmd = append(cmd, p.flag(flagNoACL), p.flag(flagNoOwner)) // -O is --no-owner
	}

	cmd = append(cmd,
		p.flag(flagClean),
		p.flag(flagIfExists),
		p.flag(flagCreate),
		p.flag(flagDBName),
		cfg.ConnString(),
	)

	if p.File != "" {
		cmd = append(cmd, p.File)
	}

	return cmd, env, nil
}

// DumpClusterCmd returns the shell command to execute pg_dumpall for src.
// Example output (components concatenated with space):
//
// PGPASSWORD=vNgR6R pg_dumpall -w -l sales -d postgres://alice:vNgR6R@db.acme.com:5432/sales -f cluster.dump
//
// Note that the dump produced by pg_dumpall is executed by psql, not pg_restore.
//
//   - https://www.postgresql.org/docs/9.6/app-pg-dumpall.html
//   - https://www.postgresql.org/docs/9.6/app-psql.html
//   - https://www.postgresql.org/docs/9.6/app-pgdump.html
//   - https://www.postgresql.org/docs/9.6/app-pgrestore.html
//   - https://cloud.google.com/sql/docs/postgres/import-export/import-export-dmp
//
// See also: [RestoreClusterCmd].
func DumpClusterCmd(src *source.Source, p *ToolParams) (cmd, env []string, err error) {
	// - https://www.postgresql.org/docs/9.6/app-pg-dumpall.html
	// - https://cloud.google.com/sql/docs/postgres/import-export/import-export-dmp

	cfg, err := getPoolConfig(src, true)
	if err != nil {
		return nil, env, err
	}

	flags := flagsShort
	if p.LongFlags {
		flags = flagsLong
	}

	// FIXME: need mechanism to indicate that env contains password
	env = []string{"PGPASSWORD=" + cfg.ConnConfig.Password}
	cmd = []string{"pg_dumpall"}
	if p.Verbose {
		cmd = append(cmd, flags[flagVerbose])
	}

	if p.NoOwner {
		// NoOwner sets both --no-owner and --no-acl. Maybe these should
		// be separate options.
		cmd = append(cmd, flags[flagNoACL], flags[flagNoOwner])
	}
	cmd = append(cmd,
		flags[flagNoPassword],
		flags[flagDatabase], cfg.ConnConfig.Database,
		flags[flagDBName], cfg.ConnString(),
	)

	if p.File != "" {
		cmd = append(cmd, flags[flagFile], p.File)
	}

	return cmd, env, nil
}

// RestoreClusterCmd returns the shell command to restore a pg cluster from a
// dump produced by pg_dumpall (DumpClusterCmd). Note that the dump produced
// by pg_dumpall is executed by psql, not pg_restore. Example command:
//
//	psql -d postgres://alice:vNgR6R@db.acme.com:5432/sales -f sales.dump
//
// Reference:
//
//   - https://www.postgresql.org/docs/9.6/app-pg-dumpall.html
//   - https://www.postgresql.org/docs/9.6/app-psql.html
//   - https://www.postgresql.org/docs/9.6/app-pgdump.html
//   - https://www.postgresql.org/docs/9.6/app-pgrestore.html
//   - https://cloud.google.com/sql/docs/postgres/import-export/import-export-dmp
//
// See also: [DumpClusterCmd].
func RestoreClusterCmd(src *source.Source, p *ToolParams) (cmd, env []string, err error) {
	// - https://gist.github.com/vielhuber/96eefdb3aff327bdf8230d753aaee1e1
	cfg, err := getPoolConfig(src, true)
	if err != nil {
		return nil, env, err
	}

	cmd = []string{"psql"}
	if p.Verbose {
		cmd = append(cmd, p.flag(flagVerbose))
	}
	cmd = append(cmd, p.flag(flagDBName), cfg.ConnString())
	if p.File != "" {
		cmd = append(cmd, p.flag(flagFile), p.File)
	}
	return cmd, env, nil
}

// flags for pg_dump and pg_restore programs.
const (
	flagNoOwner             = "--no-owner"
	flagVerbose             = "--verbose"
	flagNoACL               = "--no-acl"
	flagCreate              = "--create"
	flagDBName              = "--dbname"
	flagDatabase            = "--database"
	flagFormatCustomArchive = "--format=custom"
	flagIfExists            = "--if-exists"
	flagClean               = "--clean"
	flagNoPassword          = "--no-password"
	flagFile                = "--file"
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
	flagDatabase:            flagDatabase,
	flagFile:                flagFile,
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
	flagDatabase:            "-l",
	flagFile:                "-f",
}
