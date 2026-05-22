package htmlw

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"slices"

	"github.com/samber/lo"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

var _ output.MetadataWriter = (*metadataWriter)(nil)

// metadataWriter implements output.MetadataWriter for HTML. It renders
// source and table metadata as a standalone HTML document, including
// Mermaid.js entity-relationship diagrams. embed controls whether the
// Mermaid library is inlined (self-contained/offline) or loaded from a CDN.
type metadataWriter struct {
	out   io.Writer
	pr    *output.Printing
	embed bool
}

// NewMetadataWriter returns an output.MetadataWriter that renders metadata
// as an HTML document. When embed is true, the Mermaid.js library is inlined
// so the document renders offline; otherwise it is loaded from a CDN.
func NewMetadataWriter(out io.Writer, pr *output.Printing, embed bool) output.MetadataWriter {
	return &metadataWriter{out: out, pr: pr, embed: embed}
}

const cssStyles = `:root { color-scheme: light dark; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
  line-height: 1.5; max-width: 70rem; margin: 2rem auto; padding: 0 1rem; }
h1, h2, h3, h4 { line-height: 1.25; }
.sq-table { font-size: 1.6rem; margin-top: 2.5rem; padding-top: 1.25rem;
  border-top: 2px solid #d0d0d0; }
.sq-table code { font-size: inherit; background: none; padding: 0; }
.sq-anchor { color: inherit; text-decoration: none; }
.sq-anchor:hover::after { content: "#"; margin-left: 0.4em; color: #888;
  font-weight: normal; }
table { border-collapse: collapse; margin: 1rem 0; }
caption { text-align: left; font-weight: 600; margin-bottom: 0.3rem; }
th, td { border: 1px solid #ccc; padding: 0.3rem 0.6rem; text-align: left; vertical-align: top; }
th { background: #f2f2f2; }
code { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  background: rgba(127,127,127,0.15); padding: 0.1em 0.3em; border-radius: 3px; }
pre.mermaid { background: #fff; border: 1px solid #ddd; border-radius: 8px;
  padding: 1rem; overflow-x: auto; cursor: zoom-in; }
.summary { color: #666; margin: 0.2rem 0; }
@media (prefers-color-scheme: dark) {
  body { background: #1e1e1e; color: #ddd; }
  th, td { border-color: #444; }
  th { background: #2a2a2a; }
  .summary { color: #aaa; }
  .sq-table { border-top-color: #444; }
}
#sq-erd-overlay { position: fixed; inset: 0; z-index: 1000; display: none;
  background: rgba(0,0,0,0.85); }
#sq-erd-overlay.open { display: block; }
#sq-erd-overlay .sq-erd-stage { position: absolute; inset: 0; overflow: hidden;
  background: #fff; cursor: grab; }
#sq-erd-overlay .sq-erd-stage svg { width: 100%; height: 100%; max-width: none; }
#sq-erd-overlay .sq-erd-bar { position: absolute; top: 0.75rem; right: 0.75rem;
  display: flex; gap: 0.5rem; z-index: 1; }
#sq-erd-overlay .sq-erd-bar button { font: inherit; cursor: pointer; color: #111;
  background: #fff; border: 1px solid #ccc; border-radius: 6px; padding: 0.3rem 0.7rem; }
`

// writeDocument writes a complete standalone HTML document: <head> (charset,
// title, inlined stylesheet, and — when withMermaid — the Mermaid script),
// then <body> built by body, then the closing tags.
func (w *metadataWriter) writeDocument(
	buf *bytes.Buffer, title string, withMermaid bool, body func(*bytes.Buffer),
) error {
	buf.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n")
	buf.WriteString("<meta charset=\"utf-8\">\n")
	buf.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(buf, "<title>%s</title>\n", html.EscapeString(title))
	buf.WriteString("<style>\n")
	buf.WriteString(cssStyles)
	buf.WriteString("</style>\n")
	if withMermaid {
		if err := w.writeMermaidScript(buf); err != nil {
			return err
		}
	}
	buf.WriteString("</head>\n<body>\n")
	body(buf)
	buf.WriteString("</body>\n</html>\n")
	return nil
}

