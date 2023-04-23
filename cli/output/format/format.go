package format

import (
	"github.com/neilotoole/sq/libsq/core/errz"
)

// Format is an output format such as json or xml.
type Format string

// UnmarshalText implements encoding.TextUnmarshaler.
func (f *Format) UnmarshalText(text []byte) error {
	switch Format(text) {
	default:
		return errz.Errorf("unknown output format {%s}", string(text))
	case JSON, JSONA, JSONL, Table, Raw,
		HTML, Markdown, XLSX, XML,
		CSV, TSV, YAML:
	}

	*f = Format(text)
	return nil
}

// String returns the format value.
func (f Format) String() string {
	return string(f)
}

// Output format values.
const (
	JSON     Format = "json"
	JSONL    Format = "jsonl"
	JSONA    Format = "jsona"
	Table    Format = "table"
	Raw      Format = "raw"
	HTML     Format = "html"
	Markdown Format = "markdown"
	XLSX     Format = "xlsx"
	XML      Format = "xml"
	CSV      Format = "csv"
	TSV      Format = "tsv"
	YAML     Format = "yaml"
)
