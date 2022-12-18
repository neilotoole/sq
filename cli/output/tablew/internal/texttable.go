// Copyright 2014 Oleku Konko All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// This module is a Table writer  API for the Go Programming Language.
// The protocols were written in pure Go and works on windows and unix systems

// Package tablewriter creates & generates text based table
package internal

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

// MaxRowWidth defines maximum row width
const (
	MaxRowWidth = 30
)

const (
	// BorderCenterChar defines char at center of border
	BorderCenterChar = "+"
	// BorderRowChar defines char of row border
	BorderRowChar = "-"
	// BorderColumnChar defines char of column border
	BorderColumnChar = "|"
	// Empty defines no padding space or border
	Empty = ""
	// Space defines space char for padding and borders
	Space = " "
)

// AlignDefault and other alignment constants
const (
	AlignDefault = iota
	AlignLeft
	AlignCenter
	AlignRight
)

var (
	decimal = regexp.MustCompile(`^-*\d*\.?\d*$`)
	percent = regexp.MustCompile(`^-*\d*\.?\d*$%$`)
)

// Border struct
type Border struct {
	Left   bool
	Right  bool
	Top    bool
	Bottom bool
}

// Table struct
type Table struct {
	out         io.Writer
	rows        [][]string
	lines       [][][]string
	cs          map[int]int
	rs          map[int]int
	headers     []string
	footers     []string
	autoFmt     bool
	autoWrap    bool
	mW          int
	pCenter     string
	pRow        string
	pColumn     string
	tColumn     int
	tRow        int
	hAlign      int
	fAlign      int
	align       int
	rowLine     bool
	hdrLine     bool
	hdrDisable  bool
	borders     Border
	colSize     int
	colTrans    map[int]textTransFunc
	cellTrans   map[string]textTransFunc
	headerTrans textTransFunc
}

// NewTable returns a new table that writes to writer.
func NewTable(writer io.Writer) *Table {
	t := &Table{
		out:         writer,
		rows:        [][]string{},
		lines:       [][][]string{},
		cs:          make(map[int]int),
		rs:          make(map[int]int),
		headers:     []string{},
		footers:     []string{},
		autoFmt:     true,
		autoWrap:    true,
		mW:          MaxRowWidth,
		pCenter:     BorderCenterChar,
		pRow:        BorderRowChar,
		pColumn:     BorderColumnChar,
		tColumn:     -1,
		tRow:        -1,
		hAlign:      AlignDefault,
		fAlign:      AlignDefault,
		align:       AlignDefault,
		rowLine:     false,
		hdrLine:     false,
		hdrDisable:  false,
		borders:     Border{Left: true, Right: true, Bottom: true, Top: true},
		colSize:     -1,
		colTrans:    make(map[int]textTransFunc),
		cellTrans:   make(map[string]textTransFunc),
		headerTrans: fmt.Sprint,
	}
	return t
}

// SetColTrans sets the column transformer.
func (t *Table) SetColTrans(col int, trans textTransFunc) {
	t.colTrans[col] = trans
}

// SetCellTrans sets the cell transformer.
func (t *Table) SetCellTrans(row, col int, trans textTransFunc) {
	t.cellTrans[fmt.Sprintf("[%v][%v]", row, col)] = trans
}

// SetHeaderTrans sets the transformer for the header row.
func (t *Table) SetHeaderTrans(trans textTransFunc) {
	t.headerTrans = trans
}

func (t *Table) getCellTrans(row, col int) textTransFunc {
	colTrans := t.getColTrans(col)
	key := fmt.Sprintf("[%v][%v]", row, col)
	cellTrans := t.cellTrans[key]

	if cellTrans == nil {
		cellTrans = func(val ...any) string {
			return fmt.Sprint(val...)
		}
	}

	return func(val ...any) string {
		return cellTrans(colTrans(val...))
	}
}

func (t *Table) getColTrans(col int) textTransFunc {
	trans := t.colTrans[col]
	if trans != nil {
		return trans
	}

	return func(val ...any) string {
		return fmt.Sprint(val...)
	}
}

// RenderAll table output
func (t *Table) RenderAll() {
	if t.borders.Top {
		t.printLine(true)
	}
	t.printHeading()
	t.printRows()

	if !t.rowLine && t.borders.Bottom {
		t.printLine(true)
	}
	t.printFooter()
}

// SetHeader sets table header
func (t *Table) SetHeader(keys []string) {
	t.colSize = len(keys)
	for i, v := range keys {
		t.parseDimension(v, i, -1)
		t.headers = append(t.headers, v)
	}
}

// SetFooter sets table Footer
func (t *Table) SetFooter(keys []string) {
	// t.colSize = len(keys)
	for i, v := range keys {
		t.parseDimension(v, i, -1)
		t.footers = append(t.footers, v)
	}
}

