module github.com/neilotoole/sq

go 1.20

require (
	github.com/antlr/antlr4/runtime/Go/antlr/v4 v4.0.0-20230305170008-8188dc5388df
	github.com/c2h5oh/datasize v0.0.0-20220606134207-859f65c6625b
	github.com/djherbis/fscache v0.10.1
	github.com/emirpasic/gods v1.18.1
	github.com/fatih/color v1.15.0
	github.com/go-sql-driver/mysql v1.7.1
	github.com/google/uuid v1.3.0
	github.com/h2non/filetype v1.1.3
	github.com/mattn/go-colorable v0.1.13
	github.com/mattn/go-isatty v0.0.19
	github.com/mattn/go-runewidth v0.0.14
	github.com/mattn/go-sqlite3 v1.14.17
	github.com/microsoft/go-mssqldb v1.1.0
	github.com/ryboe/q v1.0.19
	// Be very careful changing pkg segmentio/encoding. A specific version is by our json encoder.
	github.com/segmentio/encoding v0.1.14
	github.com/spf13/cobra v1.7.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.4
	github.com/tealeg/xlsx/v2 v2.0.1 // TODO: This package is no longer supported; switch to a different impl
	github.com/xo/dburl v0.14.2
	go.uber.org/atomic v1.11.0
	go.uber.org/multierr v1.11.0
	golang.org/x/net v0.11.0
	golang.org/x/sync v0.3.0
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

require (
	github.com/ecnepsnai/osquery v1.0.0
	github.com/goccy/go-yaml v1.11.0
	github.com/jackc/pgx/v5 v5.4.1
	github.com/mitchellh/go-wordwrap v1.0.1
	github.com/muesli/mango-cobra v1.2.0
	github.com/muesli/roff v0.1.0
	github.com/ncruces/go-strftime v0.1.9
	github.com/neilotoole/shelleditor v0.3.2
	github.com/neilotoole/slogt v1.0.0
	github.com/otiai10/copy v1.12.0
	github.com/samber/lo v1.38.1
	golang.org/x/exp v0.0.0-20230420155640-133eef4313cb
)

require (
	github.com/frankban/quicktest v1.11.3 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/puddle/v2 v2.2.0 // indirect
	github.com/muesli/mango v0.1.0 // indirect
	github.com/muesli/mango-pflag v0.1.0 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	golang.org/x/crypto v0.10.0 // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect

)

require (
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/moby/term v0.0.0-20221205130635-1aeaba878587 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sethvargo/go-retry v0.2.4
	golang.org/x/mod v0.10.0
	golang.org/x/sys v0.9.0 // indirect
	golang.org/x/term v0.9.0
	golang.org/x/text v0.10.0 // indirect
	gopkg.in/djherbis/atime.v1 v1.0.0 // indirect
	gopkg.in/djherbis/stream.v1 v1.3.1 // indirect
)
