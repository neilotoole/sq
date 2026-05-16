package explore

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source/metadata"
)

func TestSchemaTree_Empty(t *testing.T) {
	tr := newSchemaTree("@s", newTheme(true))
	require.Equal(t, "@s", tr.handle)
	require.Equal(t, 1, tr.visibleCount(), "an unloaded tree shows a single 'loading' row")
}

func TestSchemaTree_SetTableNames(t *testing.T) {
	tr := newSchemaTree("@s", newTheme(true))
	tr.setTableNames([]string{"a", "b", "c"})
	// Default: top "tables (3)" node is collapsed.
	require.Equal(t, 1, tr.visibleCount())

	tr.toggleExpand(0)
	// Now: top + 3 table rows = 4.
	require.Equal(t, 4, tr.visibleCount())
}

func TestSchemaTree_SetTableMeta(t *testing.T) {
	tr := newSchemaTree("@s", newTheme(true))
	tr.setTableNames([]string{"film"})
	tr.toggleExpand(0) // expand "tables"

	// Selected row is now the first table.
	tr.move(1) // down to "film"
	require.Equal(t, "film", tr.selectedTableName())

	// Set the table's metadata (simulating tableMetaLoadedMsg arrival).
	tr.setTableMeta("film", &metadata.Table{
		Name:    "film",
		Columns: []*metadata.Column{{Name: "id"}, {Name: "title"}},
	})

	tr.toggleExpand(tr.selected) // expand "film"
	// "tables", "film", "columns (2)" — 3 visible.
	require.Equal(t, 3, tr.visibleCount())

	tr.move(1)                   // selected = columns
	tr.toggleExpand(tr.selected) // expand "columns"
	// "tables", "film", "columns (2)", "id", "title" — 5.
	require.Equal(t, 5, tr.visibleCount())
}
