module github.com/neilotoole/sq

go 1.21

// See: https://github.com/djherbis/fscache/pull/21
require github.com/neilotoole/fscache v0.0.0-20231203162946-c9808f16552e

// See: https://github.com/djherbis/stream/pull/11
replace github.com/djherbis/stream v1.4.0 => github.com/neilotoole/djherbis-stream v0.0.0-20231203160853-609f47afedda

require (
	github.com/Masterminds/sprig/v3 v3.2.3
	github.com/a8m/tree v0.0.0-20240104212747-2c8764a5f17e
	github.com/alessio/shellescape v1.4.2
	github.com/antlr4-go/antlr/v4 v4.13.0
	github.com/c2h5oh/datasize v0.0.0-20231215233829-aa82cc1e6500
	github.com/dustin/go-humanize v1.0.1
	github.com/ecnepsnai/osquery v1.0.1
	github.com/emirpasic/gods v1.18.1
	github.com/fatih/color v1.16.0
	github.com/go-sql-driver/mysql v1.7.1
	github.com/goccy/go-yaml v1.11.2
	github.com/google/uuid v1.5.0
	github.com/h2non/filetype v1.1.3
	github.com/jackc/pgx/v5 v5.5.2
	github.com/mattn/go-colorable v0.1.13
	github.com/mattn/go-runewidth v0.0.15
	github.com/mattn/go-sqlite3 v1.14.19
	github.com/microsoft/go-mssqldb v1.6.0
	github.com/mitchellh/go-wordwrap v1.0.1
	github.com/muesli/mango-cobra v1.2.0
	github.com/muesli/roff v0.1.0
	github.com/ncruces/go-strftime v0.1.9
	github.com/neilotoole/shelleditor v0.4.1
	github.com/neilotoole/slogt v1.1.0
	github.com/nightlyone/lockfile v1.0.0
	github.com/otiai10/copy v1.14.0
	github.com/ryboe/q v1.0.20
	github.com/samber/lo v1.39.0
	github.com/segmentio/encoding v0.4.0
	github.com/sethvargo/go-retry v0.2.4
	github.com/shopspring/decimal v1.3.1
	github.com/spf13/cobra v1.8.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.4
	github.com/vbauerster/mpb/v8 v8.7.2
	github.com/xo/dburl v0.20.2
	github.com/xuri/excelize/v2 v2.8.0
	go.uber.org/atomic v1.11.0
	golang.org/x/exp v0.0.0-20240112132812-db7319d0e0e3
	golang.org/x/mod v0.14.0
	golang.org/x/sync v0.6.0
	golang.org/x/sys v0.16.0
	golang.org/x/term v0.16.0
	golang.org/x/text v0.14.0
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.2.1 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/djherbis/atime v1.1.0 // indirect
	github.com/djherbis/stream v1.4.0 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/huandu/xstrings v1.4.0 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20231201235250-de7065d80cb9 // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/muesli/mango v0.2.0 // indirect
	github.com/muesli/mango-pflag v0.1.0 // indirect
	github.com/neilotoole/fifomu v0.1.1 // indirect
	github.com/neilotoole/streamcache v0.2.1-0.20240121154007-e53a3bae28e7 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/richardlehane/mscfb v1.0.4 // indirect
	github.com/richardlehane/msoleps v1.0.3 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/rogpeppe/go-internal v1.12.0 // indirect
	github.com/segmentio/asm v1.2.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/xuri/efp v0.0.0-20231025114914-d1ff6096ae53 // indirect
	github.com/xuri/nfp v0.0.0-20230919160717-d98342af3f05 // indirect
	golang.org/x/crypto v0.18.0 // indirect
	golang.org/x/net v0.20.0 // indirect
	golang.org/x/xerrors v0.0.0-20231012003039-104605ab7028 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
