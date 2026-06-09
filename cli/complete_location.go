package cli

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
)

// locCompStdDirective is the standard cobra shell completion directive
// returned by completeAddLocation.
const locCompStdDirective = cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveKeepOrder

// completeAddLocation provides completion for the "sq add LOCATION" arg.
// Driver-specific URL syntax is declared via driver.LocationShape on
// each SQLDriver; the walker (driver.Walk) consumes typed input
// against a shape and the per-kind suggest helpers generate the
// final candidates.
func completeAddLocation(cmd *cobra.Command, args []string, toComplete string) (
	[]string, cobra.ShellCompDirective,
) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveError
	}

	ctx := cmd.Context()
	ru := run.FromContext(ctx)
	if err := FinishRunInit(ctx, ru); err != nil {
		lg.FromContext(ctx).Error("Init run", lga.Err, err)
		return nil, cobra.ShellCompDirectiveError
	}

	if isDefiniteFilePath(toComplete) {
		return nil, cobra.ShellCompDirectiveDefault
	}

	shapes := collectShapes(ru.DriverRegistry)
	schemeURIs := allSchemeURIs(shapes)

	if toComplete == "" {
		cs := candidateSet{prefix: toComplete}
		cs.add(schemeURIs...)
		cs.add(locCompListFiles(ctx, "")...)
		return cs.build(), locCompStdDirective
	}

	if shape, ok := matchShape(toComplete, shapes); ok {
		drvr, err := ru.DriverRegistry.SQLDriverFor(shape.Type)
		if err != nil {
			lg.FromContext(ctx).Error("Load driver", lga.Err, err)
			return nil, cobra.ShellCompDirectiveError
		}
		m, err := driver.Walk(shape, toComplete)
		if err != nil {
			// Should be unreachable: matchShape already verified the
			// scheme prefix. Log so an unexpected Walk error is
			// diagnosable, then yield to file completion rather than
			// disturbing the user's session with a hard error.
			lg.FromContext(ctx).Debug("Walk location", lga.Err, err)
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		src := newLocSuggestions(ru.Config.Collection, shape.Type, lg.FromContext(ctx))
		return generateCandidates(ctx, shape, m, src, drvr), locCompStdDirective
	}

	partial := stringz.FilterPrefix(toComplete, schemeURIs...)
	if len(partial) == 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}

	cs := candidateSet{prefix: toComplete}
	cs.add(partial...)
	cs.add(locCompListFiles(ctx, toComplete)...)
	return cs.build(), locCompStdDirective
}

// isDefiniteFilePath reports whether the input could only be a file
// path: starts with a path separator, dot, or tilde.
func isDefiniteFilePath(s string) bool {
	if s == "" {
		return false
	}
	// Tilde for home, dot for relative, slash for Unix-absolute,
	// filepath.IsAbs for Windows-absolute (e.g. C:\foo), and a
	// UNC prefix for shared paths on Windows.
	if s[0] == '.' || s[0] == '~' || s[0] == '/' {
		return true
	}
	if strings.HasPrefix(s, `\\`) {
		return true
	}
	return filepath.IsAbs(s)
}

// collectShapes returns the LocationShape from each registered SQL
// driver. Document drivers (csv/json/xlsx) are skipped because they
// have no scheme-based URL form.
func collectShapes(reg *driver.Registry) []driver.LocationShape {
	var shapes []driver.LocationShape
	for _, d := range reg.Drivers() {
		sqld, ok := d.(driver.SQLDriver)
		if !ok {
			continue
		}
		shape := sqld.LocationShape()
		if len(shape.Schemes) > 0 {
			shapes = append(shapes, shape)
		}
	}
	return shapes
}

// allSchemeURIs returns the "scheme://" form for every scheme across
// all shapes, sorted.
func allSchemeURIs(shapes []driver.LocationShape) []string {
	var s []string
	for _, sh := range shapes {
		for _, sc := range sh.Schemes {
			s = append(s, sc+"://")
		}
	}
	slices.Sort(s)
	return s
}

// matchShape returns the shape whose scheme prefixes toComplete.
// Schemes are tested longest-first so "rqlites://" matches before
// "rqlite://".
func matchShape(toComplete string, shapes []driver.LocationShape) (
	driver.LocationShape, bool,
) {
	type entry struct {
		prefix string
		shape  driver.LocationShape
	}
	var entries []entry
	for _, sh := range shapes {
		for _, sc := range sh.Schemes {
			entries = append(entries, entry{sc + "://", sh})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return len(entries[i].prefix) > len(entries[j].prefix)
	})
	for _, e := range entries {
		if strings.HasPrefix(toComplete, e.prefix) {
			return e.shape, true
		}
	}
	return driver.LocationShape{}, false
}

// locCompListFiles completes filenames, mimicking what a shell would
// do. Errors are logged and swallowed.
func locCompListFiles(ctx context.Context, toComplete string) []string {
	var (
		start = toComplete
		files []string
		err   error
	)

	if start == "" {
		start, err = os.Getwd()
		if err != nil {
			return nil
		}
		files, err = ioz.ReadDir(start, false, true, false)
		if err != nil {
			lg.FromContext(ctx).Warn("Read dir", lga.Path, start, lga.Err, err)
		}
		return files
	}

	if strings.HasSuffix(start, "/") {
		files, err = ioz.ReadDir(start, true, true, false)
		if err != nil {
			lg.FromContext(ctx).Warn("Read dir", lga.Path, start, lga.Err, err)
		}
		return files
	}

	dir := filepath.Dir(start)
	fi, err := os.Stat(dir)
	if err == nil && fi.IsDir() {
		files, err = ioz.ReadDir(dir, true, true, false)
		if err != nil {
			lg.FromContext(ctx).Warn("Read dir", lga.Path, start, lga.Err, err)
		}
	} else {
		files = []string{start}
	}

	return stringz.FilterPrefix(toComplete, files...)
}
