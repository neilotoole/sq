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
table { border-collapse: collapse; margin: 1rem 0; }
th, td { border: 1px solid #ccc; padding: 0.3rem 0.6rem; text-align: left; vertical-align: top; }
th { background: #f2f2f2; }
code { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  background: rgba(127,127,127,0.15); padding: 0.1em 0.3em; border-radius: 3px; }
pre.mermaid { background: transparent; }
.summary { color: #666; margin: 0.2rem 0; }
@media (prefers-color-scheme: dark) {
  body { background: #1e1e1e; color: #ddd; }
  th, td { border-color: #444; }
  th { background: #2a2a2a; }
  .summary { color: #aaa; }
}
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

// writeMermaidScript writes the <script> that loads and initializes Mermaid:
// the inlined vendored bundle when embed is set, else a pinned CDN import.
func (w *metadataWriter) writeMermaidScript(buf *bytes.Buffer) error {
	if w.embed {
		js, err := mermaidJS()
		if err != nil {
			return err
		}
		buf.WriteString("<script>")
		buf.Write(js)
		buf.WriteString("</script>\n")
		buf.WriteString("<script>mermaid.initialize({ startOnLoad: true });</script>\n")
		return nil
	}
	buf.WriteString("<script type=\"module\">\n")
	buf.WriteString("import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@11/+esm';\n")
	buf.WriteString("mermaid.initialize({ startOnLoad: true });\n")
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

// writeTableEl writes a simple <table> with the given headers and rows. Each
// cell is written verbatim (callers pre-escape / wrap as needed).
func writeTableEl(buf *bytes.Buffer, headers []string, rows [][]string) {
	buf.WriteString("<table>\n<thead>\n<tr>")
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
		writeTableEl(b, []string{"Property", "Value"}, rows)
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
		writeTableEl(b, []string{"Driver", "Description", "User-defined"}, rows)
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
		writeTableEl(b, []string{"Catalog", "Active"}, rows)
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
		writeTableEl(b, []string{"Schema", "Catalog", "Owner", "Active"}, rows)
	})
	if err != nil {
		return err
	}
	_, err = buf.WriteTo(w.out)
	return err
}
