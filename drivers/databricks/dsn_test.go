package databricks_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/databricks"
)

const (
	testHost = "dbc-a1b2c3d4-e5f6.cloud.databricks.com"
	testPAT  = "dapi0123456789abcdef"
)

func TestBuildDSN(t *testing.T) {
	testCases := []struct {
		name string
		loc  string
		want string
	}{
		{
			name: "pat",
			loc:  "databricks://token:" + testPAT + "@" + testHost + "/sql/1.0/warehouses/abc123",
			want: "https://token:" + testPAT + "@" + testHost + ":443/sql/1.0/warehouses/abc123",
		},
		{
			name: "explicit_port",
			loc:  "databricks://token:" + testPAT + "@" + testHost + ":8443/sql/1.0/warehouses/abc123",
			want: "https://token:" + testPAT + "@" + testHost + ":8443/sql/1.0/warehouses/abc123",
		},
		{
			name: "trailing_slash_in_path",
			loc:  "databricks://token:" + testPAT + "@" + testHost + "/sql/1.0/warehouses/abc123/",
			want: "https://token:" + testPAT + "@" + testHost + ":443/sql/1.0/warehouses/abc123",
		},
		{
			name: "catalog_and_schema",
			loc:  "databricks://token:" + testPAT + "@" + testHost + "/sql/1.0/warehouses/abc123?schema=sakila&catalog=main",
			want: "https://token:" + testPAT + "@" + testHost + ":443/sql/1.0/warehouses/abc123?catalog=main&schema=sakila",
		},
		{
			name: "http_path_param",
			loc: "databricks://token:" + testPAT + "@" + testHost +
				"?http_path=%2Fsql%2F1.0%2Fwarehouses%2Fabc123&catalog=main",
			want: "https://token:" + testPAT + "@" + testHost + ":443/sql/1.0/warehouses/abc123?catalog=main",
		},
		{
			name: "http_path_param_no_leading_slash",
			loc:  "databricks://token:" + testPAT + "@" + testHost + "/?http_path=sql/1.0/warehouses/abc123",
			want: "https://token:" + testPAT + "@" + testHost + ":443/sql/1.0/warehouses/abc123",
		},
		{
			name: "http_path_param_matches_path",
			loc: "databricks://token:" + testPAT + "@" + testHost +
				"/sql/1.0/warehouses/abc123?http_path=/sql/1.0/warehouses/abc123",
			want: "https://token:" + testPAT + "@" + testHost + ":443/sql/1.0/warehouses/abc123",
		},
		{
			name: "session_param_passes_through",
			loc:  "databricks://token:" + testPAT + "@" + testHost + "/sql/1.0/warehouses/abc123?timezone=UTC&maxRows=500",
			want: "https://token:" + testPAT + "@" + testHost + ":443/sql/1.0/warehouses/abc123?maxRows=500&timezone=UTC",
		},
		{
			name: "oauth_m2m",
			loc:  "databricks://" + testHost + "/sql/1.0/warehouses/abc123?clientID=my-id&clientSecret=my-secret",
			want: "https://" + testHost + ":443/sql/1.0/warehouses/abc123?clientID=my-id&clientSecret=my-secret",
		},
		{
			name: "oauth_m2m_explicit_auth_type",
			loc: "databricks://" + testHost +
				"/sql/1.0/warehouses/abc123?authType=OauthM2M&clientId=my-id&clientSecret=my-secret",
			want: "https://" + testHost +
				":443/sql/1.0/warehouses/abc123?authType=OauthM2M&clientId=my-id&clientSecret=my-secret",
		},
		{
			name: "access_token_param",
			loc:  "databricks://" + testHost + "/sql/1.0/warehouses/abc123?accessToken=" + testPAT,
			want: "https://" + testHost + ":443/sql/1.0/warehouses/abc123?accessToken=" + testPAT,
		},
		{
			name: "oauth_u2m",
			loc:  "databricks://" + testHost + "/sql/1.0/warehouses/abc123?authType=OauthU2M",
			want: "https://" + testHost + ":443/sql/1.0/warehouses/abc123?authType=OauthU2M",
		},
		{
			name: "loopback_ipv4_uses_http",
			loc:  "databricks://token:" + testPAT + "@127.0.0.1:34567/sql/1.0/warehouses/abc123",
			want: "http://token:" + testPAT + "@127.0.0.1:34567/sql/1.0/warehouses/abc123",
		},
		{
			name: "loopback_localhost_uses_http",
			loc:  "databricks://token:" + testPAT + "@localhost/sql/1.0/warehouses/abc123",
			want: "http://token:" + testPAT + "@localhost:443/sql/1.0/warehouses/abc123",
		},
		{
			name: "loopback_ipv6_uses_http",
			loc:  "databricks://token:" + testPAT + "@[::1]:8080/sql/1.0/warehouses/abc123",
			want: "http://token:" + testPAT + "@[::1]:8080/sql/1.0/warehouses/abc123",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := databricks.BuildDSN(tc.loc)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestBuildDSN_CatalogSchemaOverride tests that the catalog and schema of
// the source, which the --src.schema flag sets, replace those of the location.
func TestBuildDSN_CatalogSchemaOverride(t *testing.T) {
	base := "databricks://token:" + testPAT + "@" + testHost + "/sql/1.0/warehouses/abc123"
	prefix := "https://token:" + testPAT + "@" + testHost + ":443/sql/1.0/warehouses/abc123"

	testCases := []struct {
		name    string
		loc     string
		catalog string
		schema  string
		want    string
	}{
		{"none", base, "", "", prefix},
		{"both_added", base, "samples", "nyctaxi", prefix + "?catalog=samples&schema=nyctaxi"},
		{"schema_only", base, "", "nyctaxi", prefix + "?schema=nyctaxi"},
		{"catalog_only", base, "samples", "", prefix + "?catalog=samples"},
		{
			"replaces_location_values", base + "?catalog=main&schema=sakila",
			"samples", "nyctaxi", prefix + "?catalog=samples&schema=nyctaxi",
		},
		{
			"keeps_unset_location_values", base + "?catalog=main&schema=sakila",
			"", "nyctaxi", prefix + "?catalog=main&schema=nyctaxi",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := databricks.BuildDSNFor(tc.loc, tc.catalog, tc.schema)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestBuildDSN_Errors(t *testing.T) {
	path := "/sql/1.0/warehouses/abc123"
	testCases := []struct {
		name    string
		loc     string
		wantErr string
	}{
		{"wrong_scheme", "postgres://token:" + testPAT + "@" + testHost + path, `must start with "databricks://"`},
		{"no_host", "databricks://token:" + testPAT + "@" + path, "no host"},
		{"no_http_path", "databricks://token:" + testPAT + "@" + testHost, "no HTTP path"},
		{"root_http_path", "databricks://token:" + testPAT + "@" + testHost + "/", "no HTTP path"},
		{
			"conflicting_http_path",
			"databricks://token:" + testPAT + "@" + testHost + path + "?http_path=/sql/1.0/warehouses/other",
			"two different HTTP paths",
		},
		{"username_not_token", "databricks://alice:" + testPAT + "@" + testHost + path, `username must be "token"`},
		{"username_only", "databricks://token@" + testHost + path, "no personal access token"},
		{"no_credentials", "databricks://" + testHost + path, "no credentials"},
		{"pat_auth_type_without_token", "databricks://" + testHost + path + "?authType=Pat", "no accessToken param"},
		{
			"m2m_without_secret",
			"databricks://" + testHost + path + "?authType=OauthM2M&clientID=id",
			"missing clientID or clientSecret",
		},
		{"client_id_without_secret", "databricks://" + testHost + path + "?clientID=id", "no credentials"},
		{"unknown_auth_type", "databricks://" + testHost + path + "?authType=Kerberos", "unknown authType"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := databricks.BuildDSN(tc.loc)
			require.Error(t, err)
			require.Empty(t, got)
			require.ErrorContains(t, err, tc.wantErr)
			require.NotContains(t, err.Error(), testPAT, "error must not leak the token")
		})
	}
}

// TestBuildDSN_ErrorDoesNotLeakSecret covers a location that url.Parse
// rejects: the *url.Error from the standard library echoes the input, which
// would include the token.
func TestBuildDSN_ErrorDoesNotLeakSecret(t *testing.T) {
	loc := "databricks://token:" + testPAT + "@" + testHost + ":notaport/sql/1.0/warehouses/abc123"
	_, err := databricks.BuildDSN(loc)
	require.Error(t, err)
	require.NotContains(t, err.Error(), testPAT)
}

func TestSecretsOfAndScrub(t *testing.T) {
	dsn := "https://token:" + testPAT + "@" + testHost + ":443/sql/1.0/warehouses/abc123?clientSecret=s3cr3t&catalog=main"
	secrets := databricks.SecretsOf(dsn)
	require.ElementsMatch(t, []string{testPAT, "s3cr3t"}, secrets)

	// An error whose message contains a secret is replaced by one that doesn't.
	perr := &url.Error{Op: "parse", URL: dsn, Err: url.EscapeError("%zz")}
	require.Contains(t, perr.Error(), testPAT)
	scrubbed := databricks.Scrub(perr, secrets)
	require.NotContains(t, scrubbed.Error(), testPAT)
	require.NotContains(t, scrubbed.Error(), "s3cr3t")
	require.Contains(t, scrubbed.Error(), "xxxxx")

	// An error without a secret is returned as is, so that its type survives.
	clean := &url.Error{Op: "Get", URL: "https://example.com", Err: url.EscapeError("%zz")}
	require.Same(t, clean, databricks.Scrub(clean, secrets))
	require.NoError(t, databricks.Scrub(nil, secrets))
}
