// Package json implements the sq driver for JSON. There are three
// supported types:
// - JSON: plain old JSON
// - JSONA: JSON Array, where each record is an array of JSON values on its own line.
// - JSONL: JSON Lines, where each record a JSON object on its own line.
package json

import (
	"bufio"
	"context"
	"io"

	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/source"
)

const (
	// TypeJSON is the plain-old JSON driver type.
	TypeJSON = source.Type("json")

	// TypeJSONA is the JSON Array driver type.
	TypeJSONA = source.Type("jsona")

	// TypeJSONL is the JSON Lines driver type.
	TypeJSONL = source.Type("jsonl")
)

// DetectJSON implements source.TypeDetectorFunc.
func DetectJSON(ctx context.Context, rdrs source.Readers) (detected source.Type, score float32, err error) {
	return source.TypeNone, 0, errz.New("not implemented")
}

// DetectJSONA implements source.TypeDetectorFunc for TypeJSONA.
// Each line of input must be a valid JSON array.
func DetectJSONA(ctx context.Context, rdrs source.Readers) (detected source.Type, score float32, err error) {
	var r io.Reader
	r, err = rdrs.Open()
	if err != nil {
		return source.TypeNone, 0, errz.Err(err)
	}
	defer func() { err = errz.Combine(err, rdrs.Close()) }()

	sc := bufio.NewScanner(r)
	var lineCount int
	var line []byte

	if sc.Scan() {
		if err = sc.Err(); err != nil {
			return source.TypeNone, 0, errz.Err(err)
		}
		lineCount++

		line = sc.Bytes()
		if len(line) == 0 {
			return
		}

	}

	if lineCount == 0 {
		return source.TypeNone, 0, nil
	}

	return source.TypeNone, 0, nil
}

// DetectJSONL implements source.TypeDetectorFunc.
func DetectJSONL(ctx context.Context, rdrs source.Readers) (detected source.Type, score float32, err error) {
	return source.TypeNone, 0, errz.New("not implemented")
}
