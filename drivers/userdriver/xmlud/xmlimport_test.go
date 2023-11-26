package xmlud_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/drivers/userdriver/xmlud"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/testsrc"
)

const (
	driverRSS = "rss"
	driverPpl = "ppl"
)

func TestImport_Ppl(t *testing.T) {
	th := testh.New(t)

	ext := &config.Ext{}
	require.NoError(t, ioz.UnmarshallYAML(proj.ReadFile(testsrc.PathDriverDefPpl), ext))
	require.Equal(t, 1, len(ext.UserDrivers))
	udDef := ext.UserDrivers[0]
	require.Equal(t, driverPpl, udDef.Name)
	require.Equal(t, xmlud.Genre, udDef.Genre)

	src := &source.Source{Handle: "@ppl_" + stringz.Uniq8(), Type: drivertype.None}
	scratchDB, err := th.Pools().OpenScratchFor(th.Context, src)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, scratchDB.Close())
	})

	data := proj.ReadFile("drivers/userdriver/xmlud/testdata/people.xml")
	err = xmlud.Import(th.Context, udDef, bytes.NewReader(data), scratchDB)
	require.NoError(t, err)

	srcMeta, err := scratchDB.SourceMetadata(th.Context, false)
	require.NoError(t, err)
	require.Equal(t, 2, len(srcMeta.Tables))
	require.Equal(t, "person", srcMeta.Tables[0].Name)
	require.Equal(t, "skill", srcMeta.Tables[1].Name)

	sink, err := th.QuerySQL(scratchDB.Source(), nil, "SELECT * FROM person")
	require.NoError(t, err)
	require.Equal(t, 3, len(sink.Recs))
	require.Equal(t, "Nikola", stringz.Val(sink.Recs[0][1]))
	for i, rec := range sink.Recs {
		// Verify that the primary id cols are sequential
		require.Equal(t, int64(i+1), stringz.Val(rec[0]))
	}

	sink, err = th.QuerySQL(scratchDB.Source(), nil, "SELECT * FROM skill")
	require.NoError(t, err)
	require.Equal(t, 6, len(sink.Recs))
	require.Equal(t, "Electrifying", stringz.Val(sink.Recs[0][2]))
	for i, rec := range sink.Recs {
		// Verify that the primary id cols are sequential
		require.Equal(t, int64(i+1), stringz.Val(rec[0]))
	}
}

func TestImport_RSS(t *testing.T) {
	th := testh.New(t)

	ext := &config.Ext{}
	require.NoError(t, ioz.UnmarshallYAML(proj.ReadFile(testsrc.PathDriverDefRSS), ext))
	require.Equal(t, 1, len(ext.UserDrivers))
	udDef := ext.UserDrivers[0]
	require.Equal(t, driverRSS, udDef.Name)
	require.Equal(t, xmlud.Genre, udDef.Genre)

	src := &source.Source{Handle: "@rss_" + stringz.Uniq8(), Type: drivertype.None}
	scratchDB, err := th.Pools().OpenScratchFor(th.Context, src)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, scratchDB.Close())
	})

	data := proj.ReadFile("drivers/userdriver/xmlud/testdata/nytimes_local.rss.xml")
	err = xmlud.Import(th.Context, udDef, bytes.NewReader(data), scratchDB)
	require.NoError(t, err)

	srcMeta, err := scratchDB.SourceMetadata(th.Context, false)
	require.NoError(t, err)
	require.Equal(t, 3, len(srcMeta.Tables))
	require.Equal(t, "category", srcMeta.Tables[0].Name)
	require.Equal(t, "channel", srcMeta.Tables[1].Name)
	require.Equal(t, "item", srcMeta.Tables[2].Name)

	sink, err := th.QuerySQL(scratchDB.Source(), nil, "SELECT * FROM channel")
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))
	require.Equal(t, "NYT > World", stringz.Val(sink.Recs[0][1]))
	for i, rec := range sink.Recs {
		// Verify that the primary id cols are sequential
		require.Equal(t, int64(i+1), stringz.Val(rec[0]))
	}

	sink, err = th.QuerySQL(scratchDB.Source(), nil, "SELECT * FROM category")
	require.NoError(t, err)
	require.Equal(t, 251, len(sink.Recs))
	require.EqualValues(t, "Extradition", stringz.Val(sink.Recs[0][2]))
	for i, rec := range sink.Recs {
		// Verify that the primary id cols are sequential
		require.Equal(t, int64(i+1), stringz.Val(rec[0]))
	}

	sink, err = th.QuerySQL(scratchDB.Source(), nil, "SELECT * FROM item")
	require.NoError(t, err)
	require.Equal(t, 45, len(sink.Recs))
	require.EqualValues(t, "Trilobites: Fishing for Clues to Solve Namibia’s Fairy Circle Mystery",
		stringz.Val(sink.Recs[17][4]))
	for i, rec := range sink.Recs {
		// Verify that the primary id cols are sequential
		require.Equal(t, int64(i+1), stringz.Val(rec[0]))
	}
}
