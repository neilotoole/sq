package scratch

import (
	"time"

	"os"
	"strconv"

	"path/filepath"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	"github.com/neilotoole/sq/lib/driver"
	"github.com/neilotoole/sq/lib/shutdown"
	"github.com/neilotoole/sq/lib/util"
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

func OpenNew() (*driver.Source, *sql.DB, error) {

	src, filepath, err := newScratchSrc()
	if err != nil {
		return nil, nil, err
	}

	db, err := sql.Open(string(src.Type), src.ConnURI())
	if err != nil {
		return nil, nil, util.WrapError(err)
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

	shutdown.Add(cleanup)

	return src, db, nil
}

func Type() driver.Type {
	return driver.Type("sqlite3")
}

func generateFilename(dir string) (filename string, path string) {
	// // apacheFormat is the standard apache timestamp format.
	//const apacheFormat = `02/Jan/2006:15:04:05 -0700`
	const tsFmt = `20060102.030405.000000000`

	ts := time.Now().Format(tsFmt)

	filename = ts + "__" + strconv.Itoa(os.Getpid()) + ".db"

	path = filepath.Join(dir, filename)

	lg.Debugf("generated path: %q", path)
	return

}

func newScratchSrc() (*driver.Source, string, error) {

	var dir string

	if workDir == "" {
		dir = os.TempDir()
	} else {
		dir = workDir
	}

	filename, path := generateFilename(dir)
	lg.Debugf("creating scratch datasource (sqlite3): %s", filename)

	src := &driver.Source{}
	src.Type = driver.Type("sqlite3")
	src.Handle = "scratch_" + filename
	src.Location = "sqlite3://" + path
	return src, path, nil
}
