package cli

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/core/ioz"

	"github.com/neilotoole/sq/libsq/core/errz"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/driver"
	"golang.org/x/exp/slices"

	"github.com/neilotoole/sq/drivers/mysql"
	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/drivers/sqlserver"

	"github.com/xo/dburl"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/neilotoole/sq/libsq/source"

	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

// locCompStdDirective is the standard cobra shell completion directive
// returned by completeAddLocation.
const locCompStdDirective = cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveKeepOrder

// completeAddLocation provides completion for the "sq add LOCATION" arg.
// This is a messy task, as LOCATION can be a database driver URL,
// and it can also be a filepath. To complicate matters further, sqlite
// has a format sqlite://FILE/PATH?param=val, which is a driver URL, with
// embedded file completion.
//
// The general strategy is:
//   - Does toComplete have a driver prefix ("postgres://", "sqlite3://" etc)?
//     If so, delegate to the appropriate function.
//   - Is toComplete definitively NOT a driver URL? That is to say, is toComplete
//     a file path? If so, then we need regular shell file completion.
//     Return cobra.ShellCompDirectiveDefault, and let the shell handle it.
//   - There's a messy overlap where toComplete could be either a driver URL
//     or a filepath. For example, "post" could be leading to "postgres://", or
//     to a file named "post.db". For this situation, it is necessary to
//     mimic in code the behavior of the shell's file completion.
//
// The code, as currently structured, is ungainly, and downright ugly in
// spots, and probably won't scale well if more drivers are supported. That
// is to say, this mechanism would benefit from a through refactor.
func completeAddLocation(cmd *cobra.Command, args []string, toComplete string) (
	[]string, cobra.ShellCompDirective,
) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveError
	}

	if strings.HasPrefix(toComplete, "/") || strings.HasPrefix(toComplete, ".") {
		// This has to be a file path.
		// Go straight to default (file) completion.
		return nil, cobra.ShellCompDirectiveDefault
	}

	var a []string
	if toComplete == "" {
		// No input yet. Offer both the driver URL schemes and file listing.
		a = append(a, locSchemes...)
		files := locCompListFiles(cmd.Context(), toComplete)
		if len(files) > 0 {
			a = append(a, files...)
		}

		return a, locCompStdDirective
	}

	// We've got some input in toComplete...
	if !stringz.HasAnyPrefix(toComplete, locSchemes...) {
		// But toComplete isn't a full match for any of the driver
		// URL schemes. However, it could still be a partial match.

		a = stringz.FilterPrefix(toComplete, locSchemes...)
		if len(a) == 0 {
			// We're not matching any URL prefix, fall back to default
			// shell completion, i.e. list files.
			return nil, cobra.ShellCompDirectiveDefault
		}

		// Partial match, e.g. "post". So, this could match both
		// a URL such as "postgres://", or a file such as "post.db".
		files := locCompListFiles(cmd.Context(), toComplete)
		if len(files) > 0 {
			a = append(a, files...)
		}

		return a, locCompStdDirective
	}

	// If we got this far, we know that toComplete starts with one of the
	// driver schemes, e.g. "postgres://". There's no possibility that
	// this could be a file completion.

	if strings.HasPrefix(toComplete, string(sqlite3.Type)) {
		// Special handling for sqlite.
		return locCompDoSQLite3(cmd, args, toComplete)
	}

	return locCompDoGenericDriver(cmd, args, toComplete)
}

