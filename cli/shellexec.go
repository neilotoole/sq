package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/source/location"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// ShellCommand represents an external command being prepared or run.
type ShellCommand struct {
	// Stdin is the command's stdin. If nil, [os.Stdin] is used.
	Stdin io.Reader

	// Stdout is the command's stdout. If nil, [os.Stdout] is used.
	Stdout io.Writer

	// Stderr is the command's stderr. If nil, [os.Stderr] is used.
	Stderr io.Writer

	// Name is the executable name, e.g. "pg_dump".
	Name string

	// ErrPrefix is the prefix to use for error messages.
	ErrPrefix string

	// UsesOutputFile indicates that the command its output to this filepath
	// instead of stdout. If empty, stdout is being used.
	UsesOutputFile string

	// Args is the set of args to the command.
	Args []string

	// Env is the set of environment variables to set for the command.
	Env []string

	// ProgressFromStderr indicates that the command outputs progress messages
	// on stderr.
	ProgressFromStderr bool

	// CmdDirPath controls whether the command's PATH will include the parent dir
	// of the command. This allows the command to access sibling commands in the
	// same dir,  e.g. "pg_dumpall" needs to invoke "pg_dump".
	CmdDirPath bool
}

// ShellExec2 executes cmd.
func ShellExec2(ctx context.Context, cmd *ShellCommand) (err error) {
	logShellCmd(lg.FromContext(ctx), "Executing shell cmd", cmd.Name, cmd.Args, cmd.Env)

	c := exec.CommandContext(ctx, cmd.Name, cmd.Args...) //nolint:gosec
	if cmd.CmdDirPath {
		c.Env = append(c.Env, "PATH="+filepath.Dir(c.Path))
	}
	c.Env = append(c.Env, cmd.Env...)
	c.Stdin = cmd.Stdin
	c.Stdout = cmd.Stdout

	c.Stderr = &bytes.Buffer{}
	if err = c.Run(); err != nil {
		return newShellExecError(cmd.ErrPrefix, c, err)
	}
	return nil
}

// FIXME: Switch to ShellCommand using slog.LogValuer.
func logShellCmd(log *slog.Logger, msg, cmd string, shellArgs, shellEnv []string) {
	// Make a copy of shellCmd so that mutations don't affect the original.
	shellArgs = append([]string{cmd}, shellArgs...)
	for i := range shellArgs {
		// If the command element is SQL or HTTP location, redact it.
		locType := location.TypeOf(shellArgs[i])
		if locType == location.TypeSQL || locType == location.TypeHTTP {
			// FIXME: switch to just checking for HTTP URL.
			shellArgs[i] = location.Redact(shellArgs[i])
		}

		shellArgs[i] = stringz.ShellEscape(shellArgs[i])
	}

	// Make a copy of shellEnv so that mutations don't affect the original.
	shellEnv = append([]string(nil), shellEnv...)
	for i := range shellEnv {
		if parts := strings.SplitN(shellEnv[i], "=", 2); len(parts) > 1 {
			// If the env var value is a SQL or HTTP location, redact it.
			locType := location.TypeOf(shellArgs[1])
			if locType == location.TypeSQL || locType == location.TypeHTTP {
				shellEnv[i] = parts[0] + "=" + location.Redact(parts[1])
			}
		}

		shellEnv[i] = stringz.ShellEscape(shellEnv[i])
	}

	if len(shellEnv) == 0 {
		log.Info(msg, lga.Cmd, strings.Join(shellArgs, " "))
	} else {
		log.Info(msg, lga.Cmd, strings.Join(shellArgs, " "), lga.Env, strings.Join(shellEnv, " "))
	}
}

// PrintToolCmd prints the shell command to out.
// TODO: This should really be moved to the outputters.
func PrintToolCmd(out io.Writer, shellCmd, shellEnv []string) error {
	for i := range shellCmd {
		shellCmd[i] = stringz.ShellEscape(shellCmd[i])
	}
	for i := range shellEnv {
		shellEnv[i] = stringz.ShellEscape(shellEnv[i])
	}

	if len(shellEnv) == 0 {
		fmt.Fprintln(out, strings.Join(shellCmd, " "))
	} else {
		fmt.Fprintln(out, strings.Join(shellEnv, " ")+" "+strings.Join(shellCmd, " "))
	}

	return nil
}

var _ error = (*shellExecError)(nil)

// shellExecError is an error that occurred during shell command execution.
type shellExecError struct {
	msg     string
	execErr error
	cmd     *exec.Cmd
	errOut  []byte
}

// Error returns the error message.
func (e *shellExecError) Error() string {
	s := e.msg + ": " + e.execErr.Error()

	if len(e.errOut) > 0 {
		s += ": " + string(e.errOut)
		s = strings.TrimSuffix(s, "\r\n") // windows
		s = strings.TrimSuffix(s, "\n")
	}

	return s
}

// Unwrap returns the underlying error.
func (e *shellExecError) Unwrap() error {
	return e.execErr
}

// ExitCode returns the exit code of the command execution if the underlying
// execution error was an *exec.ExitError, otherwise -1.
func (e *shellExecError) ExitCode() int {
	if ee, ok := errz.As[*exec.ExitError](e.execErr); ok {
		return ee.ExitCode()
	}
	return -1
}

// newShellExecError creates a new shellExecError. If cmd.Stderr is
// a *bytes.Buffer, it will be used to populate the errOut field,
// otherwise errOut may be nil.
func newShellExecError(msg string, cmd *exec.Cmd, execErr error) *shellExecError {
	e := &shellExecError{
		msg:     msg,
		execErr: execErr,
		cmd:     cmd,
	}

	// TODO: We should implement special handling for Lookup errors,
	// e.g. "pg_dump" not found.

	if cmd.Stderr != nil {
		if buf, ok := cmd.Stderr.(*bytes.Buffer); ok && buf != nil {
			e.errOut = buf.Bytes()
		}
	}

	return e
}
