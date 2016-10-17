package scratch

import (
	"time"

	"os"
	"strconv"

	"path/filepath"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/neilotoole/sq/libsq/util"
)

//// Open returns a handle to a new scratch database.
//func Open() (*sql.DB, error) {
//
//	_, filepath := generateFilename()
//	lg.Debugf("attempting to create sqlite3 DB: %s", filepath)
//	db, err := sql.Open("sqlite3", filepath)
//	if err != nil {
//		return nil, util.WrapError(err)
//	}
//
//	lg.Debugf("successfully created sqlite3 DB: %s", filepath)
//	return db, nil
//	//return nil, util.Errorf("not implemented")
//}

var workDir string

func Init(scratchDir string) {
	workDir = scratchDir
}

// OpenNew creates a new scratch database, and returns the data source, opened
// database, cleanup function (may be nil), or an error. It is up to the caller invoke
// db.Close() and to invoke the cleanup function.
func OpenNew() (*drvr.Source, *sql.DB, func() error, error) {

	src, filepath, err := newScratchSrc()
	if err != nil {
		return nil, nil, nil, err
	}

	db, err := sql.Open(string(src.Type), src.ConnURI())
	if err != nil {
		return nil, nil, nil, util.WrapError(err)
	}

	cleanup := func() error {
		lg.Debugf("attempting to delete scratch DB file %q", filepath)
		err := os.Remove(filepath)
		if err != nil {
			lg.Errorf("error deleting scratch DB file %q: %v", filepath, err)
		} else {
			lg.Debugf("deleted scratch DB file %q", filepath)
		}
		return err
	}

	//shutdown.Add(cleanup)

	return src, db, cleanup, nil
}

func Type() drvr.Type {
	return drvr.Type("sqlite3")
}

func generateFilename(dir string) (filename string, path string) {
	const tsFmt = `20060102.030405.000000000`

	ts := time.Now().Format(tsFmt)

	filename = ts + "__" + strconv.Itoa(os.Getpid()) + ".sqlite"

	path = filepath.Join(dir, filename)

	lg.Debugf("generated path: %q", path)
	return

}

func newScratchSrc() (*drvr.Source, string, error) {

	var dir string

	if workDir == "" {
		dir = os.TempDir()
	} else {
		dir = workDir
	}

	filename, path := generateFilename(dir)
	lg.Debugf("creating scratch datasource (sqlite3): %s", filename)

	src := &drvr.Source{}
	src.Type = drvr.Type("sqlite3")
	src.Handle = "scratch_" + filename
	src.Location = "sqlite3://" + path
	return src, path, nil
}
