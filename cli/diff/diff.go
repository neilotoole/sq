// Package diff contains the CLI's diff implementation.
//
// Reference:
// - https://github.com/aymanbagabas/go-udiff
// - https://www.gnu.org/software/diffutils/manual/html_node/Hunks.html
// - https://www.cloudbees.com/blog/git-diff-a-complete-comparison-tutorial-for-git
package diff

import (
	"fmt"
	"io"
	"strings"

	"github.com/neilotoole/sq/libsq/source"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// sourceData encapsulates data about a source.
type sourceData struct {
	handle  string
	src     *source.Source
	srcMeta *source.Metadata
}

func (sd *sourceData) clone() *sourceData { //nolint:unused // REVISIT: no longer needed?
	if sd == nil {
		return nil
	}

	return &sourceData{
		handle:  sd.handle,
		src:     sd.src.Clone(),
		srcMeta: sd.srcMeta.Clone(),
	}
}

// tableData encapsulates data about a table.
type tableData struct {
	tblName string
	tblMeta *source.TableMetadata
	src     *source.Source
	srcMeta *source.Metadata
}

func (td *tableData) clone() *tableData { //nolint:unused // REVISIT: no longer needed?
	if td == nil {
		return nil
	}

	return &tableData{
		tblName: td.tblName,
		tblMeta: td.tblMeta.Clone(),
		src:     td.src.Clone(),
		srcMeta: td.srcMeta.Clone(),
	}
}

// sourceDiff is a container for a source diff.
type sourceDiff struct {
	sd1, sd2 *sourceData
	header   string
	diff     string
}

// tableDiff is a container for a table diff.
type tableDiff struct {
	td1, td2 *tableData
	header   string
	diff     string
}

// dbPropsDiff is a container for a DB properties diff.
type dbPropsDiff struct {
	sd1, sd2 *sourceData
	header   string
	diff     string
}

// Print prints dif to w. If pr is nil, printing is in monochrome.
func Print(w io.Writer, pr *output.Printing, header, dif string) error {
	if dif == "" {
		return nil
	}

	if pr == nil || pr.IsMonochrome() {
		if header != "" {
			dif = header + "\n" + dif
		}
		_, err := fmt.Fprintln(w, dif)
		return errz.Err(err)
	}

	after := stringz.VisitLines(dif, func(i int, line string) string {
		if i == 0 && strings.HasPrefix(line, "---") {
			return pr.DiffHeader.Sprint(line)
		}
		if i == 1 && strings.HasPrefix(line, "+++") {
			return pr.DiffHeader.Sprint(line)
		}

		if strings.HasPrefix(line, "@@") {
			return pr.DiffSection.Sprint(line)
		}

		if strings.HasPrefix(line, "-") {
			return pr.DiffMinus.Sprint(line)
		}

		if strings.HasPrefix(line, "+") {
			return pr.DiffPlus.Sprint(line)
		}

		return pr.DiffNormal.Sprint(line)
	})

	if header != "" {
		after = pr.DiffHeader.Sprint(header) + "\n" + after
	}

	_, err := fmt.Fprintln(w, after)
	return errz.Err(err)
}