// locCompDoGenericDriver provides completion for generic SQL drivers.
// Specifically, it's tested with postgres, sqlserver, and mysql. Note that
// sqlserver is slightly different from the others, in that the db name goes
// in a query param, not in the URL path. It might be cleaner to split sqlserver
// off into its own function.
func locCompDoGenericDriver(cmd *cobra.Command, _ []string, toComplete string, //nolint:funlen
) ([]string, cobra.ShellCompDirective) {
	// If we get this far, then toComplete is at least a partial URL
	// starting with "postgres://", "mysql://", etc.

	var (
		ctx  = cmd.Context()
		log  = lg.FromContext(ctx)
		ru   = run.FromContext(ctx)
		drvr driver.SQLDriver
		ploc *parsedLoc
		a    []string // a holds the completion strings to be returned
		err  error
	)

	if err = FinishRunInit(ctx, ru); err != nil {
		log.Error("Init run", lga.Err, err)
		return nil, cobra.ShellCompDirectiveError
	}

	if ploc, err = locCompParseLoc(toComplete); err != nil {
		log.Error("Parse location", lga.Err, err)
		return nil, cobra.ShellCompDirectiveError
	}

	if drvr, err = ru.DriverRegistry.SQLDriverFor(ploc.typ); err != nil {
		log.Error("Load driver", lga.Err, err)
		return nil, cobra.ShellCompDirectiveError
	}

	switch ploc.stageDone { //nolint:exhaustive
	case plocScheme:
		if ploc.user == "" {
			a = []string{
				toComplete,
				toComplete + "username",
				toComplete + "username:password",
			}

			return a, locCompStdDirective
		}

		a = []string{
			toComplete + "@",
			toComplete + ":",
			toComplete + ":@",
			toComplete + ":password@",
		}

		return a, locCompStdDirective
	case plocUser:
		if ploc.pass == "" {
			a = []string{
				toComplete,
				toComplete + "@",
				toComplete + "password@",
			}

			return a, locCompStdDirective
		}

		a = []string{
			toComplete + "@",
		}

		return a, locCompStdDirective
	case plocPass:
		defaultPort := locCompDriverPort(drvr)
		afterHost := locCompAfterHost(ploc.typ)

		if ploc.hostname == "" {
			if defaultPort == "" {
				a = []string{
					toComplete + "localhost" + afterHost,
				}
			} else {
				a = []string{
					toComplete + "localhost" + afterHost,
					toComplete + "localhost:" + defaultPort + afterHost,
				}
			}

			return a, locCompStdDirective
		}

		base, _, _ := strings.Cut(toComplete, "@")
		base += "@"

		if ploc.port <= 0 {
			if defaultPort == "" {
				a = []string{
					toComplete + afterHost,
					base + "localhost" + afterHost,
				}
			} else {
				a = []string{
					toComplete + afterHost,
					toComplete + ":" + defaultPort + afterHost,
					base + "localhost" + afterHost,
					base + "localhost:" + defaultPort + afterHost,
				}
			}

			a = lo.Uniq(stringz.FilterPrefix(toComplete, a...))
			return a, locCompStdDirective
		}

		if defaultPort == "" {
			a = []string{
				base + "localhost" + afterHost,
				toComplete + afterHost,
			}
		} else {
			a = []string{
				base + "localhost" + afterHost,
				base + "localhost:" + defaultPort + afterHost,
				toComplete + afterHost,
			}
		}

		a = stringz.FilterPrefix(toComplete, a...)
		return a, locCompStdDirective
	case plocHostname:
		defaultPort := locCompDriverPort(drvr)
		afterHost := locCompAfterHost(ploc.typ)
		if strings.HasSuffix(toComplete, ":") {
			a = []string{toComplete + defaultPort + afterHost}
			return a, locCompStdDirective
		}

		a = []string{toComplete + afterHost}
		return a, locCompStdDirective

	case plocHost:
		// Special handling for SQLServer. The input is typically of the form:
		//  sqlserver://alice@server?database=db
		// But it can also be of the form:
		//  sqlserver://alice@server/instance?database=db
		if ploc.typ == sqlserver.Type {
			if ploc.du.Path == "/" {
				a = []string{toComplete + "instance?database="}
				return a, locCompStdDirective
			}

			a = []string{toComplete + "?database="}
			return a, locCompStdDirective
		}

		if ploc.name == "" {
			a = []string{toComplete + "db"}
			return a, locCompStdDirective
		}

		a = []string{toComplete + "?"}
		return a, locCompStdDirective

	default:
		// We're at plocName (db name is done), so it's on to conn params.
		return locCompDoConnParams(ploc.du, drvr, toComplete)
	}
}

// locCompDoSQLite3 completes a location starting with "sqlite3://".
// We have special handling for SQLite, because it's not a generic
// driver URL, but rather sqlite3://FILE/PATH?param=X.
func locCompDoSQLite3(cmd *cobra.Command, _ []string, toComplete string, //nolint:funlen
) ([]string, cobra.ShellCompDirective) {
	var (
		ctx  = cmd.Context()
		log  = lg.FromContext(ctx)
		ru   = run.FromContext(ctx)
		drvr driver.SQLDriver
		err  error
	)

	if err = FinishRunInit(ctx, ru); err != nil {
		log.Error("Init run", lga.Err, err)
		return nil, cobra.ShellCompDirectiveError
	}

	if drvr, err = ru.DriverRegistry.SQLDriverFor(sqlite3.Type); err != nil {
		// Shouldn't happen
		log.Error("Cannot load driver", lga.Err, err)
		return nil, cobra.ShellCompDirectiveError
	}

	du, err := dburl.Parse(toComplete)
	if err == nil {
		// Check if we're done with the filepath part, and on to conn params?
		if du.URL.RawQuery != "" || strings.HasSuffix(toComplete, "?") {
			return locCompDoConnParams(du, drvr, toComplete)
		}
	}

	// Build a list of files.
	start := strings.TrimPrefix(toComplete, "sqlite3://")
	paths := locCompListFiles(ctx, start)
	for i := range paths {
		if ioz.IsPathToRegularFile(paths[i]) && paths[i] == start {
			paths[i] += "?"
		}

		paths[i] = "sqlite3://" + paths[i]
	}

	return paths, locCompStdDirective
}