// SetAutoFormatHeaders turns header autoformatting on/off. Default is on (true).
func (t *Table) SetAutoFormatHeaders(auto bool) {
	t.autoFmt = auto
}

// SetAutoWrapText turns automatic multiline text adjustment on/off. Default is on (true).
func (t *Table) SetAutoWrapText(auto bool) {
	t.autoWrap = auto
}

// SetColWidth sets the default column width
func (t *Table) SetColWidth(width int) {
	t.mW = width
}

// SetColumnSeparator sets the Column Separator
func (t *Table) SetColumnSeparator(sep string) {
	t.pColumn = sep
}

// SetRowSeparator sets the Row Separator
func (t *Table) SetRowSeparator(sep string) {
	t.pRow = sep
}

// SetCenterSeparator sets the center Separator
func (t *Table) SetCenterSeparator(sep string) {
	t.pCenter = sep
}

// SetHeaderAlignment sets Header Alignment
func (t *Table) SetHeaderAlignment(hAlign int) {
	t.hAlign = hAlign
}

// SetFooterAlignment sets Footer Alignment
func (t *Table) SetFooterAlignment(fAlign int) {
	t.fAlign = fAlign
}

// SetAlignment sets Table Alignment
func (t *Table) SetAlignment(align int) {
	t.align = align
}

// SetHeaderLine enable / disable a line after the header
func (t *Table) SetHeaderLine(line bool) {
	t.hdrLine = line
}

// SetDisableHeader enable / disable printing of headers
func (t *Table) SetHeaderDisable(disable bool) {
	t.hdrDisable = disable
}

// SetRowLine enable / disable a line on each row of the table
func (t *Table) SetRowLine(line bool) {
	t.rowLine = line
}

// SetBorder enable / disable line around the table
func (t *Table) SetBorder(border bool) {
	t.SetBorders(Border{border, border, border, border})
}

// SetBorders sets borders
func (t *Table) SetBorders(border Border) {
	t.borders = border
}

// Append row to table
func (t *Table) Append(row []string) {
	rowSize := len(t.headers)
	if rowSize > t.colSize {
		t.colSize = rowSize
	}

	n := len(t.lines)
	line := [][]string{}
	for i, v := range row {
		// Detect string  width
		// Detect String height
		// Break strings into words
		out := t.parseDimension(v, i, n)

		// Append broken words
		line = append(line, out)
	}
	t.lines = append(t.lines, line)
}

// AppendBulk allows support for Bulk Append
// Eliminates repeated for loops
func (t *Table) AppendBulk(rows [][]string) {
	for _, row := range rows {
		t.Append(row)
	}
}

// Print line based on row width
func (t *Table) printLine(nl bool) {
	fmt.Fprint(t.out, t.pCenter)
	for i := 0; i < len(t.cs); i++ {
		v := t.cs[i]
		fmt.Fprintf(t.out, "%s%s%s%s",
			t.pRow,
			strings.Repeat(string(t.pRow), v),
			t.pRow,
			t.pCenter)
	}
	if nl {
		fmt.Fprintln(t.out)
	}
}

// Return the PadRight function if align is left, PadLeft if align is right,
// and Pad by default
func pad(align int) func(string, string, int) string {
	padFunc := Pad
	switch align {
	case AlignLeft:
		padFunc = PadRight
	case AlignRight:
		padFunc = PadLeft
	}
	return padFunc
}

// Print heading information
func (t *Table) printHeading() {
	// Check if headers is available
	if len(t.headers) < 1 || t.hdrDisable {
		return
	}

	// Check if border is set
	// Replace with space if not set
	fmt.Fprint(t.out, ConditionString(t.borders.Left, t.pColumn, Empty))

	// Identify last column
	end := len(t.cs) - 1

	// Get pad function
	padFunc := pad(t.hAlign)

	// Print Heading column
	for i := 0; i <= end; i++ {
		v := t.cs[i]
		h := t.headers[i]
		if t.autoFmt {
			h = Title(h)
		}
		pad := ConditionString(i == end && !t.borders.Left, Space, t.pColumn)

		head := t.headerTrans(fmt.Sprintf("%s %s ", padFunc(h, Space, v), pad))
		fmt.Fprint(t.out, head)

	}
	// Next line
	fmt.Fprintln(t.out)
	if t.hdrLine {
		t.printLine(true)
	}
}

