package json

import (
	"context"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/source"
)

type importFunc func(ctx context.Context, log lg.Log, src *source.Source, openFn source.FileOpenFunc, scratchDB driver.Database) error

var (
	_ importFunc = importJSON
	_ importFunc = importJSONA
	_ importFunc = importJSONL
)

func importJSON(ctx context.Context, log lg.Log, src *source.Source, openFn source.FileOpenFunc, scratchDB driver.Database) error {
	return errz.New("not implemented")
}

func importJSONA(ctx context.Context, log lg.Log, src *source.Source, openFn source.FileOpenFunc, scratchDB driver.Database) error {
	return errz.New("not implemented")
}

func importJSONL(ctx context.Context, log lg.Log, src *source.Source, openFn source.FileOpenFunc, scratchDB driver.Database) error {
	return errz.New("not implemented")

	//const optPredictKind bool = true
	//
	//var err error
	//var r io.ReadCloser
	//
	//r, err = openFn()
	//if err != nil {
	//	return err
	//}
	//
	//// We add the CR filter reader to deal with CSV files exported
	//// from Excel which can have the DOS-style \r EOL markers.
	//cr := csv.NewReader(&crFilterReader{r: r})
	//cr.Comma, err = getDelimiter(src)
	//if err != nil {
	//	return err
	//}
	//
	//// readAheadRecs temporarily holds records read from r for the purpose
	//// of determining CSV metadata such as column headers, data kind etc.
	//// These records will later be written to recordCh.
	//readAheadRecs := make([][]string, 0, readAheadBufferSize)
	//
	//colNames, err := getColNames(cr, src, &readAheadRecs)
	//if err != nil {
	//	return err
	//}
	//
	//var expectFieldCount = len(colNames)
	//
	//var colKinds []sqlz.Kind
	//if optPredictKind {
	//	colKinds, err = predictColKinds(expectFieldCount, cr, &readAheadRecs, readAheadBufferSize)
	//	if err != nil {
	//		return err
	//	}
	//} else {
	//	// If we're not predicting col kind, then we use KindText.
	//	colKinds = make([]sqlz.Kind, expectFieldCount)
	//	for i := range colKinds {
	//		colKinds[i] = sqlz.KindText
	//	}
	//}
	//
	//// And now we need to create the dest table in scratchDB
	//tblDef := createTblDef(source.MonotableName, colNames, colKinds)
	//
	//err = scratchDB.SQLDriver().CreateTable(ctx, scratchDB.DB(), tblDef)
	//if err != nil {
	//	return errz.Wrap(err, "csv: failed to create dest scratch table")
	//}
	//
	//recMeta, err := getRecMeta(ctx, scratchDB, tblDef)
	//if err != nil {
	//	return err
	//}
	//
	//insertWriter := libsq.NewDBWriter(log, scratchDB, tblDef.Name, insertChSize)
	//err = execInsert(ctx, insertWriter, recMeta, readAheadRecs, cr)
	//if err != nil {
	//	return err
	//}
	//
	//inserted, err := insertWriter.Wait()
	//if err != nil {
	//	return err
	//}
	//
	//log.Debugf("Inserted %d rows to %s.%s", inserted, scratchDB.Source().Handle, tblDef.Name)
	//return nil
}
