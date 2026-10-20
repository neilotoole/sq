package explore

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

func TestDetailPane_SourceView(t *testing.T) {
	d := newDetailPane(newTheme(true))
	size := int64(1024)
	d.setSource(&metadata.Source{
		Handle:     "@x",
		Driver:     "postgres",
		DBProduct:  "PostgreSQL 16",
		Location:   "postgres://localhost/x",
		TableCount: 12,
		ViewCount:  3,
		Size:       &size,
	})
	out := d.view(60, 20)
	for _, want := range []string{"@x", "PostgreSQL 16", "tables: 12", "views: 3"} {
		require.True(t, strings.Contains(out, want), "want %q in output, got: %s", want, out)
	}
}

func TestDetailPane_TableView(t *testing.T) {
	d := newDetailPane(newTheme(true))
	d.setTable(&metadata.Table{
		Name:      "film",
		RowCount:  1000,
		TableType: "table",
		Columns: []*metadata.Column{
			{Name: "id", BaseType: "int", PrimaryKey: true},
			{Name: "title", BaseType: "text"},
		},
	})
	out := d.view(60, 20)
	for _, want := range []string{"film", "rows: 1000", "id", "title", "PK"} {
		require.True(t, strings.Contains(out, want), "want %q in output, got: %s", want, out)
	}
}

func TestDetailPane_TableView_EmptyIndexesHidden(t *testing.T) {
	d := newDetailPane(newTheme(true))
	d.setTable(&metadata.Table{
		Name:    "csvrows",
		Columns: []*metadata.Column{{Name: "a"}, {Name: "b"}},
		// No indexes, no FK, no UniqueConstraints — typical for a CSV.
	})
	out := d.view(60, 20)
	require.NotContains(t, out, "indexes", "empty indexes section must not render")
	require.NotContains(t, out, "fk (", "empty fk section must not render")
}

func TestDetailPane_LoadingFallback(t *testing.T) {
	d := newDetailPane(newTheme(true))
	out := d.view(60, 20)
	require.Contains(t, out, "(loading)")
}

func TestFormatRecord_TruncatesByRune(t *testing.T) {
	// A value longer than 30 runes, all multi-byte: truncation must
	// produce valid UTF-8 (27 runes + ellipsis), never a split rune.
	long := strings.Repeat("é", 40)
	out := formatRecord(record.Record{long})
	require.True(t, utf8.ValidString(out), "truncated output must be valid UTF-8")
	require.True(t, strings.HasSuffix(out, "…"))
	require.Equal(t, 28, utf8.RuneCountInString(out), "27 kept runes + ellipsis")

	// Short values are passed through unchanged.
	require.Equal(t, "héllo", formatRecord(record.Record{"héllo"}))
}
