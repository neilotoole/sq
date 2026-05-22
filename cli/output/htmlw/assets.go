package htmlw

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"io"
	"sync"

	"github.com/neilotoole/sq/libsq/core/errz"
)

//go:embed assets/mermaid.min.js.gz
var mermaidGz []byte

var (
	mermaidOnce sync.Once
	mermaidSrc  []byte
	errMermaid  error
)

// mermaidJS returns the decompressed vendored Mermaid UMD bundle, used for
// self-contained (embed) HTML output. The decompressed bytes are cached
// after the first call.
func mermaidJS() ([]byte, error) {
	mermaidOnce.Do(func() {
		gz, err := gzip.NewReader(bytes.NewReader(mermaidGz))
		if err != nil {
			errMermaid = errz.Err(err)
			return
		}
		defer func() { _ = gz.Close() }()
		mermaidSrc, errMermaid = io.ReadAll(gz)
		errMermaid = errz.Err(errMermaid)
	})
	return mermaidSrc, errMermaid
}
