package cli

// cli flags.
const (
	flagActiveSrc      = "src"
	flagActiveSrcUsage = "Override the active source for this query"

	flagCSV      = "csv"
	flagCSVShort = "c"
	flagCSVUsage = "Output CSV"

	flagDriver      = "driver"
	flagDriverShort = "d"
	flagDriverUsage = "Explicitly specify the data source driver to use"

	flagHTML      = "html"
	flagHTMLUsage = "Output HTML table"

	flagHeader      = "header"
	flagHeaderShort = "h"
	flagHeaderUsage = "Print header row in output (default true)"

	flagHandle      = "handle"
	flagHandleShort = "h"
	flagHandleUsage = "Handle for the source"

	flagHelp = "help"

	flagInsert      = "insert"
	flagInsertUsage = "Insert query results into @HANDLE.TABLE. If not existing, TABLE will be created."

	flagJSON       = "json"
	flagJSONUsage  = "Output JSON"
	flagJSONShort  = "j"
	flagJSONA      = "jsona"
	flagJSONAShort = "A"
	flagJSONAUsage = "Output LF-delimited JSON arrays"
	flagJSONL      = "jsonl"
	flagJSONLShort = "l"
	flagJSONLUsage = "Output LF-delimited JSON objects"

	flagMarkdown      = "markdown"
	flagMarkdownUsage = "Output Markdown"

	flagAddActive      = "active"
	flagAddActiveShort = "a"
	flagAddActiveUsage = "Make this the active source"

	flagMonochrome      = "monochrome"
	flagMonochromeShort = "M"
	flagMonochromeUsage = "Don't colorize output"

	flagOutput      = "output"
	flagOutputShort = "o"
	flagOutputUsage = "Write output to <file> instead of stdout"

	flagPasswordPrompt      = "password"
	flagPasswordPromptShort = "p"
	flagPasswordPromptUsage = "Read password from stdin or prompt"

	flagPretty      = "pretty"
	flagPrettyUsage = "Pretty-print output"

	flagQueryDriverUsage     = "Explicitly specify the data source driver to use when piping input"
	flagQuerySrcOptionsUsage = "Driver-dependent data source options when piping input"

	flagRaw      = "raw"
	flagRawShort = "r"
	flagRawUsage = "Output each record field in raw format without any encoding or delimiter"

	flagSQLExec      = "exec"
	flagSQLExecUsage = "Execute the SQL as a statement (as opposed to query)"

	flagSQLQuery      = "query"
	flagSQLQueryUsage = "Execute the SQL as a query (as opposed to statement)"

	flagSrcOptions      = "opts"
	flagSrcOptionsUsage = "Driver-dependent data source options"

	flagTSV      = "tsv"
	flagTSVShort = "T"
	flagTSVUsage = "Output TSV"

	flagTable      = "table"
	flagTableShort = "t"
	flagTableUsage = "Output text table"

	flagTblData      = "data"
	flagTblDataUsage = "Copy table data"

	flagPingTimeout      = "timeout"
	flagPingTimeoutUsage = "Max time to wait for ping"

	flagPingAll      = "all"
	flagPingAllShort = "a"
	flagPingAllUsage = "Ping all sources"

	flagVerbose      = "verbose"
	flagVerboseShort = "v"
	flagVerboseUsage = "Print verbose output, if applicable"

	flagVersion      = "version"
	flagVersionUsage = "Print sq version"

	flagXLSX      = "xlsx"
	flagXLSXShort = "x"
	flagXLSXUsage = "Output Excel XLSX"

	flagXML      = "xml"
	flagXMLShort = "X"
	flagXMLUsage = "Output XML"

	flagSkipVerify      = "skip-verify"
	flagSkipVerifyUsage = "Don't ping source before adding it"

	flagArg      = "arg"
	flagArgUsage = "Set a string value to a variable"
)

const (
	msgInvalidArgs       = "invalid args"
	msgNoActiveSrc       = "no active data source"
	msgEmptyQueryString  = "query string is empty"
	msgSrcNoData         = "source has no data"
	msgSrcEmptyTableName = "source has empty table name"

	envarLogPath     = "SQ_LOGFILE"
	envarLogTruncate = "SQ_LOGFILE_TRUNCATE"
	envarConfigDir   = "SQ_CONFIGDIR"
)
