package explore

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// metaFetcher is the minimum metadata surface the explore TUI needs.
// Implemented by a real adapter over *run.Run and by test fakes.
type metaFetcher interface {
	// FetchSourceOverview returns the source's overview (noSchema=true).
	FetchSourceOverview(ctx context.Context, handle string) (*sourceOverview, error)

	// FetchTableNames returns the list of table names. Backed by
	// MDCache.TableNames so repeat calls are free.
	FetchTableNames(ctx context.Context, handle string) ([]string, error)

	// FetchTableMeta returns full metadata for a single table. Backed
	// by MDCache.TableMeta.
	FetchTableMeta(ctx context.Context, handle, tableName string) (*metadata.Table, error)
}

// sourceOverview is the cheap-overview shape — the subset of
// metadata.Source that survives a noSchema=true fetch. We keep it as a
// distinct type so the schema-tree pane can't accidentally try to
// iterate Tables on it.
type sourceOverview struct {
	Handle    string
	Driver    string
	DBProduct string
	DBVersion string
	Location  string
	Size      int64
	Tables    int64
	Views     int64
}

// fetchSourceOverviewCmd returns a tea.Cmd that fetches the source
// overview asynchronously and dispatches a sourceOverviewLoadedMsg.
func fetchSourceOverviewCmd(ctx context.Context, f metaFetcher, handle string) tea.Cmd {
	return func() tea.Msg {
		ov, err := f.FetchSourceOverview(ctx, handle)
		m := sourceOverviewLoadedMsg{handle: handle, err: err}
		if ov != nil {
			m.meta = &metadata.Source{
				Handle:     ov.Handle,
				Location:   ov.Location,
				DBProduct:  ov.DBProduct,
				DBVersion:  ov.DBVersion,
				Size:       ov.Size,
				TableCount: ov.Tables,
				ViewCount:  ov.Views,
			}
		}
		return m
	}
}

// fetchTableNamesCmd returns a tea.Cmd that fetches table names and
// dispatches a tableNamesLoadedMsg.
func fetchTableNamesCmd(ctx context.Context, f metaFetcher, handle string) tea.Cmd {
	return func() tea.Msg {
		names, err := f.FetchTableNames(ctx, handle)
		return tableNamesLoadedMsg{handle: handle, names: names, err: err}
	}
}

// fetchTableMetaCmd returns a tea.Cmd that fetches per-table metadata
// and dispatches a tableMetaLoadedMsg.
func fetchTableMetaCmd(ctx context.Context, f metaFetcher, handle, tableName string) tea.Cmd {
	return func() tea.Msg {
		meta, err := f.FetchTableMeta(ctx, handle, tableName)
		return tableMetaLoadedMsg{handle: handle, tableName: tableName, meta: meta, err: err}
	}
}

// runFetcher adapts *run.Run to the metaFetcher interface.
//
//nolint:unused // wired up in Phase 4.3.
type runFetcher struct {
	ru *run.Run
}

//nolint:unused // wired up in Phase 4.3.
func newRunFetcher(ru *run.Run) *runFetcher { return &runFetcher{ru: ru} }

//nolint:unused // wired up in Phase 4.3.
func (rf *runFetcher) FetchSourceOverview(ctx context.Context, handle string) (*sourceOverview, error) {
	src, err := rf.ru.Config.Collection.Get(handle)
	if err != nil {
		return nil, err
	}
	grip, err := rf.ru.Grips.Open(ctx, src)
	if err != nil {
		return nil, err
	}
	md, err := grip.SourceMetadata(ctx, true /* noSchema */)
	if err != nil {
		return nil, err
	}
	return &sourceOverview{
		Handle:    md.Handle,
		Driver:    string(md.Driver),
		DBProduct: md.DBProduct,
		DBVersion: md.DBVersion,
		Location:  md.Location,
		Size:      md.Size,
		Tables:    md.TableCount,
		Views:     md.ViewCount,
	}, nil
}

//nolint:unused // wired up in Phase 4.3.
func (rf *runFetcher) FetchTableNames(ctx context.Context, handle string) ([]string, error) {
	return rf.ru.MDCache.TableNames(ctx, handle)
}

//nolint:unused // wired up in Phase 4.3.
func (rf *runFetcher) FetchTableMeta(ctx context.Context, handle, tableName string) (*metadata.Table, error) {
	return rf.ru.MDCache.TableMeta(ctx, source.Table{Handle: handle, Name: tableName})
}
