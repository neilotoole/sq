package databricks

import (
	"errors"
	"net"
	"net/url"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source/location"
)

// Query parameter names, as used in the sq location. The names that
// databricks-sql-go itself reads (catalog, schema, accessToken and so on)
// are passed through unchanged; only http_path is consumed by sq.
const (
	paramHTTPPath     = "http_path"
	paramCatalog      = "catalog"
	paramSchema       = "schema"
	paramAccessToken  = "accessToken"
	paramAuthType     = "authType"
	paramClientID     = "clientID"
	paramClientIDAlt  = "clientId"
	paramClientSecret = "clientSecret"

	// tokenUser is the userinfo username that databricks-sql-go requires
	// when the personal access token is supplied as the password.
	tokenUser = "token"
)

// buildDSN converts the sq location loc into the DSN accepted by
// databricks-sql-go (sql.Open("databricks", dsn)). If catalog or schema is
// not empty, it sets the "catalog" or "schema" param, replacing any value in
// loc. This is how the --src.schema flag, which sets source.Source.Catalog
// and source.Source.Schema, takes effect.
//
// The location has the form:
//
//	databricks://token:PAT@HOST[:PORT]/HTTP_PATH[?PARAM=VAL...]
//
// where HTTP_PATH is the warehouse path, e.g. /sql/1.0/warehouses/abc123.
// The HTTP path may instead be supplied as the "http_path" query param, in
// which case the URL path must be empty. If both are present they must be
// equal.
//
// Without userinfo, the location must select another auth method through
// query params: accessToken, or clientID plus clientSecret (OAuth M2M), or
// authType=OauthU2M. All query params other than http_path are passed
// through to databricks-sql-go, which uses the ones it knows (catalog,
// schema, maxRows, timeout and so on) and treats any remaining ones as
// Databricks session parameters.
//
// The DSN uses https, except for a loopback host (localhost, 127.0.0.1, ::1),
// where it uses http. A real workspace is never on a loopback address, so
// this rule only matters for a local proxy or a test server, and it avoids
// a TLS setup that databricks-sql-go can't configure from a DSN.
//
// The returned DSN contains the credential. Callers must not log it.
func buildDSN(loc, catalog, schema string) (string, error) {
	u, err := url.Parse(loc)
	if err != nil {
		// url.Error embeds the raw location, which may hold the token.
		if uerr, ok := errors.AsType[*url.Error](err); ok {
			err = uerr.Err
		}
		return "", errz.Wrapf(err, "parse databricks location %s", location.Redact(loc))
	}

	if u.Scheme != "databricks" {
		return "", errz.Errorf("databricks location must start with \"databricks://\": %s",
			location.Redact(loc))
	}

	host := u.Hostname()
	if host == "" {
		return "", errz.New("databricks location has no host: " +
			"expected databricks://token:PAT@HOST/HTTP_PATH")
	}

	port := u.Port()
	if port == "" {
		port = "443"
	}

	query := u.Query()

	httpPath := strings.TrimSuffix(u.Path, "/")
	if qpath := query.Get(paramHTTPPath); qpath != "" {
		if !strings.HasPrefix(qpath, "/") {
			qpath = "/" + qpath
		}
		qpath = strings.TrimSuffix(qpath, "/")
		if httpPath != "" && httpPath != qpath {
			return "", errz.Errorf("databricks location has two different HTTP paths: "+
				"URL path %q and %s param %q", httpPath, paramHTTPPath, qpath)
		}
		httpPath = qpath
		query.Del(paramHTTPPath)
	}
	if httpPath == "" {
		return "", errz.New("databricks location has no HTTP path: " +
			"expected databricks://token:PAT@HOST/sql/1.0/warehouses/ID " +
			"(or the " + paramHTTPPath + " query param)")
	}

	var userinfo *url.Userinfo
	if u.User != nil {
		if u.User.Username() != tokenUser {
			return "", errz.Errorf("databricks location username must be %q "+
				"(the password is the personal access token), but got %q",
				tokenUser, u.User.Username())
		}
		pass, _ := u.User.Password()
		if pass == "" {
			return "", errz.New("databricks location has no personal access token: " +
				"expected databricks://token:PAT@HOST/HTTP_PATH")
		}
		userinfo = url.UserPassword(tokenUser, pass)
	} else if err = checkQueryAuth(query); err != nil {
		return "", err
	}

	if catalog != "" {
		query.Set(paramCatalog, catalog)
	}
	if schema != "" {
		query.Set(paramSchema, schema)
	}

	scheme := "https"
	if isLoopback(host) {
		scheme = "http"
	}

	dsn := url.URL{
		Scheme:   scheme,
		User:     userinfo,
		Host:     net.JoinHostPort(host, port),
		Path:     httpPath,
		RawQuery: query.Encode(),
	}
	return dsn.String(), nil
}

// checkQueryAuth verifies that, in the absence of a token in the userinfo,
// the query params select an auth method that databricks-sql-go can use.
func checkQueryAuth(query url.Values) error {
	hasClientID := query.Get(paramClientID) != "" || query.Get(paramClientIDAlt) != ""
	hasClientSecret := query.Get(paramClientSecret) != ""

	switch strings.ToLower(query.Get(paramAuthType)) {
	case "pat":
		if query.Get(paramAccessToken) == "" {
			return errz.Errorf("databricks location has %s=Pat but no %s param",
				paramAuthType, paramAccessToken)
		}
		return nil
	case "oauthm2m":
		if !hasClientID || !hasClientSecret {
			return errz.Errorf("databricks location has %s=OauthM2M but is missing "+
				"%s or %s", paramAuthType, paramClientID, paramClientSecret)
		}
		return nil
	case "oauthu2m":
		return nil
	case "":
	default:
		return errz.Errorf("databricks location has unknown %s value %q: "+
			"expected Pat, OauthM2M or OauthU2M", paramAuthType, query.Get(paramAuthType))
	}

	switch {
	case query.Get(paramAccessToken) != "":
		return nil
	case hasClientID && hasClientSecret:
		return nil
	default:
		return errz.New("databricks location has no credentials: use " +
			"databricks://token:PAT@HOST/HTTP_PATH, or for OAuth M2M the " +
			paramClientID + " and " + paramClientSecret + " query params")
	}
}

// isLoopback returns true if host is "localhost" or a loopback IP address.
func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// secretsOf returns the credential values in dsn that must not appear in
// error messages: the userinfo password and any secret query param values.
func secretsOf(dsn string) []string {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil
	}

	var secrets []string
	if u.User != nil {
		if pass, ok := u.User.Password(); ok && pass != "" {
			secrets = append(secrets, pass)
		}
	}
	for key, vals := range u.Query() {
		if !location.IsSecretQueryParam(key) {
			continue
		}
		for _, v := range vals {
			if v != "" {
				secrets = append(secrets, v)
			}
		}
	}
	return secrets
}

// scrub returns err with any of the secrets removed from its message. If
// err's message does not contain any secret, err is returned unchanged, so
// that wrapped error types remain detectable.
func scrub(err error, secrets []string) error {
	if err == nil {
		return nil
	}

	msg := err.Error()
	redacted := msg
	for _, s := range secrets {
		redacted = strings.ReplaceAll(redacted, s, "xxxxx")
	}
	if redacted == msg {
		return err
	}
	return errz.New(redacted)
}
