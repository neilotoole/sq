package diffw

import (
	"fmt"
	"io"
	"strings"

	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/sourcegraph/go-diff/diff"

	"github.com/fatih/color"
)

type Config struct {
	PlusBg  *color.Color
	Plus    *color.Color
	Minus   *color.Color
	Section *color.Color
	Normal  *color.Color
}

func NewConfig() *Config {
	return &Config{
		// PlusBg:  color.New(color.BgGreen),
		Plus:    color.New(color.FgGreen),
		Minus:   color.New(color.FgRed),
		Section: color.New(color.FgCyan),
		Normal:  color.New(color.Faint),

		// Section: color.New(color.Faint),
	}
}

func PrintSG(w io.Writer, cfg *Config, u string) error {
	_ = cfg
	fdr := diff.NewFileDiffReader(strings.NewReader(u))
	fdiff, err := fdr.Read()
	if err != nil {
		return errz.Err(err)
	}

	out, err := diff.PrintFileDiff(fdiff)
	if err != nil {
		return errz.Err(err)
	}

	_, err = fmt.Fprintln(w, string(out))
	return errz.Err(err)
}

func Print2(w io.Writer, cfg *Config, u string) error {
	lc := stringz.LineCount(strings.NewReader(u), false)
	if lc == 0 {
		return nil
	}

	after := stringz.VisitLines(u, func(i int, line string) string {
		if i == 0 && strings.HasPrefix(line, "---") {
			return cfg.Minus.Sprint(line)
		}
		if i == 1 && strings.HasPrefix(line, "+++") {
			return cfg.Plus.Sprint(line)
		}

		if strings.HasPrefix(line, "@@") {
			return cfg.Section.Sprint(line)
		}

		if strings.HasPrefix(line, "-") {
			return cfg.Minus.Sprint(line)
		}

		if strings.HasPrefix(line, "+") {
			return cfg.Plus.Sprint(line)
		}

		return cfg.Normal.Sprint(line)
	})

	_, err := fmt.Fprintln(w, after)
	return errz.Err(err)
}
