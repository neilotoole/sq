package driver

import (
	"fmt"
	"strings"

	"net/url"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/lib/util"
)

type Type string

//const DriverMySQL Type = "mysql"
//const DriverPostgres Type = "postgres"
//const DriverSQLite Type = "sqlite3"
//const DriverExcel Type = "excel"
//
//var Drivers = []Type{DriverMySQL, DriverPostgres, DriverSQLite, DriverExcel}

// Source describes a data source.
type Source struct {
	Ref      string `yaml:"ref"`
	Location string `yaml:"location"`
	Type     Type   `yaml:"type"`
}

func NewSource(name string, ref string) (*Source, error) {

	lg.Debugf("attempting to create datasource %q using ref %q", name, ref)

	typ, err := GetTypeFromSourceRef(ref)
	if err != nil {
		return nil, err
	}

	src := &Source{Ref: name, Location: ref, Type: typ}

	drvr, err := For(src)
	if err != nil {
		return nil, err
	}

	lg.Debugf("will now validate provisional new datasource: %q", src)

	canonicalSource, err := drvr.ValidateSource(src)
	return canonicalSource, err
}

func GetTypeFromSourceRef(ref string) (Type, error) {

	lg.Debugf("attempting to determine datasource type from ref %q", ref)
	// xsls content type: application/vnd.ms-excel

	// A ref can look like:
	//NAME              DRIVER    REF
	//my1               mysql     mysql://root:root@tcp(localhost:33067)/sq_mydb1
	//pg1               postgres  postgres://sq:sq@localhost/sq_pg1?sslmode=disable
	//sl1               sqlite3   sqlite3:///Users/neilotoole/nd/go/src/github.com/neilotoole/sq/test/sqlite/sqlite_db1
	//excel1            xlsx      xlsx:///Users/neilotoole/nd/go/src/github.com/neilotoole/sq/test/xlsx/test.xlsx
	//
	//excel2            xlsx      /Users/neilotoole/nd/go/src/github.com/neilotoole/sq/test/xlsx/test.xlsx
	//excel3            xlsx      test.xlsx
	//excel4            xlsx      https://s3.amazonaws.com/sq.neilotoole.io/testdata/1.0/xslx/test.xlsx

	parts := strings.Split(ref, "://")

	if len(parts) > 1 && parts[0] != "http" && parts[0] != "https" {
		drvrName := parts[0]
		drvr, ok := drvrs[Type(drvrName)]
		if !ok {
			return "", util.Errorf("unknown driver type %q in source ref %q", drvrName, ref)
		}

		lg.Debugf("found datasource type %q for ref %q", drvr.Type(), ref)
		return drvr.Type(), nil
	}

	// check if it's http or https
	if parts[0] == "http" || parts[0] == "https" {

		// https://s3.amazonaws.com/sq.neilotoole.io/testdata/1.0/xslx/test.xlsx
		u, err := url.ParseRequestURI(ref)
		if err != nil {
			return "", util.Errorf("unable to determine datasource type from ref %q due to error: %v", ref, err)
		}

		// let's see if we can determine the file extension
		// /testdata/1.0/xslx/test.xlsx
		suffix, ok := getFileSuffixFromPath(u.Path)
		if !ok {
			return "", util.Errorf("unable to determine source type from file suffix in ref %q", ref)
		}

		drvr, ok := drvrs[Type(strings.ToLower(suffix))]
		if !ok {
			return "", util.Errorf("no driver for file suffix %q in source ref %q", suffix, ref)
		}

		lg.Debugf("found datasource type %q for ref %q", drvr.Type(), ref)
		return drvr.Type(), nil

	}

	// it's most likely a file path, e.g.
	// /Users/neilotoole/nd/go/src/github.com/neilotoole/sq/test/xlsx/test.xlsx
	suffix, ok := getFileSuffixFromPath(ref)
	if !ok {
		return "", util.Errorf("unable to determine source type from file suffix in ref %q", ref)
	}
	drvr, ok := drvrs[Type(strings.ToLower(suffix))]
	if !ok {
		return "", util.Errorf("no driver for file suffix %q in source ref %q", suffix, ref)
	}

	lg.Debugf("found datasource type %q for ref %q", drvr.Type(), ref)
	return drvr.Type(), nil
}