// Print heading information
func (t *Table) printFooter() {
	// Check if headers is available
	if len(t.footers) < 1 {
		return
	}

	// Only print line if border is not set
	if !t.borders.Bottom {
		t.printLine(true)
	}
	// Check if border is set
	// Replace with space if not set
	fmt.Fprint(t.out, ConditionString(t.borders.Bottom, t.pColumn, Space))

	// Identify last column
	end := len(t.cs) - 1

	// Get pad function
	padFunc := pad(t.fAlign)

	// Print Heading column
	for i := 0; i <= end; i++ {
		v := t.cs[i]
		f := t.footers[i]
		if t.autoFmt {
			f = Title(f)
		}
		pad := ConditionString(i == end && !t.borders.Top, Space, t.pColumn)

		if len(t.footers[i]) == 0 {
			pad = Space
		}
		fmt.Fprintf(t.out, " %s %s",
			padFunc(f, Space, v),
			pad)
	}
	// Next line
	fmt.Fprintln(t.out)

	hasPrinted := false

	for i := 0; i <= end; i++ {
		v := t.cs[i]
		pad := t.pRow
		center := t.pCenter
		length := len(t.footers[i])

		if length > 0 {
			hasPrinted = true
		}

		// Set center to be space if length is 0
		if length == 0 && !t.borders.Right {
			center = Space
		}

		// Print first junction
		if i == 0 {
			fmt.Fprint(t.out, center)
		}

		// Pad With space of length is 0
		if length == 0 {
			pad = Space
		}
		// Ignore left space of it has printed before
		if hasPrinted || t.borders.Left {
			pad = t.pRow
			center = t.pCenter
		}

		// Change Center start position
		if center == Space {
			if i < end && len(t.footers[i+1]) != 0 {
				center = t.pCenter
			}
		}

		// Print the footer
		fmt.Fprintf(t.out, "%s%s%s%s",
			pad,
			strings.Repeat(string(pad), v),
			pad,
			center)

	}

	fmt.Fprintln(t.out)
}

func (t *Table) printRows() {
	for i, lines := range t.lines {
		t.printRow(lines, i)
	}
}

// Print Row Information
func (t *Table) printRow(columns [][]string, colKey int) {
	// Get Maximum Height
	max := t.rs[colKey]
	total := len(columns)

	// Pad Each Height
	for i, line := range columns {
		length := len(line)
		pad := max - length
		for n := 0; n < pad; n++ {
			columns[i] = append(columns[i], "  ")
		}
	}

	for x := 0; x < max; x++ {
		for y := 0; y < total; y++ {
			// Check if border is set
			fmt.Fprint(t.out, ConditionString(!t.borders.Left && y == 0, Empty, t.pColumn))

			text := columns[y][x]

			tran := t.getCellTrans(colKey, y)
			// This would print alignment
			// Default alignment  would use multiple configuration
			switch t.align {
			case AlignCenter: //
				fmt.Fprintf(t.out, "%s", Pad(text, Space, t.cs[y]))
			case AlignRight:
				fmt.Fprintf(t.out, "%s", PadLeft(text, Space, t.cs[y]))
			case AlignLeft:
				cellContent := text
				if y != total-1 {
					cellContent = PadRight(text, Space, t.cs[y])
				}
				fmt.Fprintf(t.out, tran("%s "), cellContent)
			default:
				if decimal.MatchString(strings.TrimSpace(text)) || percent.MatchString(strings.TrimSpace(text)) {
					fmt.Fprintf(t.out, "%s", PadLeft(text, Space, t.cs[y]))
				} else {
					fmt.Fprintf(t.out, "%s", PadRight(text, Space, t.cs[y]))
				}
			}
			fmt.Fprint(t.out, Space)
		}
		// Check if border is set
		// Replace with space if not set
		fmt.Fprint(t.out, ConditionString(t.borders.Left, t.pColumn, Space))
		fmt.Fprintln(t.out)
	}

	if t.rowLine {
		t.printLine(true)
	}
}

func (t *Table) parseDimension(str string, colKey, rowKey int) []string {
	var (
		raw []string
		max int
	)
	w := DisplayWidth(str)
	// Calculate Width
	// Check if with is grater than maximum width
	if w > t.mW {
		w = t.mW
	}

	// Check if width exists
	v, ok := t.cs[colKey]
	if !ok || v < w || v == 0 {
		t.cs[colKey] = w
	}

	if rowKey == -1 {
		return raw
	}
	// Calculate Height
	if t.autoWrap {
		raw, _ = WrapString(str, t.cs[colKey])
	} else {
		raw = getLines(str)
	}

	for _, line := range raw {
		if w := DisplayWidth(line); w > max {
			max = w
		}
	}

	// Make sure the with is the same length as maximum word
	// Important for cases where the width is smaller than maxu word
	if max > t.cs[colKey] {
		t.cs[colKey] = max
	}

	h := len(raw)
	v, ok = t.rs[rowKey]

	if !ok || v < h || v == 0 {
		t.rs[rowKey] = h
	}
	return raw
}

// textTransFunc is a function that can transform text, typically
// to add color.
type textTransFunc func(a ...any) string
