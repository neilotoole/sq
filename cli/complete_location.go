package cli

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"

	"golang.org/x/exp/slog"

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

// completeAddLocation provides completion for the "sq add LOCATION" arg.
func completeAddLocation(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var (
		ctx = cmd.Context()
		log = lg.FromContext(ctx)
		ru  = run.FromContext(ctx)
		lch = &locCompleteHelper{
			ru:  ru,
			log: log,
		}
	)

	if err := FinishRunInit(ctx, ru); err != nil {
		log.Error("Init run", lga.Err, err)
		return nil, cobra.ShellCompDirectiveError
	}

	var a []string
	const d = cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveKeepOrder

	if toComplete == "" {
		return locSchemes, d
	}

	if !stringz.HasAnyPrefix(toComplete, locSchemes...) {
		return stringz.FilterPrefix(toComplete, locSchemes...), d
	}

	ploc, err := lch.parseLoc(toComplete)
	if err != nil {
		log.Error("parse location", lga.Err, err)
		return nil, cobra.ShellCompDirectiveError
	}
	stageDone := ploc.stageDone
	log.Debug("ploc stage", lga.Val, stageDone)

	switch stageDone { //nolint:exhaustive
	case plocScheme:
		if ploc.user == "" {
			a = []string{
				toComplete,
				toComplete + "username",
				toComplete + "username:password",
			}

			return a, d
		}

		a = []string{
			toComplete + "@",
			toComplete + ":",
			toComplete + ":@",
			toComplete + ":password@",
		}

		return a, d
	case plocUser:
		if ploc.pass == "" {
			a = []string{
				toComplete,
				toComplete + "@",
				toComplete + "password@",
			}

			return a, d
		}

		a = []string{
			toComplete + "@",
		}

		return a, d
	case plocPass:
		if ploc.hostname == "" {
			a = []string{
				toComplete + "localhost/",
				toComplete + "localhost:5432/",
			}

			return a, d
		}

		base, _, _ := strings.Cut(toComplete, "@")
		base += "@"

		defaultPort := lch.driverPort()

		if ploc.port <= 0 {
			if defaultPort == "" {
				a = []string{
					toComplete + "/",
					base + "localhost/",
				}
			} else {
				a = []string{
					toComplete + "/",
					toComplete + ":" + defaultPort + "/",
					base + "localhost/",
					base + "localhost:" + defaultPort + "/",
				}
			}

			a = lo.Uniq(stringz.FilterPrefix(toComplete, a...))
			return a, d
		}

		if defaultPort == "" {
			a = []string{
				base + "localhost/",
				toComplete + "/",
			}
		} else {
			a = []string{
				base + "localhost/",
				base + "localhost:" + defaultPort + "/",
				toComplete + "/",
			}
		}

		a = stringz.FilterPrefix(toComplete, a...)
		return a, d
	case plocHostname:
		defaultPort := lch.driverPort()
		if strings.HasSuffix(toComplete, ":") {
			a = []string{toComplete + defaultPort + "/"}
			return a, d
		}

		a = []string{toComplete + "/"}
		return a, d

	case plocHost:
		if ploc.name == "" {
			a = []string{toComplete + "db"}
			return a, d
		}

		a = []string{toComplete + "?"}
		return a, d

	default:
		// We're at plocName (db name is done), so it's on to conn params
		return completeConnParams(lch, toComplete)
	}
}

