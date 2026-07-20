package execz_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/execz"
)

// TestCmd_LogValue_RedactsEnv verifies that logging a [execz.Cmd] does not
// leak env values. Env vars passed to external tools exist to carry
// connection material (e.g. PGPASSWORD set by postgres.DumpClusterCmd), so
// the log rendering must mask every env value. URL-shaped values are masked
// entirely too: partial URL redaction keeps only the userinfo password
// secret, and would leak credentials carried in the query string (e.g.
// "?sslpassword=..." or a presigned "?X-Amz-Signature=...").
func TestCmd_LogValue_RedactsEnv(t *testing.T) {
	cmd := &execz.Cmd{
		Name: "pg_dumpall",
		Env: []string{
			"PGPASSWORD=hunter2",
			"DSN=postgres://alice@db.acme.com:5432/sakila?sslpassword=hunter2",
		},
		Args: []string{"--dbname", "postgres://alice:hunter2@db.acme.com:5432/sakila"},
	}

	buf := &bytes.Buffer{}
	log := slog.New(slog.NewTextHandler(buf, nil))
	log.Info("exec", "cmd", cmd)
	got := buf.String()

	require.NotContains(t, got, "hunter2",
		"secret must not appear in log output, whether the env value is URL-shaped or not")
	require.Contains(t, got, "PGPASSWORD=",
		"env var name should remain visible")
	require.Contains(t, got, "DSN=",
		"env var name should remain visible")
	require.Contains(t, got, "db.acme.com",
		"non-sensitive parts of URL-shaped args should remain visible")
}

// TestCmd_LogValue_Variants exercises every branch of the redactedCmd switch
// (neither env nor args, env-only, args-only) plus the malformed-env-value
// fallback in redactedEnv, and the nil-receiver guard in LogValue.
func TestCmd_LogValue_Variants(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		require.Equal(t, slog.Value{}, (*execz.Cmd)(nil).LogValue())
	})

	t.Run("name_only", func(t *testing.T) {
		got := logExec(t, &execz.Cmd{Name: "pg_dump"})
		require.Contains(t, got, "exec=pg_dump")
	})

	t.Run("env_only", func(t *testing.T) {
		got := logExec(t, &execz.Cmd{Name: "pg_dump", Env: []string{"PGPASSWORD=hunter2"}})
		require.NotContains(t, got, "hunter2")
		require.Contains(t, got, "PGPASSWORD=")
	})

	t.Run("args_only", func(t *testing.T) {
		got := logExec(t, &execz.Cmd{Name: "pg_dump", Args: []string{"--verbose", "sakila"}})
		require.Contains(t, got, "--verbose")
		require.Contains(t, got, "sakila")
	})

	t.Run("malformed_env_value", func(t *testing.T) {
		// An env entry with no "=" shouldn't panic; it's passed through as-is.
		got := logExec(t, &execz.Cmd{Name: "pg_dump", Env: []string{"BAREWORD"}})
		require.Contains(t, got, "BAREWORD")
	})
}

// logExec logs cmd and returns the rendered log output.
func logExec(t *testing.T, cmd *execz.Cmd) string {
	t.Helper()
	buf := &bytes.Buffer{}
	slog.New(slog.NewTextHandler(buf, nil)).Info("exec", "cmd", cmd)
	return buf.String()
}

// TestCmd_String_IncludesCredentials pins [execz.Cmd.String]'s documented
// behavior: it is the executable (shell) rendering used by the db commands'
// --print flag, and intentionally includes credentials.
func TestCmd_String_IncludesCredentials(t *testing.T) {
	cmd := &execz.Cmd{
		Name: "pg_dumpall",
		Env:  []string{"PGPASSWORD=hunter2"},
		Args: []string{"--dbname", "postgres://alice:hunter2@db.acme.com:5432/sakila"},
	}

	require.Contains(t, cmd.String(), "PGPASSWORD=hunter2")
	require.Contains(t, cmd.String(), "postgres://alice:hunter2@db.acme.com:5432/sakila")
}

// TestCmd_String_Variants covers the nil receiver, the no-env path (sb.Len()
// == 0), and the multi-env path (the i > 0 separator branch).
func TestCmd_String_Variants(t *testing.T) {
	require.Empty(t, (*execz.Cmd)(nil).String())

	// No env: the env-prefix block is skipped entirely.
	require.Equal(t, "pg_dump --verbose", (&execz.Cmd{Name: "pg_dump", Args: []string{"--verbose"}}).String())

	// Multiple env vars: exercises the "i > 0" space separator.
	got := (&execz.Cmd{Name: "pg_dump", Env: []string{"A=1", "B=2"}}).String()
	require.Equal(t, "A=1 B=2 pg_dump", got)
}

// TestExec_NilCmd verifies that Exec returns an error (rather than panicking
// in the deferred cleanup) when cmd is nil.
func TestExec_NilCmd(t *testing.T) {
	err := execz.Exec(context.Background(), nil)
	require.Error(t, err)
}

