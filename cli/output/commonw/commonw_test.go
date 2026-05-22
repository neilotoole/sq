package commonw_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output/commonw"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

func TestColumnKey(t *testing.T) {
	tbl := &metadata.Table{
		Name: "t",
		Columns: []*metadata.Column{
			{Name: "id", PrimaryKey: true},     // PK only
			{Name: "ref"},                      // FK only
			{Name: "uq"},                       // UK only
			{Name: "id_ref", PrimaryKey: true}, // PK and FK
			{Name: "plain"},                    // no key
		},
		FK: &metadata.FKGroup{Outgoing: []*metadata.ForeignKey{
			{Columns: []string{"ref"}},
			{Columns: []string{"id_ref"}},
		}},
		UniqueConstraints: []*metadata.UniqueConstraint{{Columns: []string{"uq"}}},
	}
	fk := commonw.FKColumnSet(tbl)
	uc := commonw.UCColumnSet(tbl)

	require.Equal(t, "PK", commonw.ColumnKey(tbl.Columns[0], fk, uc))
	require.Equal(t, "FK", commonw.ColumnKey(tbl.Columns[1], fk, uc))
	require.Equal(t, "UK", commonw.ColumnKey(tbl.Columns[2], fk, uc))
	require.Equal(t, "PK,FK", commonw.ColumnKey(tbl.Columns[3], fk, uc))
	require.Equal(t, "", commonw.ColumnKey(tbl.Columns[4], fk, uc))
}
