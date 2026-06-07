package rqlite

import (
	"context"
	"net"
	"net/url"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// errw wraps any error from the db. It should be called at every
// interaction with the db. If err is nil, nil is returned. Certain errors
// are wrapped in specific error types, e.g. errz.NotExistError.
func errw(err error) error {
	switch {
	case err == nil:
		return nil
	case strings.Contains(err.Error(), "no such table:"):
		// rqlite returns SQLite-formatted error messages over the wire,
		// so the "no such table:" prefix carries through verbatim.
		return driver.NewNotExistError(err)
	default:
		return errz.Err(err)
	}
}

// docsLocalhostAnchor is the docs URL for the single-node-localhost
// case. Kept as a package-level constant so the add-time hint and the
// DNS error rewrite point at the same place.
const docsLocalhostAnchor = "https://sq.io/docs/drivers/rqlite#single-node-localhost"

// maybeWarnLocalhostDiscovery emits a one-line Warn log when src's URL
// host is loopback AND disableClusterDiscovery is not explicitly set on
// the query string. Single-node localhost (Docker container reached
// from the host) is the most common newcomer setup and gorqlite's
// default cluster discovery fails opaquely there; a Warn at add/open
// time provides a breadcrumb in the log file pointing at the docs.
//
// Best-effort: any failure to parse src.Location or extract the host
// is a silent no-op. The warning must never affect Open behavior.
func maybeWarnLocalhostDiscovery(ctx context.Context, src *source.Source) {
	u, err := url.Parse(src.Location)
	if err != nil {
		return
	}
	if u.Query().Has("disableClusterDiscovery") {
		// User has made an explicit choice (true or false). Don't
		// second-guess them.
		return
	}
	host := u.Hostname()
	if host == "" {
		return
	}
	if !isLoopbackHost(host) {
		return
	}
	lg.FromContext(ctx).Warn(
		"rqlite: source points at loopback but disableClusterDiscovery is not set; "+
			"single-node localhost setups typically need ?disableClusterDiscovery=true "+
			"to avoid peer-discovery failures from the host. See "+docsLocalhostAnchor,
		lga.Src, src.Handle,
	)
}

// isLoopbackHost reports whether host is a literal loopback reference.
// Matches "localhost" (case-insensitive) and any IP whose net.IP
// representation reports IsLoopback (covers 127.0.0.0/8, ::1, and
// ::ffff:127.x.x.x mappings). No DNS resolution is performed.
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// rewritePeerDNSError is added in Task 4 below.