func getFileSuffixFromPath(path string) (suffix string, ok bool) {

	pathComponents := strings.Split(path, "/")
	if len(pathComponents) == 0 {
		//return "", false
		return
	}

	// splitting a value such as "test.xlsx"
	fileComponents := strings.Split(pathComponents[len(pathComponents)-1], ".")
	if len(fileComponents) <= 1 {
		//return "", false
		return
	}

	suffix = fileComponents[len(fileComponents)-1]
	ok = true
	return
}

// DriverURI returns the value required by the specific driver to access the
// datasource. For example, for a DB driver, this would be the connection string.
func (s Source) ConnURI() string {

	if s.Type == Type("mysql") {
		parts := strings.Split(s.Location, "://")
		uri := strings.Join(parts[1:], "")
		return uri
	}

	if s.Type == Type("sqlite3") {
		return s.Location[9:]
	}

	return s.Location

}

func IsValidSourceRef(ref string) bool {

	lg.Debugf("checking source ref %q", ref)
	parts := strings.Split(ref, "://")

	if len(parts) != 2 {
		lg.Debugf("expected source ref %q to have two parts, but had %d", ref, len(parts))
		return false
	}

	//lg.Debugf("searching for driver for %q", Type(parts[0]))
	//for _, driver := range Drivers {
	//
	//	if driver == Type(parts[0]) {
	//		lg.Debugf("found driver")
	//		return true
	//	}
	//
	//}

	typ := Type(parts[0])

	_, ok := drvrs[typ]
	if !ok {
		lg.Debugf("given source ref %q, no driver found for ref component %q", ref, typ)
	}

	return ok

}

func (s Source) String() string {

	return fmt.Sprintf("[%s] %s", s.Ref, s.Location)
}

type SourceSet struct {
	ActiveSrc string    `yaml:"active"`
	Items     []*Source `yaml:"items"`
}

func (ss *SourceSet) Add(src *Source) error {

	if i, _ := ss.IndexOf(src.Ref); i != -1 {
		return util.Errorf(`data source with name "%v" already exists`, src.Ref)
	}

	ss.Items = append(ss.Items, src)
	return nil
}

func (ss *SourceSet) IndexOf(name string) (int, *Source) {

	for i, src := range ss.Items {
		if src.Ref == name {
			return i, src
		}
	}

	return -1, nil
}

func (ss *SourceSet) Active() (*Source, bool) {

	if ss.ActiveSrc == "" {
		return nil, false
	}

	i, src := ss.IndexOf(ss.ActiveSrc)
	if i == -1 {
		return nil, false
	}

	return src, true
}

func (ss *SourceSet) Get(name string) (*Source, error) {

	lg.Debugf("attempting to get datasource %q", name)
	if !strings.HasPrefix(name, "@") {
		name = "@" + name
		//lg.Debugf("stripping leading @ for canonical name %q", name)
	}

	if name == "" {
		return nil, newUnknownSourceError(name)
	}

	i, src := ss.IndexOf(name)
	if i == -1 {
		return nil, newUnknownSourceError(name)
	}

	lg.Debugf("returning datasource %q", src.String())
	return src, nil
}

func (ss *SourceSet) SetActive(name string) (*Source, error) {

	for _, src := range ss.Items {
		if src.Ref == name {
			ss.ActiveSrc = name
			return src, nil
		}
	}

	return nil, newUnknownSourceError(name)
}

func (ss *SourceSet) Remove(name string) error {

	if len(ss.Items) == 0 {
		return newUnknownSourceError(name)
	}

	i, _ := ss.IndexOf(name)
	if i == -1 {
		return newUnknownSourceError(name)
	}

	if ss.ActiveSrc == name {
		ss.ActiveSrc = ""
	}

	if len(ss.Items) == 1 {
		ss.Items = ss.Items[0:0]
		return nil
	}
	//
	//if i == 0 {
	//
	//}

	if ss.ActiveSrc == name {
		ss.ActiveSrc = ""
	}

	pre := ss.Items[:i]
	post := ss.Items[i+1:]

	ss.Items = append(pre, post...)
	return nil

}

func NewSourceSet() *SourceSet {

	ss := &SourceSet{}
	ss.Items = []*Source{}
	return ss
}

func newUnknownSourceError(name string) error {
	return util.ErrorfN(1, `unknown data source "%v"`, name)
}
