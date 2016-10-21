package drvr

import (
	"fmt"
	"strings"

	"net/url"

	"regexp"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/libsq/shutdown"
	"github.com/neilotoole/sq/libsq/util"
)

type Type string

// Source describes a data source.
type Source struct {
	Handle   string     `yaml:"handle"`
	Location string     `yaml:"location"`
	Type     Type       `yaml:"type"`
	Options  url.Values `yaml:"options,omitempty"`
}

var handlePattern = regexp.MustCompile(`\A[@][a-zA-Z][a-zA-Z0-9_]*$`)

// CheckHandleText returns an error if handle is not an acceptable value.
func CheckHandleValue(handle string) error {

	matches := handlePattern.MatchString(handle)

	if !matches {
		return util.Errorf(`invalid data source handle value %q: must begin with @, followed by a letter, followed by zero or more letters, digits, or underscores, e.g. "@my_db1"`, handle)
	}

	return nil
}

// AddSource attempts to register a new data source. driverName is optional; if not
// provided, the function attempts to guess the driver type.
func AddSource(handle string, location string, driverName string, opts url.Values) (*Source, error) {

	msg := fmt.Sprintf("attempting to create data source %q [%s] from %q", handle, driverName, location)
	if opts != nil {
		msg = fmt.Sprintf(msg+" with options: %s", opts.Encode())
	}

	lg.Debugf(msg)
	err := CheckHandleValue(handle)
	if err != nil {
		return nil, err
	}

	var driverType Type
	if driverName != "" {
		_, ok := registeredDrivers[Type(driverName)]
		if !ok {
			return nil, util.Errorf("unknown driver type %q", driverName)
		}
		driverType = Type(driverName)
	}

	if driverType == "" {
		// check if it's standard driver URL
		driverType, _ = getDriverTypeFromStdDriverURL(location)
	}

	if driverType == "" {
		// not a standard driver URL, probably a file-based source or a remote source
		file, mediatype, cleanup, err := GetSourceFile(location)

		// TODO (neilotoole): ^^ do we really want to retrieve the file at this stage, or just
		// get the mediatype etc?
		if err != nil {
			lg.Errorf("unable to determine driver for data src: %s", location)
			// at this stage, we just give up
			return nil, err
		}

		if file != nil {
			shutdown.Add(func() error {
				return file.Close()
			})
		}

		shutdown.Add(cleanup)

		ok := false
		driverType, ok = getDriverTypeFromMediaType(mediatype)
		if !ok {
			return nil, util.Errorf("unable to determine driver for data src: %s [%s]", location, mediatype)
		}

	}

	//lg.Debugf("attempting to create data source %q at %q", handle, location)
	//
	//if driverType == "" {
	//	driverType, err = GetTypeFromSrcLocation(location)
	//	if err != nil {
	//		return nil, err
	//	}
	//}

	src := &Source{Handle: handle, Location: location, Type: driverType, Options: opts}

	drv, err := For(src)
	if err != nil {
		return nil, err
	}

	lg.Debugf("will now validate provisional new datasource: %q", src)

	canonicalSource, err := drv.ValidateSource(src)
	return canonicalSource, err
}

// getDriverTypeFromMediaType returns the driver type corresponding to mediatype.
// For example:
//
//  application/vnd.openxmlformats-officedocument.spreadsheetml.sheet  -->  xlsx
//  text/csv --> csv
func getDriverTypeFromMediaType(mediatype string) (driverType Type, ok bool) {

	switch {
	case strings.Index(mediatype, `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`) != -1:
		driverType = Type("xlsx")
		ok = true
	case strings.Index(mediatype, `text/csv`) != -1:
		driverType = Type("csv")
		ok = true

	case strings.Index(mediatype, `text/tab-separated-values`) != -1:
		driverType = Type("tsv")
		ok = true
	}

	lg.Debugf("%q  :  %q", mediatype, driverType)
	return driverType, ok
}

// getDriverTypeFromStdDriverURL checks if location maps to a typical registered SQL driver,
// and if so returns the driver type and true. That is, does the location start with
// the name of the SQL driver? Examples:
//
//  mysql://root:root@tcp(localhost:33067)/sq_mydb1
//  postgres://sq:sq@localhost/sq_pg1?sslmode=disable
//
func getDriverTypeFromStdDriverURL(location string) (Type, bool) {

	u, err := url.ParseRequestURI(location)
	if err != nil {
		return "", false
	}

	drv, ok := registeredDrivers[Type(u.Scheme)]
	if !ok {
		return "", false
	}

	return drv.Type(), true
}