// TestExec_Success verifies that a command exiting 0 returns no error and that
// its stdout is written to the configured writer.
func TestExec_Success(t *testing.T) {
	stdout := &bytes.Buffer{}
	cmd := helperCmd(t, helperControl{stdout: "hello stdout"})
	cmd.Stdout = stdout
	cmd.Stderr = &bytes.Buffer{}
	cmd.NoProgress = true

	require.NoError(t, execz.Exec(context.Background(), cmd))
	require.Equal(t, "hello stdout", stdout.String())
}

// TestExec_NilStreamsDefault verifies that nil Stdin/Stdout/Stderr are
// defaulted to the os.Std* streams without panicking. NoProgress is set so the
// default branch doesn't attach a progress bar to the real os.Stdout.
func TestExec_NilStreamsDefault(t *testing.T) {
	cmd := helperCmd(t, helperControl{})
	cmd.NoProgress = true
	require.Nil(t, cmd.Stdin)
	require.Nil(t, cmd.Stdout)
	require.Nil(t, cmd.Stderr)

	require.NoError(t, execz.Exec(context.Background(), cmd))
}

// TestExec_DefaultProgressBar exercises the default (stdout-sizing) branch when
// Stdout is an *os.File and NoProgress is false. With no progress in the
// context, NewFilesizeCounter returns a no-op bar, so this checks the branch
// is wired up and Stop is safe.
func TestExec_DefaultProgressBar(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "stdout-*.txt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	cmd := helperCmd(t, helperControl{stdout: "data"})
	cmd.Stdout = f
	cmd.Stderr = &bytes.Buffer{}
	cmd.NoProgress = false

	require.NoError(t, execz.Exec(context.Background(), cmd))

	got, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	require.Equal(t, "data", string(got))
}

// TestExec_ProgressFromStderr exercises the ProgressFromStderr switch branch.
// It's currently a no-op (pending a TODO in Exec), so this just pins that the
// branch runs cleanly.
func TestExec_ProgressFromStderr(t *testing.T) {
	cmd := helperCmd(t, helperControl{stdout: "ok"})
	cmd.Stdout = &bytes.Buffer{}
	cmd.Stderr = &bytes.Buffer{}
	cmd.ProgressFromStderr = true

	require.NoError(t, execz.Exec(context.Background(), cmd))
}

// TestExec_Error verifies that a non-zero exit yields an error that carries the
// ErrPrefix, the underlying exit status, the captured stderr, the correct exit
// code (via errz.ExitCode / the ExitCoder interface), and unwraps to the
// underlying *exec.ExitError.
func TestExec_Error(t *testing.T) {
	stderr := &bytes.Buffer{}
	cmd := helperCmd(t, helperControl{stderr: "boom on stderr\n", exitCode: 3})
	cmd.Stdout = &bytes.Buffer{}
	cmd.Stderr = stderr
	cmd.ErrPrefix = "dump failed"
	cmd.NoProgress = true

	err := execz.Exec(context.Background(), cmd)
	require.Error(t, err)
	require.Contains(t, err.Error(), "dump failed", "error should carry the ErrPrefix")
	require.Contains(t, err.Error(), "boom on stderr", "error should include captured stderr")
	require.NotContains(t, err.Error(), "boom on stderr\n", "trailing newline should be trimmed")
	require.Equal(t, 3, errz.ExitCode(err), "exit code should propagate via ExitCoder")

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr, "error should unwrap to the underlying *exec.ExitError")

	// The captured stderr is still mirrored to the caller's writer.
	require.Equal(t, "boom on stderr\n", stderr.String())
}

// TestExec_StartFailure_ExitCodeMinusOne verifies that when the command fails
// to start (not an *exec.ExitError), ExitCode reports -1.
func TestExec_StartFailure_ExitCodeMinusOne(t *testing.T) {
	cmd := &execz.Cmd{
		Name:      filepath.Join(t.TempDir(), "definitely-not-a-real-binary"),
		Stdout:    &bytes.Buffer{},
		Stderr:    &bytes.Buffer{},
		ErrPrefix: "launch failed",
	}
	cmd.NoProgress = true

	err := execz.Exec(context.Background(), cmd)
	require.Error(t, err)
	require.Equal(t, -1, errz.ExitCode(err),
		"a start failure is not an ExitError, so ExitCode should be -1")
}

// TestExec_UsesOutputFile_Success verifies that the output-file branch runs,
// truncates any pre-existing file, and leaves the command's output in place on
// success. NoProgress is left false to exercise the filesize-counter wiring
// (with no progress in the context, the counter is a no-op).
func TestExec_UsesOutputFile_Success(t *testing.T) {
	outFile := filepath.Join(t.TempDir(), "dump.sql")
	require.NoError(t, os.WriteFile(outFile, []byte("stale data that must be gone"), 0o600))

	cmd := helperCmd(t, helperControl{outFile: outFile, outFileContent: "fresh dump output"})
	cmd.Stdout = &bytes.Buffer{}
	cmd.Stderr = &bytes.Buffer{}
	cmd.UsesOutputFile = outFile
	cmd.NoProgress = false

	require.NoError(t, execz.Exec(context.Background(), cmd))

	got, err := os.ReadFile(outFile)
	require.NoError(t, err)
	require.Equal(t, "fresh dump output", string(got))
}

