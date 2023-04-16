// Package flag holds CLI flags.
package flag

const (
	ActiveSrc      = "src"
	ActiveSrcUsage = "Override the active source for this query"

	CSV      = "csv"
	CSVShort = "c"
	CSVUsage = "Output CSV"

	Driver      = "driver"
	DriverShort = "d"
	DriverUsage = "Explicitly specify the data source driver to use"

	HTML      = "html"
	HTMLUsage = "Output HTML table"

	Header      = "header"
	HeaderShort = "h"
	HeaderUsage = "Print header row in output (default true)"

	Handle      = "handle"
	HandleShort = "h"
	HandleUsage = "Handle for the source"

	ListGroup      = "group"
	ListGroupShort = "g"
	ListGroupUsage = "List groups instead of sources"

	Help = "help"

	Insert      = "insert"
	InsertUsage = "Insert query results into @HANDLE.TABLE. If not existing, TABLE will be created."

	JSON       = "json"
	JSONShort  = "j"
	JSONUsage  = "Output JSON"
	JSONA      = "jsona"
	JSONAShort = "A"
	JSONAUsage = "Output LF-delimited JSON arrays"
	JSONL      = "jsonl"
	JSONLShort = "l"
	JSONLUsage = "Output LF-delimited JSON objects"

	Markdown      = "md"
	MarkdownUsage = "Output Markdown"

	AddActive      = "active"
	AddActiveShort = "a"
	AddActiveUsage = "Make this the active source"

	Monochrome      = "monochrome"
	MonochromeShort = "M"
	MonochromeUsage = "Don't colorize output"

	Output      = "output"
	OutputShort = "o"
	OutputUsage = "Write output to <file> instead of stdout"

	PasswordPrompt      = "password"
	PasswordPromptShort = "p"
	PasswordPromptUsage = "Read password from stdin or prompt"

	Pretty      = "pretty"
	PrettyUsage = "Pretty-print output"

	QueryDriverUsage     = "Explicitly specify the data source driver to use when piping input"
	QuerySrcOptionsUsage = "Driver-dependent data source options when piping input"

	Raw      = "raw"
	RawShort = "r"
	RawUsage = "Output each record field in raw format without any encoding or delimiter"

	SQLExec      = "exec"
	SQLExecUsage = "Execute the SQL as a statement (as opposed to query)"

	SQLQuery      = "query"
	SQLQueryUsage = "Execute the SQL as a query (as opposed to statement)"

	SrcOptions      = "opts"
	SrcOptionsUsage = "Driver-dependent data source options"

	TSV      = "tsv"
	TSVShort = "T"
	TSVUsage = "Output TSV"

	Table = "table" // TODO: Rename "table" to "text" (output is not always a table).

	TableShort = "t"
	TableUsage = "Output text table"

	TblData      = "data"
	TblDataUsage = "Copy table data"

	PingTimeout      = "timeout"
	PingTimeoutUsage = "Max time to wait for ping"

	Verbose      = "verbose"
	VerboseShort = "v"
	VerboseUsage = "Verbose output"

	Version      = "version"
	VersionUsage = "Print version info"

	XLSX      = "xlsx"
	XLSXShort = "x"
	XLSXUsage = "Output Excel XLSX"

	XML      = "xml"
	XMLShort = "X"
	XMLUsage = "Output XML"

	SkipVerify      = "skip-verify"
	SkipVerifyUsage = "Don't ping source before adding it"

	Arg      = "arg"
	ArgUsage = "Set a string value to a variable"

	ConfigDir      = "config.dir"
	ConfigDirUsage = "Use config dir"
)
