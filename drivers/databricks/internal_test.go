package databricks

import "github.com/neilotoole/sq/libsq/core/tablefq"

// BuildDSN is buildDSN without a catalog or schema override.
func BuildDSN(loc string) (string, error) { return buildDSN(loc, "", "") }

// Exported variables for testing unexported functions from the external
// test package (package databricks_test). The naming convention is to
// capitalize the first letter of the unexported name.

var (
	BuildDSNFor         = buildDSN
	KindFromDBTypeName  = kindFromDBTypeName
	ParseSemver         = parseSemver
	QuoteString         = quoteString
	TableTypeFromDBType = tableTypeFromDBType
	Scrub               = scrub
	SecretsOf           = secretsOf
	IsNotExist          = isNotExist
	RequireInfoSchema   = requireInfoSchema
	Placeholders        = placeholders
	RenderRange         = renderRange
	Tblfmt              = tblfmt[string]
	TblfmtFQ            = tblfmt[tablefq.T]
)
