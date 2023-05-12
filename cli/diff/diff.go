// Package diff contains the CLI's diff implementation.
package diff

import (
	"fmt"
	"io"
	"strings"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// func PrintSG(w io.Writer, pr *output.Printing, u string) error {
//	fdr := sgdiff.NewFileDiffReader(strings.NewReader(u))
//	fdiff, err := fdr.Read()
//	if err != nil {
//		return errz.Err(err)
//	}
//
//	out, err := sgdiff.PrintFileDiff(fdiff)
//	if err != nil {
//		return errz.Err(err)
//	}
//
//	_, err = fmt.Fprintln(w, string(out))
//	return errz.Err(err)
//}

func doPrintColor(w io.Writer, pr *output.Printing, dif string) error {
	lc := stringz.LineCount(strings.NewReader(dif), false)
	if lc == 0 {
		return nil
	}

	after := stringz.VisitLines(dif, func(i int, line string) string {
		if i == 0 && strings.HasPrefix(line, "---") {
			return pr.DiffMinus.Sprint(line)
		}
		if i == 1 && strings.HasPrefix(line, "+++") {
			return pr.DiffPlus.Sprint(line)
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

	_, err := fmt.Fprintln(w, after)
	return errz.Err(err)
}
