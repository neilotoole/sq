// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package udiff

import (
	"bytes"
	"unicode/utf8"

	"github.com/neilotoole/sq/cli/diff/libdiff/internal/go-udiff/lcs"
)

// Strings computes the differences between two strings.
// The resulting edits respect rune boundaries.
func Strings(before, after string) []Edit {
	if before == after {
		return nil // common case
	}

	if stringIsASCII(before) && stringIsASCII(after) {
		// TODO(adonovan): opt: specialize diffASCII for strings.
		return diffASCII([]byte(before), []byte(after))
	}
	return diffRunes([]rune(before), []rune(after))
}

// Bytes computes the differences between two byte slices.
// The resulting edits respect rune boundaries.
func Bytes(before, after []byte) []Edit {
	if bytes.Equal(before, after) {
		return nil // common case
	}

	if bytesIsASCII(before) && bytesIsASCII(after) {
		return diffASCII(before, after)
	}
	return diffRunes(runes(before), runes(after))
}

func diffASCII(before, after []byte) []Edit {
	diffs := lcs.DiffBytes(before, after)

	// Convert from LCS diffs.
	res := make([]Edit, len(diffs))
	for i, d := range diffs {
		res[i] = Edit{Start: d.Start, End: d.End, New: string(after[d.ReplStart:d.ReplEnd])}
	}
	return res
}

func diffRunes(before, after []rune) []Edit {
	diffs := lcs.DiffRunes(before, after)

	// The diffs returned by the lcs package use indexes
	// into whatever slice was passed in.
	// Convert rune offsets to byte offsets.
	res := make([]Edit, len(diffs))
	lastEnd := 0
	utf8Len := 0
	for i, d := range diffs {
		utf8Len += runesLen(before[lastEnd:d.Start]) // text between edits
		start := utf8Len
		utf8Len += runesLen(before[d.Start:d.End]) // text deleted by this edit
		res[i] = Edit{Start: start, End: utf8Len, New: string(after[d.ReplStart:d.ReplEnd])}
		lastEnd = d.End
	}
	return res
}

// runes is like []rune(string(bytes)) without the duplicate allocation.
func runes(b []byte) []rune {
	n := utf8.RuneCount(b)
	rs := make([]rune, n)
	for i := 0; i < n; i++ {
		r, sz := utf8.DecodeRune(b)
		b = b[sz:]
		rs[i] = r
	}
	return rs
}

// runesLen returns the length in bytes of the UTF-8 encoding of runes.
func runesLen(runes []rune) (length int) {
	for _, r := range runes {
		length += utf8.RuneLen(r)
	}
	return length
}

// stringIsASCII reports whether s contains only ASCII.
// TODO(adonovan): combine when x/tools allows generics.
func stringIsASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func bytesIsASCII(s []byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}
