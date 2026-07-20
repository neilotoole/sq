package explore

import (
	"context"
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
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

	// RefreshSource forces a fresh fetch, bypassing any cache.
	RefreshSource(ctx context.Context, handle string) ([]string, error)
}

// sourceOverview is the cheap-overview shape — the subset of
// metadata.Source that survives a noSchema=true fetch. We keep it as a
// distinct type so the schema-tree pane can't accidentally try to
// iterate Tables on it.
type sourceOverview struct {
	Size      *int64
	Handle    string
	Driver    string
	DBProduct string
	DBVersion string
	Location  string
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
				Driver:     drivertype.Type(ov.Driver),
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

// refreshSourceCmd returns a tea.Cmd that forces a fresh metadata fetch
// (bypassing the mdcache) and dispatches a tableNamesLoadedMsg.
func refreshSourceCmd(ctx context.Context, f metaFetcher, handle string) tea.Cmd {
	return func() tea.Msg {
		names, err := f.RefreshSource(ctx, handle)
		return tableNamesLoadedMsg{handle: handle, names: names, err: err}
	}
}

// runFetcher adapts *run.Run to the metaFetcher interface.
type runFetcher struct {
	ru *run.Run
}

func newRunFetcher(ru *run.Run) *runFetcher { return &runFetcher{ru: ru} }

func (rf *runFetcher) FetchSourceOverview(ctx context.Context, handle string) (*sourceOverview, error) {
	src, err := rf.ru.Config.Collection.Get(handle)
	if err != nil {
		return nil, err
	}
	grip, err := rf.ru.Grips.Open(ctx, src, driver.ModeReadOnly)
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

func (rf *runFetcher) FetchTableNames(ctx context.Context, handle string) ([]string, error) {
	return rf.ru.MDCache.TableNames(ctx, handle)
}

func (rf *runFetcher) FetchTableMeta(ctx context.Context, handle, tableName string) (*metadata.Table, error) {
	return rf.ru.MDCache.TableMeta(ctx, source.Table{Handle: handle, Name: tableName})
}

// RefreshSource forces a fresh fetch of source metadata, bypassing the
// mdcache, and returns the table names from the refreshed metadata.
func (rf *runFetcher) RefreshSource(ctx context.Context, handle string) ([]string, error) {
	src, err := rf.ru.Config.Collection.Get(handle)
	if err != nil {
		return nil, err
	}
	grip, err := rf.ru.Grips.Open(ctx, src, driver.ModeReadOnly)
	if err != nil {
		return nil, err
	}
	// noSchema must be false: SourceMetadata returns early (before
	// populating md.Tables) when noSchema is true, so a true here would
	// always yield an empty table list and blank the schema pane on R.
	md, err := grip.SourceMetadata(ctx, false)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(md.Tables))
	for _, t := range md.Tables {
		names = append(names, t.Name)
	}
	return names, nil
}

// previewFunc is the signature used to launch a preview-row stream.
// Kept as a function value so tests can substitute a stub.
type previewFunc func(ctx context.Context, send func(any), handle, table string, n int)

// runPreview is the production implementation of previewFunc. It
// constructs a previewWriter (which streams up to n records via send)
// and executes the SLQ query in a goroutine. Errors are reported back
// to the caller via send(previewErrMsg{...}).
func (rf *runFetcher) runPreview(ctx context.Context, send func(any), handle, table string, n int) {
	// The writer stops the pipeline once it has enough rows by cancelling
	// this context with cause errz.ErrStop, rather than invoking the
	// cancelFn that ExecSLQ passes to Open — the RecordWriter contract
	// reserves that cancelFn for Wait.
	ctx, stop := context.WithCancelCause(ctx)
	pw := newPreviewWriter(handle, table, n, send, func() { stop(errz.ErrStop) })
	qc := &libsq.QueryContext{
		Collection: rf.ru.Config.Collection,
		Grips:      rf.ru.Grips,
	}
	// SLQ row-limit: `@handle.table | .[0:N]`.
	query := fmt.Sprintf("%s.%s | .[0:%d]", handle, table, n)
	go func() {
		err := libsq.ExecSLQ(ctx, qc, query, pw)
		// A capped preview stops the pipeline (cause errz.ErrStop), and
		// program shutdown cancels the parent ctx (context.Canceled).
		// Neither is a real error, so don't surface them to the user.
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, errz.ErrStop) {
			lg.FromContext(ctx).Error("explore: preview query failed",
				lga.Src, handle, lga.Table, table, lga.Err, err)
			send(previewErrMsg{handle: handle, tableName: table, err: err})
		}
		// Honor the RecordWriter contract: Wait invokes the Open cancelFn.
		_, _ = pw.Wait()
		stop(nil)
	}()
}
