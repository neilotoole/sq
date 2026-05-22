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

//go:embed assets/panzoom.min.js.gz
var panzoomGz []byte

// gzAsset lazily decompresses an embedded gzipped asset, caching the
// decompressed bytes (and any error) after the first call.
type gzAsset struct { //nolint:govet // field order reads better than the alignment-optimal one
	gz   []byte
	once sync.Once
	src  []byte
	err  error
}

func (a *gzAsset) bytes() ([]byte, error) {
	a.once.Do(func() {
		gz, err := gzip.NewReader(bytes.NewReader(a.gz))
		if err != nil {
			a.err = errz.Err(err)
			return
		}
		defer func() { _ = gz.Close() }()
		a.src, a.err = io.ReadAll(gz)
		a.err = errz.Err(a.err)
	})
	return a.src, a.err
}

var (
	mermaidAsset = &gzAsset{gz: mermaidGz}
	panzoomAsset = &gzAsset{gz: panzoomGz}
)

// mermaidJS returns the decompressed vendored Mermaid UMD bundle, used for
// self-contained (embed) HTML output.
func mermaidJS() ([]byte, error) { return mermaidAsset.bytes() }

// panzoomJS returns the decompressed vendored panzoom UMD bundle, used for
// self-contained (embed) HTML output to power the click-to-zoom diagram
// overlay.
func panzoomJS() ([]byte, error) { return panzoomAsset.bytes() }