func completeConnParams(lch *locCompleteHelper, toComplete string) ([]string, cobra.ShellCompDirective) {
	var (
		d             = cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveKeepOrder
		a             []string
		drvrParams    = lch.drvr.ConnParams()
		drvrParamKeys = lo.Keys(drvrParams)
		query         = lch.ploc.du.RawQuery
	)

	slices.Sort(drvrParamKeys)

	if query == "" {
		a = stringz.PrefixSlice(drvrParamKeys, toComplete)
		a = stringz.SuffixSlice(a, "=")
		return a, d
	}

	actualKeys, err := stringz.QueryParamKeys(query)
	if err != nil || len(actualKeys) == 0 {
		return nil, cobra.ShellCompDirectiveError
	}

	actualValues, err := url.ParseQuery(query)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	elements := strings.Split(lch.ploc.du.RawQuery, "&")

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

		return a, d
	}

	candidateVals := drvrParams[before]
	if len(candidateVals) == 0 {
		return nil, d
	}

	for i := range candidateVals {
		s := stump + before + "=" + candidateVals[i]
		a = append(a, s)
	}

	a = stringz.FilterPrefix(toComplete, a...)
	if len(a) == 0 {
		// If it's an unknown value, append "&" to move
		// on to a further query param.
		a = []string{toComplete + "&"}
		return a, d
	}

	if len(a) == 1 && a[0] == toComplete {
		// If it's a completed known value ("sslmode=disable"),
		// then append "?" to move on to a further query param.
		a[0] += "&"
	}

	return a, d
}

// locCompleteHelper is a helper for completing the "sq add location" arg.
type locCompleteHelper struct {
	ru   *run.Run
	log  *slog.Logger
	ploc *parsedLoc
	drvr driver.SQLDriver
}

// driverPort returns the default port for the driver
// type from h.ploc.typ, or empty string if not applicable.
func (h *locCompleteHelper) driverPort() string {
	if h.drvr == nil {
		panic("invoke loadDriver first")
	}

	p := h.drvr.DriverMetadata().DefaultPort
	if p <= 0 {
		return ""
	}

	return strconv.Itoa(p)
}

func (h *locCompleteHelper) parseLoc(loc string) (*parsedLoc, error) {
	h.ploc = &parsedLoc{loc: loc}
	p := h.ploc

	if !stringz.HasAnyPrefix(loc, locSchemes...) {
		return p, nil
	}

	p.stageDone = plocScheme

	var s string
	var ok bool
	p.scheme, s, ok = strings.Cut(loc, "://")
	p.typ = source.DriverType(p.scheme)

	var err error
	if h.drvr, err = h.ru.DriverRegistry.SQLDriverFor(p.typ); err != nil {
		return nil, err
	}

	if s == "" || !ok {
		return p, nil
	}

	var creds string
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

	//
	du, err := dburl.Parse(p.loc)
	if err != nil {
		return p, errz.Err(err)
	}
	p.du = du

	p.scheme = du.OriginalScheme
	p.dsn = du.DSN
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

	case sqlite3.Type:
		// FIXME: implement
	case postgres.Type, mysql.Type:
		p.name = strings.TrimPrefix(du.Path, "/")
	}

	if strings.HasSuffix(s, "/") || strings.HasSuffix(s, `\?`) || du.URL.Path != "" {
		p.stageDone = plocHost
	}

	if p.name == "" {
		return p, nil
	}

	if strings.HasSuffix(s, "?") {
		p.stageDone = plocPath
	}

	if du.URL.RawQuery != "" {
		p.stageDone = plocPath
	}

	return p, nil
}

// parsedLoc is a parsed representation of a source location.
type parsedLoc struct {
	// loc is the original unparsed location value.
	loc string

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

	// port is the port number or 0 if not applicable.
	port int

	// name is the "source name", e.g. "sakila". Typically this
	// is the database name, but for a file location such
	// as "/path/to/things.xlsx" it would be "things".
	name string

	// ext is the file extension, if applicable.
	ext string //nolint:unused

	// dsn is the connection "data source name" that can be used in a
	// call to sql/Open. Empty for non-SQL locations.
	dsn string

	// du holds the parsed db url.
	du *dburl.URL
}

// FIXME: do we still parsedLoc.text?
func (p *parsedLoc) text() string { //nolint:unused
	if p == nil {
		return ""
	}

	if p.du != nil {
		return p.du.String()
	}

	sb := strings.Builder{}
	if p.typ == "" {
		return sb.String()
	}

	sb.WriteString(p.typ.String())
	sb.WriteString("://")

	if p.hostname == "" {
		return sb.String()
	}

	sb.WriteString(p.hostname)
	if p.port >= 0 {
		sb.WriteString(strconv.Itoa(p.port))
	}

	return sb.String()
}

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
	"sqlserver://",
}