// locCompDoConnParams completes the query params. For example, given
// a toComplete value "sqlite3://my.db?", the result would include values
// such as "sqlite3://my.db?cache=".
func locCompDoConnParams(du *dburl.URL, drvr driver.SQLDriver, toComplete string) (
	[]string, cobra.ShellCompDirective,
) {
	var (
		a                         []string
		query                     = du.RawQuery
		drvrParamKeys, drvrParams = locCompGetConnParams(drvr)
	)

	if query == "" {
		a = stringz.PrefixSlice(drvrParamKeys, toComplete)
		a = stringz.SuffixSlice(a, "=")
		return a, locCompStdDirective
	}

	actualKeys, err := stringz.QueryParamKeys(query)
	if err != nil || len(actualKeys) == 0 {
		return nil, cobra.ShellCompDirectiveError
	}

	actualValues, err := url.ParseQuery(query)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	elements := strings.Split(query, "&")

	// could be "sslmo", "sslmode", "sslmode=", "sslmode=dis"
	lastElement := elements[len(elements)-1]
	stump := strings.TrimSuffix(toComplete, lastElement)

	before, _, ok := strings.Cut(lastElement, "=")

	if !ok {
		candidateKeys := stringz.ElementsHavingPrefix(drvrParamKeys, before)
		candidateKeys = lo.Reject(candidateKeys, func(candidateKey string, index int) bool {
			// We don't want the same candidate to show up twice, so we exclude
			// it, but only if it already has a value in the query string.
			if slices.Contains(actualKeys, candidateKey) {
				vals, ok := actualValues[candidateKey]
				if !ok || len(vals) == 0 {
					return false
				}

				for _, val := range vals {
					if val != "" {
						return true
					}
				}
			}

			return false
		})

		for i := range candidateKeys {
			s := stump + candidateKeys[i] + "="
			a = append(a, s)
		}

		return a, locCompStdDirective
	}

	candidateVals := drvrParams[before]
	for i := range candidateVals {
		s := stump + before + "=" + candidateVals[i]
		a = append(a, s)
	}

	a = stringz.FilterPrefix(toComplete, a...)
	if len(a) == 0 {
		// If it's an unknown value, append "&" to move
		// on to a further query param.
		a = []string{toComplete + "&"}
		return a, locCompStdDirective
	}

	if len(a) == 1 && a[0] == toComplete {
		// If it's a completed known value ("sslmode=disable"),
		// then append "?" to move on to a further query param.
		a[0] += "&"
	}

	return a, locCompStdDirective
}

// locCompGetConnParams returns the driver's connection params. The returned
// keys are sorted appropriately for the driver, and are query encoded.
func locCompGetConnParams(drvr driver.SQLDriver) (keys []string, params map[string][]string) {
	ogParams := drvr.ConnParams()
	ogKeys := lo.Keys(ogParams)
	slices.Sort(ogKeys)

	if drvr.DriverMetadata().Type == sqlserver.Type {
		// For SQLServer, the "database" key should come first, because
		// it's required.
		ogKeys = lo.Without(ogKeys, "database")
		ogKeys = append([]string{"database"}, ogKeys...)
	}

	keys = make([]string, len(ogKeys))
	params = make(map[string][]string, len(ogParams))
	for i := range ogKeys {
		k := url.QueryEscape(ogKeys[i])
		keys[i] = k
		params[k] = ogParams[ogKeys[i]]
	}

	return keys, params
}

// locCompDriverPort returns the default port for the driver, as a string,
// or empty string if not applicable.
func locCompDriverPort(drvr driver.SQLDriver) string {
	p := drvr.DriverMetadata().DefaultPort
	if p <= 0 {
		return ""
	}

	return strconv.Itoa(p)
}

// locCompAfterHost returns the next text to show after the host
// part of the URL is complete.
func locCompAfterHost(typ source.DriverType) string {
	if typ == sqlserver.Type {
		return "?database="
	}

	return "/"
}

