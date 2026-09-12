package footer

import (
	"bytes"
	"context"
	"io"

	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/cli/updatecheck"
)

type updateNotice struct{}

func (updateNotice) Eligible(_ context.Context, ru *run.Run, err error) bool {
	if footerSuppressingError(err) {
		return false
	}
	if isVersionCommand(ru) {
		return false
	}

	latest, ok := cachedLatest(ru)
	if !ok || latest == "" {
		return false
	}

	current := buildinfo.Get().Version
	return semver.Compare(latest, current) > 0
}

func (updateNotice) Write(w io.Writer, pr *output.Printing, ru *run.Run, width int) error {
	latest, ok := cachedLatest(ru)
	if !ok {
		return nil
	}

	var buf bytes.Buffer
	pr.Warning.Fprint(&buf, "Update available: ")
	pr.UpdateAvailable.Fprint(&buf, latest)

	line := alignRight(buf.String(), width)
	_, err := io.WriteString(w, line+"\n")
	return err
}

func cachedLatest(ru *run.Run) (string, bool) {
	return updatecheck.CachedLatest(updatecheck.CacheDirForRun(ru))
}

func isVersionCommand(ru *run.Run) bool {
	if ru.Cmd == nil {
		return false
	}
	if ru.Cmd.Name() == "version" {
		return true
	}
	return cmdFlagChanged(ru.Cmd, flag.Version)
}