// TestExec_UsesOutputFile_RemovedOnError verifies the deferred cleanup: when a
// command using an output file fails, the (partial) output file is removed.
func TestExec_UsesOutputFile_RemovedOnError(t *testing.T) {
	outFile := filepath.Join(t.TempDir(), "dump.sql")

	cmd := helperCmd(t, helperControl{
		outFile:        outFile,
		outFileContent: "partial output before failure",
		exitCode:       1,
	})
	cmd.Stdout = &bytes.Buffer{}
	cmd.Stderr = &bytes.Buffer{}
	cmd.UsesOutputFile = outFile
	cmd.NoProgress = true

	err := execz.Exec(context.Background(), cmd)
	require.Error(t, err)
	require.NoFileExists(t, outFile, "the partial output file must be removed on error")
}

// TestExec_CmdDirPath verifies that CmdDirPath prepends the command's directory
// to PATH. The behavioral assertion is unix-only (Windows env semantics for
// duplicate keys differ); on all platforms it confirms the branch runs.
func TestExec_CmdDirPath(t *testing.T) {
	stdout := &bytes.Buffer{}
	cmd := helperCmd(t, helperControl{echoPath: true, stripPath: true})
	cmd.Stdout = stdout
	cmd.Stderr = &bytes.Buffer{}
	cmd.NoProgress = true
	cmd.CmdDirPath = true

	require.NoError(t, execz.Exec(context.Background(), cmd))

	if runtime.GOOS != "windows" {
		require.Contains(t, stdout.String(), filepath.Dir(testExe(t)),
			"CmdDirPath should put the command's own dir on PATH")
	}
}

// testExe returns the absolute, on-disk path of the running test binary. It's
// used (rather than os.Args[0], which is only "the name used to invoke the
// program") so the subprocess re-exec and the CmdDirPath assertion stay stable
// across runners.
func testExe(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	require.NoError(t, err)
	return exe
}

// helperControl configures the behavior of the helper subprocess.
type helperControl struct {
	stdout         string
	stderr         string
	outFile        string
	outFileContent string
	exitCode       int
	echoPath       bool

	// stripPath removes PATH from the inherited env so that a CmdDirPath
	// prepend isn't overridden by os/exec's last-wins env dedup. This mirrors
	// real usage, where Exec doesn't inherit the parent env.
	stripPath bool
}

// helperCmd builds an [execz.Cmd] that re-executes the test binary, landing in
// [TestHelperProcess]. This is the standard, fully portable pattern for testing
// os/exec wrappers without depending on platform shell utilities.
func helperCmd(t *testing.T, ctrl helperControl) *execz.Cmd {
	t.Helper()

	// Inherit the real environment so the re-executed test binary has
	// everything it needs (e.g. SystemRoot on Windows), optionally dropping
	// PATH, then append the control vars that drive the helper.
	base := os.Environ()
	if ctrl.stripPath {
		filtered := base[:0:0]
		for _, e := range base {
			if !strings.HasPrefix(e, "PATH=") {
				filtered = append(filtered, e)
			}
		}
		base = filtered
	}

	return &execz.Cmd{
		Name: testExe(t),
		Args: []string{"-test.run=^TestHelperProcess$", "--"},
		Env: append(
			base,
			"GO_EXECZ_WANT_HELPER_PROCESS=1",
			"HELPER_STDOUT="+ctrl.stdout,
			"HELPER_STDERR="+ctrl.stderr,
			"HELPER_OUTFILE="+ctrl.outFile,
			"HELPER_OUTFILE_CONTENT="+ctrl.outFileContent,
			"HELPER_EXIT="+strconv.Itoa(ctrl.exitCode),
			"HELPER_ECHO_PATH="+strconv.FormatBool(ctrl.echoPath),
		),
	}
}

// TestHelperProcess is not a real test. It's the subprocess entry point used by
// helperCmd: when GO_EXECZ_WANT_HELPER_PROCESS is set, it emulates an external
// command whose behavior is controlled by HELPER_* env vars, then exits.
func TestHelperProcess(_ *testing.T) {
	if os.Getenv("GO_EXECZ_WANT_HELPER_PROCESS") != "1" {
		return
	}

	if s := os.Getenv("HELPER_STDOUT"); s != "" {
		fmt.Fprint(os.Stdout, s)
	}
	if s := os.Getenv("HELPER_STDERR"); s != "" {
		fmt.Fprint(os.Stderr, s)
	}
	if os.Getenv("HELPER_ECHO_PATH") == "true" {
		fmt.Fprint(os.Stdout, os.Getenv("PATH"))
	}
	if fp := os.Getenv("HELPER_OUTFILE"); fp != "" {
		if err := os.WriteFile(fp, []byte(os.Getenv("HELPER_OUTFILE_CONTENT")), 0o600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1) //nolint:revive // subprocess helper must exit directly
		}
	}

	code, _ := strconv.Atoi(os.Getenv("HELPER_EXIT"))
	os.Exit(code) //nolint:revive // subprocess helper must exit directly
}
