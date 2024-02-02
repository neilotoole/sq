// Package flag holds CLI flags.
package flag

const (
	ActiveSrc      = "src"
	ActiveSrcUsage = "Override active source for this query"

	ActiveSchema      = "src.schema"
	ActiveSchemaUsage = "Override active schema or catalog.schema for this query"

	ConfigSrc      = "src"
	ConfigSrcUsage = "Config for source"

	CSV      = "csv"
	CSVShort = "C"
	CSVUsage = "Output CSV"

	AddDriver      = "driver"
	AddDriverShort = "d"
	AddDriverUsage = "Explicitly specify driver to use"

	IngestDriver      = "ingest.driver"
	IngestDriverUsage = "Explicitly specify driver to use for ingesting data"

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
	InsertUsage = "Insert query results into @HANDLE.TABLE; if not existing, TABLE will be created"

	JSON       = "json"
	JSONShort  = "j"
	JSONUsage  = "Output JSON"
	JSONA      = "jsona"
	JSONAShort = "A"
	JSONAUsage = "Output LF-delimited JSON arrays"
	JSONL      = "jsonl"
	JSONLShort = "J"
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

	// Input sets Run.Stdin to the named file. At this time, this is used
	// mainly for debugging, so it's marked hidden by the CLI. I'm not
	// sure if this will ever be generally useful. Also, there's been no
	// testing done to see how this flag would interact with, say,
	// flag.PasswordPrompt, which also reads from stdin.
	Input      = "input"
	InputShort = "i"
	InputUsage = "Read input from <file> instead of stdin"

	InspectOverview      = "overview"
	InspectOverviewShort = "O"
	InspectOverviewUsage = "Show metadata only (no schema)"

	PasswordPrompt      = "password"
	PasswordPromptShort = "p"
	PasswordPromptUsage = "Read password from stdin or prompt"

	CacheTreeSize      = "size"
	CacheTreeSizeShort = "s"
	CacheTreeSizeUsage = "Show sizes in cache tree"

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

	InspectDBProps      = "dbprops"
	InspectDBPropsShort = "p"
	InspectDBPropsUsage = "Show DB properties only"

	InspectCatalogs      = "catalogs"
	InspectCatalogsShort = "C"
	InspectCatalogsUsage = "List catalogs only"

	InspectSchemata      = "schemata"
	InspectSchemataShort = "S"
	InspectSchemataUsage = "List schemas (in current catalog) only"

	LogEnabled      = "log"
	LogEnabledUsage = "Enable logging"

	LogFile      = "log.file"
	LogFileUsage = "Path to log file; empty disables logging"

	LogLevel      = "log.level"
	LogLevelUsage = "Log level: one of DEBUG, INFO, WARN, ERROR"

	LogFormat      = "log.format"
	LogFormatUsage = `Log format: one of "text" or "json"`

	DiffOverview      = "overview"
	DiffOverviewShort = "O"
	DiffOverviewUsage = "Compare source overview"

	DiffSchema      = "schema"
	DiffSchemaShort = "S"
	DiffSchemaUsage = "Compare schema structure"

	DiffDBProps      = "dbprops"
	DiffDBPropsShort = "B"
	DiffDBPropsUsage = "Compare DB properties"

	DiffRowCount      = "counts"
	DiffRowCountShort = "N"
	DiffRowCountUsage = "When comparing table schema structure, include row counts"

	DiffData      = "data"
	DiffDataShort = "d"
	DiffDataUsage = "Compare values of each data row (caution: may be slow)"

	DiffAll      = "all"
	DiffAllShort = "a"
	DiffAllUsage = "Compare everything (caution: may be slow)"

	DumpCatalog      = "catalog"
	DumpCatalogUsage = "Dump the named catalog"
	DumpNoOwner      = "no-owner"
	DumpNoOwnerUsage = "Don't set ownership or ACL"
	DumpFile         = "file"
	DumpFileShort    = "f"
	DumpFileUsage    = "Write dump to file; if omitted, write to stdout"

	PrintToolCmd          = "cmd"
	PrintToolCmdUsage     = "Print the db-native tool command, but don't execute it"
	PrintLongToolCmd      = "cmd-long"
	PrintLongToolCmdUsage = "Print the db-native tool command in long form, but don't execute it"

	RestoreFrom         = "from"
	RestoreFromShort    = "f"
	RestoreFromUsage    = "Restore from dump file; if omitted, read from stdin"
	RestoreNoOwner      = "no-owner"
	RestoreNoOwnerUsage = "Don't use ownership or ACL from dump"
)

// OutputFormatFlags is the set of flags that control output format.
var OutputFormatFlags = []string{
	Text,
	JSON,
	JSONA,
	JSONL,
	CSV,
	TSV,
	HTML,
	Markdown,
	Raw,
	XLSX,
	XML,
	YAML,
}
