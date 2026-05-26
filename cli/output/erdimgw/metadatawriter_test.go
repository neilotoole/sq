package erdimgw_test

import (
	"bytes"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/erdimgw"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// newTestSource builds a small deterministic two-table source
// (actor + film_actor, with film_actor.actor_id → actor.actor_id) and links
// its foreign keys so FK.Incoming is populated.
func newTestSource() *metadata.Source {
	actor := &metadata.Table{
		Name: "actor", TableType: "table", RowCount: 200,
		Columns: []*metadata.Column{
			{Name: "actor_id", Position: 1, PrimaryKey: true, ColumnType: "INTEGER", Kind: kind.Int},
			{Name: "first_name", Position: 2, ColumnType: "TEXT", Kind: kind.Text},
		},
	}
	filmActor := &metadata.Table{
		Name: "film_actor", TableType: "table", RowCount: 5462,
		Columns: []*metadata.Column{
			{Name: "actor_id", Position: 1, PrimaryKey: true, ColumnType: "INTEGER", Kind: kind.Int},
			{Name: "film_id", Position: 2, PrimaryKey: true, ColumnType: "INTEGER", Kind: kind.Int},
		},
		FK: &metadata.FKGroup{Outgoing: []*metadata.ForeignKey{{
			Name: "fk_film_actor_actor", Table: "film_actor", Columns: []string{"actor_id"},
			RefTable: "actor", RefColumns: []string{"actor_id"},
		}}},
	}
	src := &metadata.Source{
		Handle: "@test", Name: "testdb", Driver: drivertype.Type("sqlite3"),
		Schema: "main", Tables: []*metadata.Table{actor, filmActor},
	}
	metadata.LinkForeignKeys(nil, src)
	return src
}

// TestSVG_SourceMetadata renders the whole-source ERD to SVG and checks it's
// valid SVG markup carrying the table names.
func TestSVG_SourceMetadata(t *testing.T) {
	buf := &bytes.Buffer{}
	w := erdimgw.NewSVGMetadataWriter(buf, output.NewPrinting())
	require.NoError(t, w.SourceMetadata(newTestSource(), true))

	out := buf.String()
	require.Contains(t, out, "<svg", "output should be SVG markup")
	require.Contains(t, out, "actor")
	require.Contains(t, out, "film_actor")
}

// TestSVG_TableMetadata renders a focused single-table ERD to SVG.
func TestSVG_TableMetadata(t *testing.T) {
	src := newTestSource()
	filmActor := src.Table("film_actor")
	require.NotNil(t, filmActor)

	buf := &bytes.Buffer{}
	w := erdimgw.NewSVGMetadataWriter(buf, output.NewPrinting())
	require.NoError(t, w.TableMetadata(filmActor))

	out := buf.String()
	require.Contains(t, out, "<svg")
	require.Contains(t, out, "film_actor")
}

// TestPNG_SourceMetadata renders the whole-source ERD to PNG and checks the
// bytes decode as a non-empty PNG image. PNG raster output isn't byte-stable
// across platforms/library versions, so we validate by decoding rather than
// by golden comparison.
func TestPNG_SourceMetadata(t *testing.T) {
	buf := &bytes.Buffer{}
	w := erdimgw.NewPNGMetadataWriter(buf, output.NewPrinting())
	require.NoError(t, w.SourceMetadata(newTestSource(), true))

	require.True(t, strings.HasPrefix(buf.String(), "\x89PNG\r\n\x1a\n"),
		"output should start with the PNG magic bytes")

	img, err := png.Decode(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	require.Positive(t, img.Bounds().Dx())
	require.Positive(t, img.Bounds().Dy())
}

// TestUnsupported verifies that operations with no ERD representation return
// an error and write nothing, rather than emitting an empty (invalid) image.
func TestUnsupported(t *testing.T) {
	for _, newWriter := range map[string]func(*bytes.Buffer) output.MetadataWriter{
		"png": func(b *bytes.Buffer) output.MetadataWriter {
			return erdimgw.NewPNGMetadataWriter(b, output.NewPrinting())
		},
		"svg": func(b *bytes.Buffer) output.MetadataWriter {
			return erdimgw.NewSVGMetadataWriter(b, output.NewPrinting())
		},
	} {
		buf := &bytes.Buffer{}
		w := newWriter(buf)

		// Overview mode (showSchema=false) carries no schema to diagram.
		require.Error(t, w.SourceMetadata(newTestSource(), false))
		require.Error(t, w.DBProperties(map[string]any{"k": "v"}))
		require.Error(t, w.DriverMetadata(nil))
		require.Error(t, w.Catalogs("sakila", []string{"sakila"}))
		require.Error(t, w.Schemata("public", []*metadata.Schema{{Name: "public"}}))
		require.Empty(t, buf.String())
	}
}

// TestNothingToRender verifies that a source/table with nothing to diagram
// returns an error and writes nothing, rather than producing an empty image.
func TestNothingToRender(t *testing.T) {
	buf := &bytes.Buffer{}
	w := erdimgw.NewSVGMetadataWriter(buf, output.NewPrinting())

	src := &metadata.Source{
		Handle: "@empty", Name: "empty", Driver: drivertype.Type("sqlite3"),
		Tables: []*metadata.Table{{Name: "t", TableType: "table"}},
	}
	require.Error(t, w.SourceMetadata(src, true))
	require.Empty(t, buf.String())

	buf.Reset()
	require.Error(t, w.TableMetadata(&metadata.Table{Name: "t", TableType: "table"}))
	require.Empty(t, buf.String())
}
