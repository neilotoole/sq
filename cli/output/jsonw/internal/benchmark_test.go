package internal_test

import (
	stdj "encoding/json"
	"io"
	"testing"

	segmentj "github.com/segmentio/encoding/json"

	"github.com/neilotoole/jsoncolor"

	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// The following benchmarks compare the encoding performance
// of JSON encoders. These are:
//
// - stdj: the std lib json encoder
// - segmentj: the encoder by segment.io
// - jsoncolor: github.com/neilotoole/jsoncolor (color-capable fork of segmentj)
//
// The three-way comparison runs every encoder without colors, which is what
// makes it apples-to-apples: stdj and segmentj cannot colorize at all. So
// BenchmarkJSONColor names the library, not colorized output, and it measures
// jsoncolor's colorless path (the one sq uses when output is redirected).
//
// The colorized path, which sq uses when writing to a terminal, is measured
// separately by the BenchmarkJSONColor_Colorized benchmarks below. Those have
// no stdj or segmentj counterpart by definition.

func BenchmarkStdj(b *testing.B) {
	_, recs := testh.RecordsFromTbl(b, sakila.SL3, "payment")
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		enc := stdj.NewEncoder(io.Discard)
		enc.SetEscapeHTML(false)

		for i := range recs {
			err := enc.Encode(recs[i])
			if err != nil {
				b.Error(err)
			}
		}
	}
}

func BenchmarkStdj_Indent(b *testing.B) {
	_, recs := testh.RecordsFromTbl(b, sakila.SL3, "payment")
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		enc := stdj.NewEncoder(io.Discard)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")

		for i := range recs {
			err := enc.Encode(recs[i])
			if err != nil {
				b.Error(err)
			}
		}
	}
}

func BenchmarkSegmentj(b *testing.B) {
	_, recs := testh.RecordsFromTbl(b, sakila.SL3, "payment")
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		enc := segmentj.NewEncoder(io.Discard)
		enc.SetEscapeHTML(false)

		for i := range recs {
			err := enc.Encode(recs[i])
			if err != nil {
				b.Error(err)
			}
		}
	}
}

func BenchmarkSegmentj_Indent(b *testing.B) {
	_, recs := testh.RecordsFromTbl(b, sakila.SL3, "payment")
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		enc := segmentj.NewEncoder(io.Discard)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")

		for i := range recs {
			err := enc.Encode(recs[i])
			if err != nil {
				b.Error(err)
			}
		}
	}
}

func BenchmarkJSONColor(b *testing.B) {
	_, recs := testh.RecordsFromTbl(b, sakila.SL3, "payment")
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		enc := jsoncolor.NewEncoder(io.Discard)
		enc.SetEscapeHTML(false)

		for i := range recs {
			err := enc.Encode(recs[i])
			if err != nil {
				b.Error(err)
			}
		}
	}
}

func BenchmarkJSONColor_Indent(b *testing.B) {
	_, recs := testh.RecordsFromTbl(b, sakila.SL3, "payment")
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		enc := jsoncolor.NewEncoder(io.Discard)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")

		for i := range recs {
			err := enc.Encode(recs[i])
			if err != nil {
				b.Error(err)
			}
		}
	}
}

// BenchmarkJSONColor_Colorized measures jsoncolor's colorized path, which sq
// uses when writing to a terminal. The colorless BenchmarkJSONColor above does
// not exercise it: without SetColors, the encoder takes a fast path that skips
// per-token color dispatch entirely.
//
// The palette is jsoncolor.DefaultColors rather than one built from
// output.Printing. What matters here is that every token type carries a
// prefix, and using the library default keeps this benchmark from needing its
// own copy of the Printing-to-Colors mapping.
func BenchmarkJSONColor_Colorized(b *testing.B) {
	_, recs := testh.RecordsFromTbl(b, sakila.SL3, "payment")
	clrs := jsoncolor.DefaultColors()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		enc := jsoncolor.NewEncoder(io.Discard)
		enc.SetEscapeHTML(false)
		enc.SetColors(clrs)

		for i := range recs {
			err := enc.Encode(recs[i])
			if err != nil {
				b.Error(err)
			}
		}
	}
}

// BenchmarkJSONColor_Colorized_Indent is the colorized path with indentation,
// matching sq's default pretty-printed terminal output.
func BenchmarkJSONColor_Colorized_Indent(b *testing.B) {
	_, recs := testh.RecordsFromTbl(b, sakila.SL3, "payment")
	clrs := jsoncolor.DefaultColors()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		enc := jsoncolor.NewEncoder(io.Discard)
		enc.SetEscapeHTML(false)
		enc.SetColors(clrs)
		enc.SetIndent("", "  ")

		for i := range recs {
			err := enc.Encode(recs[i])
			if err != nil {
				b.Error(err)
			}
		}
	}
}
