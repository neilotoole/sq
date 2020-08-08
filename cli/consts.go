package cli

// cli flags
const (
	flagActiveSrc      = "src"
	flagActiveSrcUsage = "Override the active source for this query"

	flagCSV      = "csv"
	flagCSVShort = "c"
	flagCSVUsage = "CSV output"

	flagDriver      = "driver"
	flagDriverShort = "d"
	flagDriverUsage = "Explicitly specify the data source driver to use"

	flagHTML      = "html"
	flagHTMLUsage = "HTML table output"

	flagHeader      = "header"
	flagHeaderShort = "h"
	flagHeaderUsage = "Print header row in output"

	flagHandle      = "handle"
	flagHandleShort = "h"
	flagHandleUsage = "Handle for the source"

	flagHelp = "help"

	flagInsert      = "insert"
	flagInsertUsage = "Insert query results into @HANDLE.TABLE"

	flagInspectFull      = "full"
	flagInspectFullUsage = "Output full data source details (JSON only)"

	flagJSON       = "json"
	flagJSONUsage  = "JSON output"
	flagJSONShort  = "j"
	flagJSONA      = "jsona"
	flagJSONAShort = "A"
	flagJSONAUsage = "JSON: output each record's values as a JSON array on its own line"
	flagJSONL      = "jsonl"
	flagJSONLShort = "l"
	flagJSONLUsage = "JSON: output each record as a JSON object on its own line"

	flagMarkdown      = "markdown"
	flagMarkdownUsage = "Markdown table output"

	flagMonochrome      = "monochrome"
	flagMonochromeShort = "M"
	flagMonochromeUsage = "Don't colorize output"

	flagNoHeader      = "no-header"
	flagNoHeaderShort = "H"
	flagNoHeaderUsage = "Don't print header row in output"

	flagNotifierLabel      = "label"
	flagNotifierLabelUsage = "Optional label for the notification destination"

	flagOutput      = "output"
	flagOutputShort = "o"
	flagOutputUsage = "Write output to <file> instead of stdout"

	flagPretty      = "pretty"
	flagPrettyUsage = "Pretty-print output for certain formats such as JSON or XML"

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
	flagTSVUsage = "TSV output"

	flagTable      = "table"
	flagTableShort = "t"
	flagTableUsage = "Table output"

	flagTblData      = "data"
	flagTblDataUsage = "Copy table data (defualt true)"

	flagTimeout          = "timeout"
	flagTimeoutPingUsage = "Max time to wait for ping"

	flagVerbose      = "verbose"
	flagVerboseShort = "v"
	flagVerboseUsage = "Print verbose data, if applicable"

	flagVersion      = "version"
	flagVersionUsage = "Print sq version"

	flagXLSX      = "xlsx"
	flagXLSXShort = "x"
	flagXLSXUsage = "Excel XLSX output"

	flagXML      = "xml"
	flagXMLShort = "X"
	flagXMLUsage = "XML output"
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
