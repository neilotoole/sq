package internal_test

import (
	stdj "encoding/json"
	"io"
	"testing"

	segmentj "github.com/segmentio/encoding/json"

	jcolorenc "github.com/neilotoole/sq/cli/output/jsonw/internal/jcolorenc"

	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// The following benchmarks compare the encoding performance
// of JSON encoders. These are:
//
// - stdj: the std lib json encoder
// - segmentj: the encoder by segment.io
// - jcolorenc: sq's fork of segmentj that supports color

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

func BenchmarkJColorEnc(b *testing.B) {
	_, recs := testh.RecordsFromTbl(b, sakila.SL3, "payment")
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		enc := jcolorenc.NewEncoder(io.Discard)
		enc.SetEscapeHTML(false)

		for i := range recs {
			err := enc.Encode(recs[i])
			if err != nil {
				b.Error(err)
			}
		}
	}
}

func BenchmarkJColorEnc_Indent(b *testing.B) {
	_, recs := testh.RecordsFromTbl(b, sakila.SL3, "payment")
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		enc := jcolorenc.NewEncoder(io.Discard)
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
