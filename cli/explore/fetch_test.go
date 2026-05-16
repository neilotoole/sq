package explore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source/metadata"
)

// fakeFetcher implements the metaFetcher interface used by fetch.go,
// returning canned results so we can drive tea.Cmds in unit tests.
type fakeFetcher struct {
	tableNames map[string][]string
	tableErr   map[string]error
}

func (f *fakeFetcher) FetchTableNames(_ context.Context, handle string) ([]string, error) {
	if err, ok := f.tableErr[handle]; ok {
		return nil, err
	}
	return f.tableNames[handle], nil
}

func (f *fakeFetcher) FetchSourceOverview(_ context.Context, handle string) (*sourceOverview, error) {
	return &sourceOverview{Handle: handle}, nil
}

func (f *fakeFetcher) FetchTableMeta(_ context.Context, _, table string) (*metadata.Table, error) {
	return &metadata.Table{Name: table}, nil
}

func TestFetchTableNames_Cmd_DispatchesMsg(t *testing.T) {
	f := &fakeFetcher{tableNames: map[string][]string{"@x": {"a", "b"}}}
	cmd := fetchTableNamesCmd(context.Background(), f, "@x")
	require.NotNil(t, cmd)

	msg := cmd()
	loaded, ok := msg.(tableNamesLoadedMsg)
	require.True(t, ok, "expected tableNamesLoadedMsg, got %T", msg)
	require.Equal(t, "@x", loaded.handle)
	require.Equal(t, []string{"a", "b"}, loaded.names)
	require.NoError(t, loaded.err)
}

func TestFetchTableNames_Cmd_PropagatesError(t *testing.T) {
	f := &fakeFetcher{tableErr: map[string]error{"@x": context.DeadlineExceeded}}
	cmd := fetchTableNamesCmd(context.Background(), f, "@x")
	msg := cmd().(tableNamesLoadedMsg)
	require.ErrorIs(t, msg.err, context.DeadlineExceeded)
	require.Nil(t, msg.names)
}

func TestFetchTableMeta_Cmd_DispatchesMsg(t *testing.T) {
	f := &fakeFetcher{}
	cmd := fetchTableMetaCmd(context.Background(), f, "@x", "actor")
	msg := cmd().(tableMetaLoadedMsg)
	require.Equal(t, "@x", msg.handle)
	require.Equal(t, "actor", msg.tableName)
	require.NotNil(t, msg.meta)
	require.Equal(t, "actor", msg.meta.Name)
}

func TestFetchSourceOverview_Cmd_DispatchesMsg(t *testing.T) {
	f := &fakeFetcher{}
	cmd := fetchSourceOverviewCmd(context.Background(), f, "@x")
	msg := cmd().(sourceOverviewLoadedMsg)
	require.Equal(t, "@x", msg.handle)
	require.NotNil(t, msg.meta)
	require.Equal(t, "@x", msg.meta.Handle)
	require.NoError(t, msg.err)
}
