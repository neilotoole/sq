package cli

import (
	"bytes"
	"os/exec"
	"strings"

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
