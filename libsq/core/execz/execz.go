// Package execz builds on stdlib os/exec.
package execz

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/langz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/termz"
	"github.com/neilotoole/sq/libsq/source/location"
)

// Cmd represents an external command being prepared or run.
type Cmd struct {
	// Stdin is the command's stdin. If nil, [os.Stdin] is used.
	Stdin io.Reader

	// Stdout is the command's stdout. If nil, [os.Stdout] is used.
	Stdout io.Writer

	// Stderr is the command's stderr. If nil, [os.Stderr] is used.
	Stderr io.Writer

	// Name is the executable name, e.g. "pg_dump".
	Name string

	// Label is a human-readable label for the command, e.g. "@sakila: dump".
	// If empty, [Cmd.Name] is used.
	Label string

	// ErrPrefix is the prefix to use for error messages.
	ErrPrefix string

	// UsesOutputFile indicates that the command its output to this filepath
	// instead of stdout. If empty, stdout is being used.
	UsesOutputFile string

	// Args is the set of args to the command.
	Args []string

	// Env is the set of environment variables to set for the command.
	Env []string

	// NoProgress indicates that progress messages should not be output.
	NoProgress bool

	// ProgressFromStderr indicates that the command outputs progress messages
	// on stderr.
	ProgressFromStderr bool

	// CmdDirPath controls whether the command's PATH will include the parent dir
	// of the command. This allows the command to access sibling commands in the
	// same dir,  e.g. "pg_dumpall" needs to invoke "pg_dump".
	CmdDirPath bool
}

// String returns what command would look like if executed in a shell.
// Note that the returned string could contain sensitive information such as
// passwords, so it's not safe for logging. Instead, see [Cmd.LogValue].
func (c *Cmd) String() string {
	if c == nil {
		return ""
	}

	sb := strings.Builder{}

	for i := range c.Env {
		if i > 0 {
			sb.WriteRune(' ')
		}
		sb.WriteString(stringz.ShellEscape(c.Env[i]))
	}

	if sb.Len() > 0 {
		sb.WriteRune(' ')
	}

	sb.WriteString(stringz.ShellEscape(c.Name))

	for i := range c.Args {
		sb.WriteRune(' ')
		sb.WriteString(stringz.ShellEscape(c.Args[i]))
	}

	return sb.String()
}

// redactedCmd returns a redacted rendering of c, suitable for logging (but
// not execution). If escape is true, the string is also shell-escaped.
func (c *Cmd) redactedCmd(escape bool) string {
	if c == nil {
		return ""
	}

	env := c.redactedEnv(escape)
	args := c.redactedArgs(escape)

	switch {
	case len(env) == 0 && len(args) == 0:
		return c.Name
	case len(env) == 0:
		return c.Name + " " + strings.Join(args, " ")
	case len(args) == 0:
		return strings.Join(env, " ") + " " + c.Name
	default:
		return strings.Join(env, " ") + " " + c.Name + " " + strings.Join(args, " ")
	}
}

// redactedEnv returns c's env with sensitive values redacted.
// If escape is true, the values are also shell-escaped.
func (c *Cmd) redactedEnv(escape bool) []string {
	if c == nil || len(c.Env) == 0 {
		return []string{}
	}

	envars := make([]string, len(c.Env))
	for i := range c.Env {
		parts := strings.SplitN(c.Env[i], "=", 2)
		if len(parts) < 2 {
			// Shouldn't happen, but just in case.
			if escape {
				envars[i] = stringz.ShellEscape(c.Env[i])
			} else {
				envars[i] = c.Env[i]
			}
			continue
		}

		// If the envar value is a SQL or HTTP location, redact it.
		if location.TypeOf(parts[1]).IsURL() {
			parts[1] = location.Redact(parts[1])
		}

		if escape {
			envars[i] = parts[0] + "=" + stringz.ShellEscape(parts[1])
		} else {
			envars[i] = parts[0] + "=" + parts[1]
		}
	}
	return envars
}

// redactedArgs returns c's args with sensitive values redacted.
// If escape is true, the values are also shell-escaped.
func (c *Cmd) redactedArgs(escape bool) []string {
	if c == nil || len(c.Args) == 0 {
		return []string{}
	}

	args := make([]string, len(c.Args))
	for i := range c.Args {
		if location.TypeOf(c.Args[i]).IsURL() {
			args[i] = location.Redact(c.Args[i])
			if escape {
				args[i] = stringz.ShellEscape(args[i])
			}
			continue
		}

		if escape {
			args[i] = stringz.ShellEscape(c.Args[i])
		} else {
			args[i] = c.Args[i]
		}
	}
	return args
}

