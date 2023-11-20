// Copyright 2014 Oleku Konko All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// This module is a Table writer  API for the Go Programming Language.
// The protocols were written in pure Go and works on windows and unix systems

package internal

import (
	"regexp"
	"strings"

	runewidth "github.com/mattn/go-runewidth"
)

var ansi = regexp.MustCompile("\033\\[(?:[0-9]{1,3}(?:;[0-9]{1,3})*)?[m|K]")

// DisplayWidth returns display width
func DisplayWidth(str string) int {
	return runewidth.StringWidth(ansi.ReplaceAllLiteralString(str, ""))
}

// ConditionString returns valid or invalid string based on condition
func ConditionString(cond bool, valid, inValid string) string {
	if cond {
		return valid
	}
	return inValid
}

// Title format table header, replaces _ , . and spaces
func Title(name string) string {
	name = strings.Replace(name, "_", " ", -1)
	name = strings.Replace(name, ".", " ", -1)
	name = strings.TrimSpace(name)
	return strings.ToUpper(name)
}

// Pad string
// Attempts to play string in the center
func Pad(s, pad string, width int) string {
	gap := width - DisplayWidth(s)
	if gap > 0 {
		gapLeft := int(float64(gap / 2))
		gapRight := gap - gapLeft
		return strings.Repeat(pad, gapLeft) + s + strings.Repeat(pad, gapRight)
	}
	return s
}

// PadRight paces string at the left side fo the screen
func PadRight(s, pad string, width int) string {
	gap := width - DisplayWidth(s)
	if gap > 0 {
		return s + strings.Repeat(pad, gap)
	}
	return s
}

// PadLeft paces string at the right side fo the screen
func PadLeft(s, pad string, width int) string {
	gap := width - DisplayWidth(s)
	if gap > 0 {
		return strings.Repeat(string(pad), gap) + s
	}
	return s
}