// mermaidInit is the Mermaid initialization call. It always uses the default
// (light) theme; the diagram is rendered on a light panel (see cssStyles
// pre.mermaid) so it stays legible even when the page is in dark mode. We
// don't use Mermaid's "dark" theme because in v11 it leaves the ER focal
// table's header and attribute-row backgrounds light while switching text to
// light (unreadable), and those colors aren't reachable via themeVariables.
//
// erdOverlayJS is appended after it: a vanilla-JS module that makes each
// rendered diagram open in a fullscreen, panzoom-able overlay on click. It
// uses event delegation, so it is independent of Mermaid's async render
// timing, and references panzoom, which writeMermaidScript loads alongside
// Mermaid (vendored UMD global in embed mode, ESM import otherwise).
const mermaidInit = "mermaid.initialize({ startOnLoad: true });\n"

const erdOverlayJS = `(function(){
  var overlay, stage, pz;
  function build(){
    overlay = document.createElement('div');
    overlay.id = 'sq-erd-overlay';
    var bar = document.createElement('div'); bar.className = 'sq-erd-bar';
    var reset = document.createElement('button'); reset.type = 'button'; reset.textContent = 'Reset';
    var close = document.createElement('button'); close.type = 'button'; close.textContent = '✕ Close';
    bar.appendChild(reset); bar.appendChild(close);
    stage = document.createElement('div'); stage.className = 'sq-erd-stage';
    overlay.appendChild(bar); overlay.appendChild(stage);
    document.body.appendChild(overlay);
    close.addEventListener('click', hide);
    reset.addEventListener('click', function(){ if (pz) { pz.moveTo(0,0); pz.zoomAbs(0,0,1); } });
    overlay.addEventListener('click', function(e){ if (e.target === overlay) hide(); });
  }
  function show(svg){
    if (!overlay) build();
    stage.textContent = '';
    // Clone keeps its id: Mermaid scopes its internal <style> rules (e.g.
    // fill:none on relationship paths) to "#<svg-id> ...", so dropping the id
    // would make every edge render as a solid black blob. The resulting
    // duplicate id is invalid but harmless here. Only the inline sizing style
    // is stripped so the clone can fill the stage.
    var clone = svg.cloneNode(true);
    clone.removeAttribute('style');
    stage.appendChild(clone);
    overlay.classList.add('open');
    pz = panzoom(clone, { maxZoom: 40, minZoom: 0.1, smoothScroll: false });
  }
  function hide(){
    if (pz) { pz.dispose(); pz = null; }
    if (overlay) { overlay.classList.remove('open'); stage.textContent = ''; }
  }
  document.addEventListener('click', function(e){
    if (overlay && overlay.classList.contains('open')) return;
    var pre = e.target.closest ? e.target.closest('pre.mermaid') : null;
    if (!pre) return;
    var svg = pre.querySelector('svg');
    if (svg) show(svg);
  });
  document.addEventListener('keydown', function(e){ if (e.key === 'Escape') hide(); });
})();
`

// writeMermaidScript writes the <script>s that load and initialize Mermaid and
// panzoom and wire up the click-to-zoom overlay: inlined vendored bundles when
// embed is set, else pinned CDN imports.
func (w *metadataWriter) writeMermaidScript(buf *bytes.Buffer) error {
	if w.embed {
		mjs, err := mermaidJS()
		if err != nil {
			return err
		}
		pjs, err := panzoomJS()
		if err != nil {
			return err
		}
		buf.WriteString("<script>")
		buf.Write(mjs)
		buf.WriteString("</script>\n<script>")
		buf.Write(pjs)
		buf.WriteString("</script>\n<script>")
		buf.WriteString(mermaidInit)
		buf.WriteString(erdOverlayJS)
		buf.WriteString("</script>\n")
		return nil
	}
	buf.WriteString("<script type=\"module\">\n")
	buf.WriteString("import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@11/+esm';\n")
	buf.WriteString("import panzoom from 'https://cdn.jsdelivr.net/npm/panzoom@9/+esm';\n")
	buf.WriteString(mermaidInit)
	buf.WriteString(erdOverlayJS)
	buf.WriteString("</script>\n")
	return nil
}