func GetTypeFromSrcLocation(location string) (Type, error) {

	lg.Debugf("attempting to determine datasource type from %q", location)
	// xsls content type: application/vnd.ms-excel

	// A location can look like:
	//HANDLE             DRIVER    LOCATION
	//@my1               mysql     mysql://root:root@tcp(localhost:33067)/sq_mydb1
	//@pg1               postgres  postgres://sq:sq@localhost/sq_pg1?sslmode=disable
	//@sl1               sqlite3   sqlite3:///Users/neilotoole/nd/go/src/github.com/neilotoole/sq/test/sqlite/sqlite_db1
	//@excel2            xlsx      /Users/neilotoole/nd/go/src/github.com/neilotoole/sq/test/xlsx/test.xlsx
	//@excel3            xlsx      test.xlsx
	//@excel4            xlsx      https://s3.amazonaws.com/sq.neilotoole.io/testdata/1.0/xslx/test.xlsx

	parts := strings.Split(location, "://")

	if len(parts) > 1 && parts[0] != "http" && parts[0] != "https" {
		drvName := parts[0]
		drv, ok := registeredDrivers[Type(drvName)]
		if !ok {
			return "", util.Errorf("unknown driver type %q in source location %q", drvName, location)
		}

		lg.Debugf("found datasource type %q for location %q", drv.Type(), location)
		return drv.Type(), nil
	}

	// check if it's http or https
	if parts[0] == "http" || parts[0] == "https" {

		// https://s3.amazonaws.com/sq.neilotoole.io/testdata/1.0/xslx/test.xlsx
		u, err := url.ParseRequestURI(location)
		if err != nil {
			return "", util.Errorf("unable to determine datasource type from location %q due to error: %v", location, err)
		}

		// let's see if we can determine the file extension
		// /testdata/1.0/xslx/test.xlsx
		suffix, ok := getFileSuffixFromPath(u.Path)
		if !ok {
			return "", util.Errorf("unable to determine source type from file suffix in location %q", location)
		}

		drv, ok := registeredDrivers[Type(strings.ToLower(suffix))]
		if !ok {
			return "", util.Errorf("no driver for file suffix %q in source location %q", suffix, location)
		}

		lg.Debugf("found datasource type %q for location %q", drv.Type(), location)
		return drv.Type(), nil

	}

	// it's most likely a file path, e.g.
	// /Users/neilotoole/nd/go/src/github.com/neilotoole/sq/test/xlsx/test.xlsx
	suffix, ok := getFileSuffixFromPath(location)
	if !ok {
		return "", util.Errorf("unable to determine source type from file suffix in location %q", location)
	}
	drv, ok := registeredDrivers[Type(strings.ToLower(suffix))]
	if !ok {
		return "", util.Errorf("no driver for file suffix %q in source location %q", suffix, location)
	}

	lg.Debugf("found datasource type %q for location %q", drv.Type(), location)
	return drv.Type(), nil
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

func IsValidSourceLocation(location string) bool {

	lg.Debugf("checking source location %q", location)
	parts := strings.Split(location, "://")

	if len(parts) != 2 {
		lg.Debugf("expected source location %q to have two parts, but had %d", location, len(parts))
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

	_, ok := registeredDrivers[typ]
	if !ok {
		lg.Debugf("given source location %q, no driver found for location component %q", location, typ)
	}

	return ok

}

func (s Source) String() string {

	return fmt.Sprintf("[%s] %s", s.Handle, s.Location)
}

type SourceSet struct {
	ActiveSrc string    `yaml:"active"`
	Items     []*Source `yaml:"items"`
}

func (ss *SourceSet) Add(src *Source) error {

	if i, _ := ss.IndexOf(src.Handle); i != -1 {
		return util.Errorf(`data source with name "%v" already exists`, src.Handle)
	}

	ss.Items = append(ss.Items, src)
	return nil
}

func (ss *SourceSet) IndexOf(name string) (int, *Source) {

	for i, src := range ss.Items {
		if src.Handle == name {
			return i, src
		}
	}

	return -1, nil
}

func (ss *SourceSet) Active() (*Source, bool) {

	// TODO: contemplate changing "Active" to default??
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
		if src.Handle == name {
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
	//ss.Items = []*Source{}
	return ss
}

func newUnknownSourceError(name string) error {
	return util.ErrorfN(1, `unknown data source "%v"`, name)
}