// locCompParseLoc parses a location string. The string can
// be in various stages of construction, e.g. "postgres://user" or
// "postgres://user@locahost/db". The stage is noted in parsedLoc.stageDone.
func locCompParseLoc(loc string) (*parsedLoc, error) {
	p := &parsedLoc{loc: loc}
	if !stringz.HasAnyPrefix(loc, locSchemes...) {
		return p, nil
	}

	var (
		s     string
		ok    bool
		err   error
		creds string
	)

	p.stageDone = plocScheme
	p.scheme, s, ok = strings.Cut(loc, "://")
	p.typ = source.DriverType(p.scheme)

	if s == "" || !ok {
		return p, nil
	}

	creds, s, ok = strings.Cut(s, "@")
	if creds != "" {
		// creds can be:
		//  user:pass
		//  user:
		//  user

		var hasColon bool
		p.user, p.pass, hasColon = strings.Cut(creds, ":")
		if hasColon {
			p.stageDone = plocUser
		}
	}
	if !ok {
		return p, nil
	}

	p.stageDone = plocPass

	// At a minimum, we're at this point:
	//  postgres://

	// Next we're looking for user:pass, e.g.
	//  postgres://alice:huzzah@localhost

	if p.du, err = dburl.Parse(p.loc); err != nil {
		return p, errz.Err(err)
	}
	du := p.du
	p.scheme = du.OriginalScheme
	if du.User != nil {
		p.user = du.User.Username()
		p.pass, _ = du.User.Password()
	}
	p.hostname = du.Hostname()

	if strings.ContainsRune(du.URL.Host, ':') {
		p.stageDone = plocHostname
	}

	if du.Port() != "" {
		p.stageDone = plocHostname
		p.port, err = strconv.Atoi(du.Port())
		if err != nil {
			p.port = -1
			return p, nil //nolint:nilerr
		}
	}

	switch p.typ { //nolint:exhaustive
	default:
	case sqlserver.Type:
		var u *url.URL
		if u, err = url.ParseRequestURI(loc); err == nil {
			var vals url.Values
			if vals, err = url.ParseQuery(u.RawQuery); err == nil {
				p.name = vals.Get("database")
			}
		}

	case postgres.Type, mysql.Type:
		p.name = strings.TrimPrefix(du.Path, "/")
	}

	if strings.HasSuffix(s, "/") || strings.HasSuffix(s, `\?`) || du.URL.Path != "" {
		p.stageDone = plocHost
	}

	if strings.HasSuffix(s, "?") {
		p.stageDone = plocPath
	}

	if du.URL.RawQuery != "" {
		p.stageDone = plocPath
	}

	return p, nil
}

// parsedLoc is a parsed representation of a driver location URL.
// It can represent partial or fully constructed locations. The stage
// of construction is noted in parsedLoc.stageDone.
type parsedLoc struct {
	// loc is the original unparsed location value.
	loc string

	// stageDone indicates what stage of construction the location
	// string is in.
	stageDone plocStage

	// typ is the associated source driver type, which may
	// be empty until later determination.
	typ source.DriverType

	// scheme is the original location scheme
	scheme string

	// user is the username, if applicable.
	user string

	// pass is the password, if applicable.
	pass string

	// hostname is the hostname, if applicable.
	hostname string

	// port is the port number, or 0 if not applicable.
	port int

	// name is the database name.
	name string

	// du holds the parsed db url. This may be nil.
	du *dburl.URL
}

// plocStage is an enum indicating what stage of construction
// a location string is in.
type plocStage string

const (
	plocInit     plocStage = ""
	plocScheme   plocStage = "scheme"
	plocUser     plocStage = "user"
	plocPass     plocStage = "pass"
	plocHostname plocStage = "hostname"
	plocHost     plocStage = "host" // host is hostname+port, or just hostname
	plocPath     plocStage = "path"
)

var locSchemes = []string{
	"mysql://",
	"postgres://",
	"sqlite3://",
	"sqlserver://",
}

// locCompListFiles completes filenames. This function tries to
// mimic what a shell would do. Any errors are logged and swallowed.
func locCompListFiles(ctx context.Context, toComplete string) []string {
	var (
		start = toComplete
		files []string
		err   error
	)

	if start == "" {
		start, err = os.Getwd()
		if err != nil {
			return nil
		}
		files, err = ioz.ReadDir(start, false, true, false)
		if err != nil {
			lg.FromContext(ctx).Warn("Read dir", lga.Path, start, lga.Err, err)
		}

		return files
	}

	if strings.HasSuffix(start, "/") {
		files, err = ioz.ReadDir(start, true, true, false)
		if err != nil {
			lg.FromContext(ctx).Warn("Read dir", lga.Path, start, lga.Err, err)
		}
		return files
	}

	// We could have a situation like this:
	//  + [working dir]
	//    - my.db
	//    - my/my2.db

	dir := filepath.Dir(start)
	fi, err := os.Stat(dir)
	if err == nil && fi.IsDir() {
		files, err = ioz.ReadDir(dir, true, true, false)
		if err != nil {
			lg.FromContext(ctx).Warn("Read dir", lga.Path, start, lga.Err, err)
		}
	} else {
		files = []string{start}
	}

	files = stringz.FilterPrefix(toComplete, files...)
	return files
}
