module github.com/neilotoole/sq

go 1.16

// Using forked cobra for now because v1.1.3 does not pass Context
// to valid args completion funcs. There's an open PR for
// this: https://github.com/spf13/cobra/pull/1265
replace github.com/spf13/cobra v1.1.3 => github.com/neilotoole/cobra v1.1.4-0.20210220092732-c11dbd416310

require (
	github.com/antlr/antlr4 v0.0.0-20191011202612-ad2bd05285ca
	github.com/c2h5oh/datasize v0.0.0-20170519143321-54516c931ae9
	github.com/denisenkom/go-mssqldb v0.0.0-20200620013148-b91950f658ec
	github.com/djherbis/fscache v0.10.1
	github.com/emirpasic/gods v1.9.0
	github.com/fatih/color v1.9.0
	github.com/go-sql-driver/mysql v1.5.0
	github.com/google/uuid v1.1.1
	github.com/h2non/filetype v1.1.0
	github.com/jackc/pgconn v1.5.0
	github.com/jackc/pgx/v4 v4.6.0
	github.com/kr/text v0.2.0 // indirect
	github.com/magefile/mage v1.9.0
	github.com/mattn/go-colorable v0.1.4
	github.com/mattn/go-isatty v0.0.12
	github.com/mattn/go-runewidth v0.0.4
	github.com/mattn/go-sqlite3 v1.14.6
	github.com/mitchellh/go-homedir v1.1.0
	github.com/neilotoole/errgroup v0.1.5
	github.com/neilotoole/lg v0.3.0
	github.com/pkg/errors v0.9.1
	github.com/ryboe/q v1.0.12
	github.com/satori/go.uuid v1.2.0
	github.com/segmentio/encoding v0.1.14
	github.com/shopspring/decimal v0.0.0-20180709203117-cd690d0c9e24
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.com/tealeg/xlsx/v2 v2.0.1
	github.com/testcontainers/testcontainers-go v0.5.0
	github.com/twpayne/go-geom v1.3.6
	github.com/xo/dburl v0.0.0-20200124232849-e9ec94f52bc3
	go.uber.org/atomic v1.5.0
	go.uber.org/multierr v1.4.0
	golang.org/x/crypto v0.0.0-20200728195943-123391ffb6de
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	gopkg.in/djherbis/atime.v1 v1.0.0 // indirect
	gopkg.in/djherbis/stream.v1 v1.3.1 // indirect
	gopkg.in/yaml.v2 v2.4.0
)
