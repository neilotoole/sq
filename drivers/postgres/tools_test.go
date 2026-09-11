package postgres

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

func toolSrc() *source.Source {
	return &source.Source{Handle: "@h", Type: drivertype.Pg, Location: pgLoc}
}

func TestDumpCatalogCmd(t *testing.T) {
	src := toolSrc()

	// Short flags.
	cmd, err := DumpCatalogCmd(src, &ToolParams{Verbose: true, NoOwner: true, File: "out.dump"})
	require.NoError(t, err)
	require.Equal(t, "pg_dump", cmd.Name)
	require.True(t, cmd.ProgressFromStderr)
	require.Equal(t, "out.dump", cmd.UsesOutputFile)
	require.True(t, slices.Contains(cmd.Args, "-v"))
	require.True(t, slices.Contains(cmd.Args, "-Fc"))
	require.True(t, slices.Contains(cmd.Args, "-x")) // --no-acl from NoOwner
	require.True(t, slices.Contains(cmd.Args, "-f"))

	// Long flags, no file, no verbose.
	cmd, err = DumpCatalogCmd(src, &ToolParams{LongFlags: true})
	require.NoError(t, err)
	require.False(t, cmd.ProgressFromStderr)
	require.Empty(t, cmd.UsesOutputFile)
	require.True(t, slices.Contains(cmd.Args, "--format=custom"))
	require.False(t, slices.Contains(cmd.Args, "--verbose"))
	require.False(t, slices.Contains(cmd.Args, "--file"))
}

func TestRestoreCatalogCmd(t *testing.T) {
	src := toolSrc()
	cmd, err := RestoreCatalogCmd(src, &ToolParams{Verbose: true, NoOwner: true, File: "in.dump", LongFlags: true})
	require.NoError(t, err)
	require.Equal(t, "pg_restore", cmd.Name)
	require.True(t, cmd.CmdDirPath)
	require.Equal(t, "in.dump", cmd.UsesOutputFile)
	require.True(t, slices.Contains(cmd.Args, "--no-owner"))
	require.True(t, slices.Contains(cmd.Args, "--create"))
	require.Equal(t, "in.dump", cmd.Args[len(cmd.Args)-1])
}

func TestDumpClusterCmd(t *testing.T) {
	src := toolSrc()
	cmd, err := DumpClusterCmd(src, &ToolParams{Verbose: true, NoOwner: true, File: "cluster.dump"})
	require.NoError(t, err)
	require.Equal(t, "pg_dumpall", cmd.Name)
	require.True(t, cmd.CmdDirPath)
	// The password is passed via PGPASSWORD env var.
	require.True(t, slices.Contains(cmd.Env, "PGPASSWORD=secret"))
	require.True(t, slices.Contains(cmd.Args, "-w")) // --no-password
	require.True(t, slices.Contains(cmd.Args, "-f"))
}

func TestRestoreClusterCmd(t *testing.T) {
	src := toolSrc()
	cmd, err := RestoreClusterCmd(src, &ToolParams{Verbose: true, File: "cluster.dump"})
	require.NoError(t, err)
	require.Equal(t, "psql", cmd.Name)
	require.True(t, slices.Contains(cmd.Args, "-v"))
	require.True(t, slices.Contains(cmd.Args, "-f"))
}

func TestExecCmd(t *testing.T) {
	src := toolSrc()

	// Script file, non-verbose -> quiet flag present.
	cmd, err := ExecCmd(src, &ExecToolParams{ScriptFile: "query.sql"})
	require.NoError(t, err)
	require.Equal(t, "psql", cmd.Name)
	require.True(t, slices.Contains(cmd.Args, "-q"))
	require.True(t, slices.Contains(cmd.Args, "-f"))

	// Command string, verbose -> no quiet flag.
	cmd, err = ExecCmd(src, &ExecToolParams{CmdString: "SELECT 1", Verbose: true})
	require.NoError(t, err)
	require.False(t, slices.Contains(cmd.Args, "-q"))
	require.True(t, slices.Contains(cmd.Args, "-c"))

	// Both script file and command string is an error.
	_, err = ExecCmd(src, &ExecToolParams{ScriptFile: "query.sql", CmdString: "SELECT 1"})
	require.Error(t, err)

	// A malformed location surfaces the getPoolConfig error.
	_, err = ExecCmd(&source.Source{Handle: "@h", Type: drivertype.Pg, Location: "://bad"}, &ExecToolParams{})
	require.Error(t, err)
}
