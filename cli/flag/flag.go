// Package flag holds CLI flags.
package flag

const (
	ActiveSrc      = "src"
	ActiveSrcUsage = "Override the active source for this query"

	ConfigSrc      = "src"
	ConfigSrcUsage = "Config for source"

	CSV      = "csv"
	CSVShort = "C"
	CSVUsage = "Output CSV"

	AddDriver      = "driver"
	AddDriverShort = "d"
	AddDriverUsage = "Explicitly specify the driver to use"

	IngestDriver      = "ingest.driver"
	IngestDriverUsage = "Explicitly specify the driver to use for ingesting data"

	HTML      = "html"
	HTMLUsage = "Output HTML table"

	Header      = "header"
	HeaderShort = "h"
	HeaderUsage = "Print header row"

	NoHeader      = "no-header"
	NoHeaderShort = "H"
	NoHeaderUsage = "Don't print header row"

	Handle      = "handle"
	HandleShort = "n"
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

	Markdown      = "markdown"
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

	Compact      = "compact"
	CompactShort = "c"
	CompactUsage = "Compact instead of pretty-printed output"

	Raw      = "raw"
	RawShort = "r"
	RawUsage = "Output each record field in raw format without any encoding or delimiter"

	SQLExec      = "exec"
	SQLExecUsage = "Execute the SQL as a statement (as opposed to query)"

	SQLQuery      = "query"
	SQLQueryUsage = "Execute the SQL as a query (as opposed to statement)"

	TSV      = "tsv"
	TSVShort = "T"
	TSVUsage = "Output TSV"

	Text      = "text"
	TextShort = "t"
	TextUsage = "Output text"

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

	YAML      = "yaml"
	YAMLShort = "y"
	YAMLUsage = "Output YAML"

	XML      = "xml"
	XMLShort = "X"
	XMLUsage = "Output XML"

	SkipVerify      = "skip-verify"
	SkipVerifyUsage = "Don't ping source before adding it"

	Arg      = "arg"
	ArgUsage = "Set a string value to a variable"

	Config      = "config"
	ConfigUsage = "Load config from here"

	IngestHeader      = "ingest.header"
	IngestHeaderUsage = "Treat first row of ingest data as header"

	CSVEmptyAsNull      = "driver.csv.empty-as-null"
	CSVEmptyAsNullUsage = "Treat empty CSV fields as null"

	CSVDelim        = "driver.csv.delim"
	CSVDelimUsage   = "CSV delimiter: one of comma, space, pipe, tab, colon, semi, period"
	CSVDelimDefault = "comma"

	ConfigDelete      = "delete"
	ConfigDeleteShort = "D"
	ConfigDeleteUsage = "Reset this option to default value"

	LogEnabled      = "log"
	LogEnabledUsage = "Enable logging"

	LogFile      = "log.file"
	LogFileUsage = "Path to log file; empty disables logging"

	LogLevel      = "log.level"
	LogLevelUsage = "Log level: one of DEBUG, INFO, WARN, ERROR"

	DiffSummary      = "summary"
	DiffSummaryUsage = "Show summary-level diff only"

	DiffNumLines      = "lines"
	DiffNumLinesShort = "n"
	DiffNumLinesUsage = "Number of lines surrounding diff"
)
