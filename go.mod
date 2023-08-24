module github.com/neilotoole/sq

go 1.21

require (
	github.com/Masterminds/sprig/v3 v3.2.3
	github.com/alessio/shellescape v1.4.2
	github.com/antlr4-go/antlr/v4 v4.13.0
	github.com/c2h5oh/datasize v0.0.0-20220606134207-859f65c6625b
	github.com/djherbis/fscache v0.10.1
	github.com/ecnepsnai/osquery v1.0.1
	github.com/emirpasic/gods v1.18.1
	github.com/fatih/color v1.15.0
	github.com/go-sql-driver/mysql v1.7.1
	github.com/goccy/go-yaml v1.11.0
	github.com/google/uuid v1.3.1
	github.com/h2non/filetype v1.1.3
	github.com/jackc/pgx/v5 v5.4.3
	github.com/mattn/go-colorable v0.1.13
	github.com/mattn/go-isatty v0.0.19
	github.com/mattn/go-runewidth v0.0.15
	github.com/mattn/go-sqlite3 v1.14.17
	github.com/microsoft/go-mssqldb v1.5.0
	github.com/mitchellh/go-wordwrap v1.0.1
	github.com/muesli/mango-cobra v1.2.0
	github.com/muesli/roff v0.1.0
	github.com/ncruces/go-strftime v0.1.9
	github.com/neilotoole/shelleditor v0.3.2
	github.com/neilotoole/slogt v1.1.0
	github.com/otiai10/copy v1.12.0
	github.com/ryboe/q v1.0.19
	github.com/samber/lo v1.38.1
	github.com/segmentio/encoding v0.1.14 // Be very careful changing pkg segmentio/encoding. A specific version is used by our json encoder.
	github.com/sethvargo/go-retry v0.2.4
	github.com/spf13/cobra v1.7.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.4
	github.com/xo/dburl v0.16.0

	// At the time of writing (2023-08-13), there was an issue with the latest
	// release of excelize, so we're using master. On the next release we
	// hopefully can change back to a tagged release.
	// See: https://github.com/qax-os/excelize/issues/660
	github.com/xuri/excelize/v2 v2.7.2-0.20230808161106-ae17fa87d506
	go.uber.org/atomic v1.11.0
	go.uber.org/multierr v1.11.0
	golang.org/x/exp v0.0.0-20230515195305-f3d0a9c9a5cc
	golang.org/x/mod v0.12.0
	golang.org/x/net v0.14.0
	golang.org/x/sync v0.3.0
	golang.org/x/term v0.11.0
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.2.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/huandu/xstrings v1.4.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/term v0.0.0-20221205130635-1aeaba878587 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/muesli/mango v0.1.0 // indirect
	github.com/muesli/mango-pflag v0.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/richardlehane/mscfb v1.0.4 // indirect
	github.com/richardlehane/msoleps v1.0.3 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/spf13/cast v1.5.1 // indirect
	github.com/xuri/efp v0.0.0-20230802181842-ad255f2331ca // indirect
	github.com/xuri/nfp v0.0.0-20230802015359-2d5eeba905e9 // indirect
	golang.org/x/crypto v0.12.0 // indirect
	golang.org/x/sys v0.11.0 // indirect
	golang.org/x/text v0.12.0 // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	gopkg.in/djherbis/atime.v1 v1.0.0 // indirect
	gopkg.in/djherbis/stream.v1 v1.3.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