// htmlCode wraps s in <code>, HTML-escaped. Empty input yields "".
func htmlCode(s string) string {
	if s == "" {
		return ""
	}
	return "<code>" + html.EscapeString(s) + "</code>"
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// checkMark returns "✓" when b is true, else "" — for boolean table cells
// such as an index's Unique / Primary columns.
func checkMark(b bool) string {
	if b {
		return "✓"
	}
	return ""
}

// writeTableEl writes a simple <table> with the given headers and rows, and an
// optional <caption> (omitted when caption is ""). When captionID is non-empty
// the caption is rendered as a deep-linkable self-link (id + #captionID). The
// caption is HTML-escaped; each cell is written verbatim (callers pre-escape).
func writeTableEl(buf *bytes.Buffer, caption, captionID string, headers []string, rows [][]string) {
	buf.WriteString("<table>\n")
	switch {
	case caption == "":
	case captionID == "":
		fmt.Fprintf(buf, "<caption>%s</caption>\n", html.EscapeString(caption))
	default:
		// Deep-linkable caption: an id + a self-link that reveals a "#" on
		// hover (same .sq-anchor affordance as the table headings).
		fmt.Fprintf(buf,
			"<caption id=\"%s\"><a class=\"sq-anchor\" href=\"#%s\">%s</a></caption>\n",
			captionID, captionID, html.EscapeString(caption))
	}
	buf.WriteString("<thead>\n<tr>")
	for _, h := range headers {
		fmt.Fprintf(buf, "<th>%s</th>", html.EscapeString(h))
	}
	buf.WriteString("</tr>\n</thead>\n<tbody>\n")
	for _, row := range rows {
		buf.WriteString("<tr>")
		for _, cell := range row {
			fmt.Fprintf(buf, "<td>%s</td>", cell)
		}
		buf.WriteString("</tr>\n")
	}
	buf.WriteString("</tbody>\n</table>\n")
}

// DBProperties implements output.MetadataWriter.
func (w *metadataWriter) DBProperties(props map[string]any) error {
	if len(props) == 0 {
		return nil
	}
	buf := &bytes.Buffer{}
	err := w.writeDocument(buf, "Database properties", false, func(b *bytes.Buffer) {
		keys := lo.Keys(props)
		slices.Sort(keys)
		var rows [][]string
		for _, k := range keys {
			v := props[k]
			if v == nil {
				continue
			}
			rows = append(rows, []string{htmlCode(k), html.EscapeString(fmt.Sprintf("%v", v))})
		}
		writeTableEl(b, "", "", []string{"Property", "Value"}, rows)
	})
	if err != nil {
		return err
	}
	_, err = buf.WriteTo(w.out)
	return err
}

// DriverMetadata implements output.MetadataWriter.
func (w *metadataWriter) DriverMetadata(drvrs []driver.Metadata) error {
	if len(drvrs) == 0 {
		return nil
	}
	buf := &bytes.Buffer{}
	err := w.writeDocument(buf, "Drivers", false, func(b *bytes.Buffer) {
		var rows [][]string
		for _, md := range drvrs {
			rows = append(rows, []string{
				htmlCode(string(md.Type)),
				html.EscapeString(md.Description),
				yesNo(md.UserDefined),
			})
		}
		writeTableEl(b, "", "", []string{"Driver", "Description", "User-defined"}, rows)
	})
	if err != nil {
		return err
	}
	_, err = buf.WriteTo(w.out)
	return err
}

// Catalogs implements output.MetadataWriter.
func (w *metadataWriter) Catalogs(currentCatalog string, catalogs []string) error {
	if len(catalogs) == 0 {
		return nil
	}
	buf := &bytes.Buffer{}
	err := w.writeDocument(buf, "Catalogs", false, func(b *bytes.Buffer) {
		var rows [][]string
		for _, c := range catalogs {
			active := ""
			if c == currentCatalog {
				active = "✓"
			}
			rows = append(rows, []string{htmlCode(c), active})
		}
		writeTableEl(b, "", "", []string{"Catalog", "Active"}, rows)
	})
	if err != nil {
		return err
	}
	_, err = buf.WriteTo(w.out)
	return err
}

// Schemata implements output.MetadataWriter.
func (w *metadataWriter) Schemata(currentSchema string, schemas []*metadata.Schema) error {
	if len(schemas) == 0 {
		return nil
	}
	buf := &bytes.Buffer{}
	err := w.writeDocument(buf, "Schemas", false, func(b *bytes.Buffer) {
		var rows [][]string
		for _, s := range schemas {
			active := ""
			if s.Name == currentSchema {
				active = "✓"
			}
			rows = append(rows, []string{
				htmlCode(s.Name), htmlCode(s.Catalog), html.EscapeString(s.Owner), active,
			})
		}
		writeTableEl(b, "", "", []string{"Schema", "Catalog", "Owner", "Active"}, rows)
	})
	if err != nil {
		return err
	}
	_, err = buf.WriteTo(w.out)
	return err
}
