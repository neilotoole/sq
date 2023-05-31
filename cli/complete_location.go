package cli

import (
	"context"
	"net/url"
	"strconv"
	"strings"

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

	"github.com/neilotoole/sq/libsq/core/errz"

	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

// completeAddLocation provides completion for the "sq add LOCATION" arg.
func completeAddLocation(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	log := lg.FromContext(ctx)
	ru := run.FromContext(ctx)
	if err := FinishRunInit(ctx, ru); err != nil {
		log.Error("Init run", lga.Err, err)
		return nil, cobra.ShellCompDirectiveError
	}

	var a []string
	d := cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveKeepOrder

	if toComplete == "" {
		return locSchemes, cobra.ShellCompDirectiveDefault
	}

	if !stringz.HasAnyPrefix(toComplete, locSchemes...) {
		return filterPrefix(toComplete, locSchemes...), d
	}

	ploc := parseLoc(toComplete)
	stageDone := ploc.stageDone
	log.Debug("ploc stage", lga.Val, stageDone)

	switch stageDone {
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

		if ploc.port <= 0 {
			a = []string{
				toComplete + "/",
				toComplete + ":5432/",
				base + "localhost/",
				base + "localhost:5432/",
			}

			a = lo.Uniq(stringz.FilterPrefix(toComplete, a...))
			return a, d
		}

		a = []string{
			base + "localhost/",
			base + "localhost:5432/",
			toComplete + "/",
		}

		a = stringz.FilterPrefix(toComplete, a...)
		return a, d
	case plocHostname:
		if strings.HasSuffix(toComplete, ":") {
			a = []string{toComplete + "5432/"}
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
		// We're at plocName, continue below
	}

	log.Debug("at ploc name")

	a, d = completeConnParams(ctx, ploc, toComplete)
	log.Debug("conn params", "params", toComplete)

	return a, d
}

func completeConnParams(ctx context.Context, ploc *parsedLoc, toComplete string) ([]string, cobra.ShellCompDirective) {
	d := cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveKeepOrder
	var a []string
	drvrParams := getDriverConnParams(ctx, ploc.typ)
	drvrParamKeys := lo.Keys(drvrParams)
	slices.Sort(drvrParamKeys)
	query := ploc.du.RawQuery

	if query == "" {
		a = stringz.PrefixSlice(drvrParamKeys, toComplete)
		a = stringz.SuffixSlice(a, "=")
		return a, d
	}

	actualKeys, err := stringz.QueryParamKeys(query)
	if err != nil || len(actualKeys) == 0 {
		return nil, cobra.ShellCompDirectiveError
	}

	elements := strings.Split(ploc.du.RawQuery, "&")

	// could be "sslmo", "sslmode", "sslmode=", "sslmode=dis"
	lastElement := elements[len(elements)-1]
	stump := strings.TrimSuffix(toComplete, lastElement)

	before, _, ok := strings.Cut(lastElement, "=")

	if !ok {
		possibleKeys := stringz.ElementsHavingPrefix(drvrParamKeys, before)
		for i := range possibleKeys {
			s := stump + possibleKeys[i] + "="
			a = append(a, s)
		}

		return a, d
	}

	possibleVals := drvrParams[before]
	if len(possibleVals) == 0 {
		return nil, d
	}

	for i := range possibleVals {
		s := stump + before + "=" + possibleVals[i]
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

func getDriverConnParams(ctx context.Context, typ source.DriverType) map[string][]string {
	ru := run.FromContext(ctx)

	drvr, err := ru.DriverRegistry.DriverFor(typ)
	if err != nil {
		// Shouldn't happen
		lg.FromContext(ctx).Error("Unknown driver type", lga.Driver, typ)
		return map[string][]string{}
	}

	sqlDrvr, ok := drvr.(driver.SQLDriver)
	if !ok {
		// Shouldn't happen
		lg.FromContext(ctx).Error("Not a driver.SQLDriver", lga.Driver, typ)
		return map[string][]string{}
	}

	return sqlDrvr.ConnParams()
}

func parseLoc(loc string) *parsedLoc {
	p := &parsedLoc{}
	p.loc = loc

	if !stringz.HasAnyPrefix(loc, locSchemes...) {
		return p
	}

	p.stageDone = plocScheme

	var s string
	var ok bool
	p.scheme, s, ok = strings.Cut(loc, "://")
	p.typ = source.DriverType(p.scheme)

	if s == "" || !ok {
		return p
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
		return p
	}

	p.stageDone = plocPass

	// At a minimum, we're at this point:
	//  postgres://

	// Next we're looking for user:pass, e.g.
	//  postgres://alice:huzzah@localhost

	//
	du, err := dburl.Parse(p.loc)
	if err != nil {
		return p
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
		p.port, _ = strconv.Atoi(du.Port())
		if err != nil {
			p.port = -1
			return p
		}
	}

	switch p.typ {
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
		return p
	}

	if strings.HasSuffix(s, "?") {
		p.stageDone = plocName
	}

	if du.URL.RawQuery != "" {
		p.stageDone = plocName
	}

	return p
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
	ext string

	// dsn is the connection "data source name" that can be used in a
	// call to sql/Open. Empty for non-SQL locations.
	dsn string

	// du holds the parsed db url.
	du *dburl.URL
}

func (p *parsedLoc) text() string {
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
	plocScheme             = "scheme"
	plocUser               = "user"
	plocPass               = "pass"
	plocHostname           = "hostname"
	plocHost               = "host" // host is hostname+port, or just hostname
	plocName               = "name"
)

func filterPrefix(prefix string, a ...string) []string {
	return lo.Filter(a, func(item string, index int) bool {
		return strings.HasPrefix(item, prefix)
	})
}

type dsnParser struct {
	input string
	url   *url.URL
}

func (p *dsnParser) parse(input string) error {
	p.input = input
	u, err := url.ParseRequestURI(input)
	if err != nil {
		return errz.Err(err)
	}
	p.url = u
	return nil
}

var locSchemes = []string{
	"mysql://",
	"postgres://",
	"sqlserver://",
}

var addLocBasics = []string{
	`mysql://user:pass@localhost/db`,
	`postgres://user:pass@localhost/db`,
	`sqlserver://user:pass@localhost\?database=db`,
}

/*
sq add postgres://sakila:p_ssW0rd@192.168.50.132/sakila
sq add sqlserver://sakila:p_ssW0rd@192.168.50.130\?database=sakila
sq add sqlserver://sakila:p_ssW0rd@192.168.50.130\?database=sakila&\keepAlive=30

*/