var _ slog.LogValuer = (*Cmd)(nil)

// LogValue implements [slog.LogValuer]. It redacts sensitive information
// (passwords etc.) from URL-like values.
func (c *Cmd) LogValue() slog.Value {
	if c == nil {
		return slog.Value{}
	}

	attrs := []slog.Attr{
		slog.String("name", c.Name),
		slog.String("exec", c.redactedCmd(false)),
	}

	return slog.GroupValue(attrs...)
}

// Exec executes cmd.
func Exec(ctx context.Context, cmd *Cmd) (err error) {
	log := lg.FromContext(ctx)

	defer func() {
		if err != nil && cmd.UsesOutputFile != "" {
			// If an error occurred, we want to remove the output file.
			lg.WarnIfError(lg.FromContext(ctx), lgm.RemoveFile, os.Remove(cmd.UsesOutputFile))
		}
	}()

	if cmd.Stdin == nil {
		cmd.Stdin = os.Stdin
	}
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}

	execCmd := exec.CommandContext(ctx, cmd.Name, cmd.Args...) //nolint:gosec
	if cmd.CmdDirPath {
		execCmd.Env = append(execCmd.Env, "PATH="+filepath.Dir(execCmd.Path))
	}
	execCmd.Env = append(execCmd.Env, cmd.Env...)
	execCmd.Stdin = cmd.Stdin
	execCmd.Stdout = cmd.Stdout

	stderrBuf := &bytes.Buffer{}
	execCmd.Stderr = io.MultiWriter(stderrBuf, cmd.Stderr)

	switch {
	case cmd.ProgressFromStderr:
		log.Warn("It's cmd.ProgressFromStderr")
		// TODO: We really want to print stderr.
	case cmd.UsesOutputFile != "":
		log.Warn("It's cmd.UsesOutputFile")
		// Truncate the file, ignoring any error (e.g. if it doesn't exist).
		_ = os.Truncate(cmd.UsesOutputFile, 0)

		if !cmd.NoProgress {
			bar := progress.FromContext(ctx).NewFilesizeCounter(
				langz.NonEmptyOf(cmd.Label, cmd.Name),
				nil,
				cmd.UsesOutputFile,
				progress.OptTimer,
			)
			defer bar.Stop()
		}

	default:
		log.Warn("It's default")

		// We're reduced to reading the size of stdout, but not if we're on a
		// terminal. If we are on a terminal, then the user will get to see the
		// command output in real-time and we don't need a progress bar.
		if !termz.IsTerminal(os.Stdout) {
			log.Warn("It's not a terminal")

			if _, ok := cmd.Stdout.(*os.File); ok && !cmd.NoProgress {
				bar := progress.FromContext(ctx).NewFilesizeCounter(
					langz.NonEmptyOf(cmd.Label, cmd.Name),
					cmd.Stdout.(*os.File),
					"",
					progress.OptTimer,
				)
				defer bar.Stop()
			}
		}
	}

	if err = execCmd.Run(); err != nil {
		return newExecError(cmd.ErrPrefix, cmd, execCmd, stderrBuf, err)
	}
	return nil
}

var _ error = (*execError)(nil)

// execError is an error that occurred during command execution.
type execError struct {
	msg     string
	execErr error
	cmd     *Cmd
	execCmd *exec.Cmd
	errOut  []byte
}

// Error returns the error message.
func (e *execError) Error() string {
	s := e.msg + ": " + e.execErr.Error()

	if len(e.errOut) > 0 {
		s += ": " + string(e.errOut)
		s = strings.TrimSuffix(s, "\r\n") // windows
		s = strings.TrimSuffix(s, "\n")
	}

	return s
}

// Unwrap returns the underlying error.
func (e *execError) Unwrap() error {
	return e.execErr
}

// ExitCode returns the exit code of the command execution if the underlying
// execution error was an *exec.ExitError, otherwise -1.
func (e *execError) ExitCode() int {
	if ee, ok := errz.As[*exec.ExitError](e.execErr); ok {
		return ee.ExitCode()
	}
	return -1
}

// newExecError creates a new execError. If cmd.Stderr is
// a *bytes.Buffer, it will be used to populate the errOut field,
// otherwise errOut may be nil.
func newExecError(msg string, cmd *Cmd, execCmd *exec.Cmd, stderrBuf *bytes.Buffer, execErr error) *execError {
	e := &execError{
		msg:     msg,
		execErr: execErr,
		cmd:     cmd,
		execCmd: execCmd,
	}

	// TODO: We should implement special handling for Lookup errors,
	// e.g. "pg_dump" not found.

	if stderrBuf != nil {
		e.errOut = stderrBuf.Bytes()
	}
	return e
}
