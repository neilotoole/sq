package sqlparser

import (
	"bytes"
	"context"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz/scannerz"
)

// StmtType is the type of SQL statement such as "select".
type StmtType string

const (
	// StmtSelect is executed using sql.DB.Query.
	StmtSelect = "select"

	// StmtOther is executed using sql.DB.Exec.
	StmtOther = "other"
)

// SplitSQL splits SQL text into multiple statements,
// demarcated by delim (typically a semicolon) or additional
// delim values such as "GO" or "GO;"
// For example, this is useful for splitting up a .sql file
// containing multiple statements.
// Empty lines and comment lines are not returned, nor are the
// separator elements themselves.
//
// This is a very rudimentary implementation.
// It currently only works if the delimiters are at the
// end of the line. Also, its ability to detect the correct
// statement type is limited.
func SplitSQL(ctx context.Context, input io.Reader, delim string, moreDelims ...string,
) (stmts []string, types []StmtType, err error) {
	// NOTE: There are parser libraries such as xwb1989/sqlparser
	//  but from a quick look, it seems that they cannot parse
	//  all SQL dialects. Also, the input->parse->output process
	//  munges the input SQL when the tree is rendered back into
	//  SQL, and we want to pass the SQL statements through as
	//  unmolested as possible. It certainly is worth doing more
	//  research on what parsers are available and then
	//  hopefully we can ditch this brittle code.

	allDelims := append([]string{delim}, moreDelims...)

	data, err := io.ReadAll(input)
	if err != nil {
		return nil, types, errz.Err(err)
	}

	scanner := scannerz.NewScanner(ctx, bytes.NewReader(data))
	sb := strings.Builder{}

	// First pass, ditch comments and empty lines
	for scanner.Scan() {
		err = scanner.Err()
		if err != nil {
			return nil, types, errz.Err(err)
		}

		line := scanner.Text()
		trimLine := strings.TrimSpace(line)

		switch {
		case trimLine == "":
			// Ignore empty lines?
			continue
		case strings.HasPrefix(trimLine, "--"):
			// Ditch standalone comment lines
			continue
		}

		sb.WriteString(line)
		sb.WriteRune('\n')
	}

	firstPassResult := sb.String()

	// Second pass, split her up by delim at end of line
	scanner = scannerz.NewScanner(ctx, strings.NewReader(firstPassResult))
	buf := &bytes.Buffer{}

	for scanner.Scan() {
		err = scanner.Err()
		if err != nil {
			return nil, types, errz.Err(err)
		}

		line := scanner.Text()
		// Trim any trailing whitespace
		lineTrimRightSpace := strings.TrimRightFunc(line, unicode.IsSpace)

		// Trim any trailing delims
		lineDelimTrimmed := trimTrailingDelims(lineTrimRightSpace, allDelims...)
		if lineDelimTrimmed == lineTrimRightSpace {
			// If this line doesn't have a trailing delim, we
			// write the line into buf (along with its newline)
			buf.WriteString(line)
			buf.WriteRune('\n')
			continue
		}

		// Else we did find delims

		// else we've got a separator
		// lineNoSep := strings.TrimSuffix(lineTrimRight, delim)
		buf.WriteString(lineDelimTrimmed)

		// The statement is everything in buf
		stmt := buf.String()
		if strings.TrimSpace(stmt) != "" {
			stmts = append(stmts, stmt)
		}

		buf.Reset()
	}

	// Catch the last line, which may not have a delim suffix
	if buf.Len() > 0 {
		stmts = append(stmts, buf.String())
	}

	for _, stmt := range stmts {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(stmt)), "select") {
			types = append(types, StmtSelect)
		} else {
			types = append(types, StmtOther)
		}
	}

	return stmts, types, nil
}

// trimTrailingDelims iteratively trims trailing whitespace
// and delims from line. If delim starts with a letter, care
// is taken that the delim is only stripped on a word boundary.
// For example, using delim "go":
//
//	"select * from food go"		--> "select * from food"
//	"select * from food2go"		--> "select * from food2go"
//	"select * from food2go go"	--> "select * from food2go"
//
// This implementation is mighty inefficient, don't use on
// the hot path.
func trimTrailingDelims(line string, delims ...string) string {
	working := line

	for {
		for _, delim := range delims {
			if delim == "" {
				// shouldn't happen
				continue
			}
			working = trimDelimSuffix(working, delim)
		}

		if working == "" || working == line {
			break
		}

		line = working
	}

	return working
}

func trimDelimSuffix(line, delim string) (stripped string) {
	if line == "" || line == delim {
		return ""
	}

	// Trim any trailing whitespace
	lineTrimRight := strings.TrimRightFunc(line, unicode.IsSpace)
	if lineTrimRight == "" {
		return ""
	}

	if lineTrimRight == delim {
		return ""
	}

	// lineTrimRight contains at least some text

	// Take the case where delim is "go" and line is "select * from tblgo".
	// We don't want to strip "go", so we verify that the previous
	// rune isn't alphanumeric
	r, _ := utf8.DecodeRuneInString(delim)
	if !unicode.IsLetter(r) {
		// If delim doesn't with a letter, just do the trim.
		// We don't check for delim starting with a number.
		stripped = strings.TrimSuffix(lineTrimRight, delim)
		return stripped
	}

	stripped = strings.TrimSuffix(lineTrimRight, delim)
	if stripped == "" {
		return ""
	}

	// stripped is non-empty
	r, _ = utf8.DecodeLastRuneInString(stripped)
	if unicode.IsLetter(r) || unicode.IsNumber(r) {
		// We can't allow this
		return lineTrimRight
	}

	return stripped
}
