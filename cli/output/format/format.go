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
	case JSON, JSONA, JSONL, Text, Raw,
		HTML, Markdown, XLSX, XML,
		CSV, TSV, YAML:
	case "table":
		// Legacy: the "text" format used to be named "table".
		text = []byte(Text)
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
	Text     Format = "text"
	JSON     Format = "json"
	JSONL    Format = "jsonl"
	JSONA    Format = "jsona"
	HTML     Format = "html"
	Markdown Format = "markdown"
	XLSX     Format = "xlsx"
	XML      Format = "xml"
	CSV      Format = "csv"
	TSV      Format = "tsv"
	Raw      Format = "raw"
	YAML     Format = "yaml"
)

// All returns a new slice containing all format.Format values.
func All() []Format {
	return []Format{
		Text,
		JSON,
		JSONL,
		JSONA,
		Raw,
		HTML,
		Markdown,
		XLSX,
		XML,
		CSV,
		TSV,
		YAML,
	}
}
