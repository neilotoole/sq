package cli

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/source/location"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"

	"github.com/neilotoole/sq/libsq/core/errz"
)

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

// shellExec executes shellCmd with the environment specified in shellEnv.
// The first element of shellCmd is the command name; the remaining elements
// are passed as args to the command. If cmdDirPath is true, the command's PATH
// will include the directory of the command. This allows the command to access
// sibling commands in the same dir, e.g. "pg_dumpall" needs to invoke "pg_dump".
// If outfile is empty, stdout is used; if non-empty, the command's output is
// written to outfile.
//
// TODO: Move shellExec stuff to its own package.
func shellExec(ru *run.Run, errMsg string, shellCmd, shellEnv []string, cmdDirPath bool) (err error) {
	ctx := ru.Cmd.Context()

	logShellCmd(lg.FromContext(ctx), "Executing shell cmd", shellCmd, shellEnv)

	c := exec.CommandContext(ctx, shellCmd[0], shellCmd[1:]...) //nolint:gosec
	if cmdDirPath {
		c.Env = append(c.Env, "PATH="+filepath.Dir(c.Path))
	}
	c.Env = append(c.Env, shellEnv...)
	c.Stdin = ru.Stdin
	c.Stdout = os.Stdout

	c.Stderr = &bytes.Buffer{}
	if err = c.Run(); err != nil {
		return newShellExecError(errMsg, c, err)
	}
	return nil
}

func logShellCmd(log *slog.Logger, msg string, shellCmd, shellEnv []string) {
	// Make a copy of shellCmd so that mutations don't affect the original.
	shellCmd = append([]string(nil), shellCmd...)
	for i := range shellCmd {
		// If the command element is SQL or HTTP location, redact it.
		locType := location.TypeOf(shellCmd[i])
		if locType == location.TypeSQL || locType == location.TypeHTTP {
			shellCmd[i] = location.Redact(shellCmd[i])
		}

		shellCmd[i] = stringz.ShellEscape(shellCmd[i])
	}

	// Make a copy of shellEnv so that mutations don't affect the original.
	shellEnv = append([]string(nil), shellEnv...)
	for i := range shellEnv {
		if parts := strings.SplitN(shellEnv[i], "=", 2); len(parts) > 1 {
			// If the env var value is a SQL or HTTP location, redact it.
			locType := location.TypeOf(shellCmd[1])
			if locType == location.TypeSQL || locType == location.TypeHTTP {
				shellEnv[i] = parts[0] + "=" + location.Redact(parts[1])
			}
		}

		shellEnv[i] = stringz.ShellEscape(shellEnv[i])
	}

	if len(shellEnv) == 0 {
		log.Info(msg, lga.Cmd, strings.Join(shellCmd, " "))
	} else {
		log.Info(msg, lga.Cmd, strings.Join(shellCmd, " "), lga.Env, strings.Join(shellEnv, " "))
	}
}

//nolint:gocritic
func shellExecPgRestoreCluster(ru *run.Run, src *source.Source, shellCmd, shellEnv []string) error {
	_ = ru
	_ = src
	_ = shellCmd
	_ = shellEnv

	return errz.New("not implemented")
	//c := exec.CommandContext(ru.Cmd.Context(), shellCmd[0], shellCmd[1:]...) //nolint:gosec
	//
	//// PATH shenanigans are required to ensure that pg_dumpall can find pg_dump.
	//// Otherwise we see this error:
	////
	////  pg_dumpall: error: program "pg_dump" is needed by pg_dumpall but was not
	////   found in the same directory as "pg_dumpall"
	//c.Env = append(c.Env, "PATH="+filepath.Dir(c.Path))
	//c.Env = append(c.Env, shellEnv...)
	//
	//c.Stdout = os.Stdout
	//c.Stderr = &bytes.Buffer{}
	//if err := c.Run(); err != nil {
	//	return newShellExecError(fmt.Sprintf("db dump --all: %s", src.Handle), c, err)
	//}
	//return nil
}

func printToolCmd(ru *run.Run, shellCmd, shellEnv []string) error {
	for i := range shellCmd {
		shellCmd[i] = stringz.ShellEscape(shellCmd[i])
	}
	for i := range shellEnv {
		shellEnv[i] = stringz.ShellEscape(shellEnv[i])
	}

	if len(shellEnv) == 0 {
		fmt.Fprintln(ru.Out, strings.Join(shellCmd, " "))
	} else {
		fmt.Fprintln(ru.Out, strings.Join(shellEnv, " ")+" "+strings.Join(shellCmd, " "))
	}

	return nil
}
