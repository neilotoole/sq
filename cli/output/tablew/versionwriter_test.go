package tablew_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/hostinfo"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/tablew"
)

func TestVersionWriter_verbose_newerAvailable(t *testing.T) {
	t.Parallel()

	bi := buildinfo.Info{
		Version:   "v0.53.0",
		Commit:    "abc123",
		Timestamp: time.Date(2026, 5, 26, 1, 21, 12, 0, time.UTC),
	}
	hi := hostinfo.Info{
		Platform:       "darwin",
		Arch:           "arm64",
		Kernel:         "Darwin",
		KernelVersion:  "25.5.0",
		Variant:        "macOS",
		VariantVersion: "26.5.1",
	}

	t.Run("monochrome", func(t *testing.T) {
		t.Parallel()

		buf := &bytes.Buffer{}
		pr := output.NewPrinting()
		pr.Verbose = true
		pr.EnableColor(false)

		err := tablew.NewVersionWriter(buf, pr).Version(bi, "v0.54.0", hi)
		require.NoError(t, err)

		got := buf.String()
		require.Contains(t, got, "Latest version:  v0.54.0\n")
		require.NotContains(t, got, "\x1b[")
	})

	t.Run("color", func(t *testing.T) {
		t.Parallel()

		buf := &bytes.Buffer{}
		pr := output.NewPrinting()
		pr.Verbose = true
		pr.EnableColor(true)

		err := tablew.NewVersionWriter(buf, pr).Version(bi, "v0.54.0", hi)
		require.NoError(t, err)

		got := buf.String()
		require.Contains(t, got, "Latest version:  ")
		require.Contains(t, got, "v0.54.0")
		require.True(t, strings.Contains(got, "\x1b["), "expected ANSI background on latest version")
	})
}
