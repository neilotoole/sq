module github.com/neilotoole/sq

go 1.19

// Using forked cobra for now because v1.1.3 does not pass Context
// to valid args completion funcs. There's an open PR for
// this: https://github.com/spf13/cobra/pull/1265
replace github.com/spf13/cobra v1.1.3 => github.com/neilotoole/cobra v1.1.4-0.20210220092732-c11dbd416310

require (
	github.com/alexflint/go-filemutex v1.2.0
	github.com/antlr/antlr4 v0.0.0-20191011202612-ad2bd05285ca
	github.com/c2h5oh/datasize v0.0.0-20170519143321-54516c931ae9
	github.com/denisenkom/go-mssqldb v0.0.0-20200620013148-b91950f658ec
	github.com/djherbis/fscache v0.10.1
	github.com/emirpasic/gods v1.9.0
	github.com/fatih/color v1.13.0
	github.com/go-sql-driver/mysql v1.7.0
	github.com/google/uuid v1.3.0
	github.com/h2non/filetype v1.1.0
	github.com/jackc/pgconn v1.5.0
	github.com/jackc/pgx/v4 v4.6.0
	github.com/magefile/mage v1.9.0
	github.com/mattn/go-colorable v0.1.12
	github.com/mattn/go-isatty v0.0.14
	github.com/mattn/go-runewidth v0.0.4
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
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
	github.com/stretchr/testify v1.8.1
	github.com/tealeg/xlsx/v2 v2.0.1
	github.com/testcontainers/testcontainers-go v0.15.0
	github.com/xo/dburl v0.11.0
	go.uber.org/atomic v1.5.0
	go.uber.org/multierr v1.4.0
	golang.org/x/crypto v0.0.0-20220511200225-c6db032c6c88
	golang.org/x/net v0.2.0
	golang.org/x/sync v0.1.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/Microsoft/go-winio v0.6.0 // indirect
	github.com/Microsoft/hcsshim v0.9.5 // indirect
	github.com/cenkalti/backoff/v4 v4.2.0 // indirect
	github.com/containerd/cgroups v1.0.4 // indirect
	github.com/containerd/containerd v1.6.10 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/docker v20.10.21+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-sql/civil v0.0.0-20190719163853-cb61b32ac6fe // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.0.1 // indirect
	github.com/jackc/pgservicefile v0.0.0-20200307190119-3430c5407db8 // indirect
	github.com/jackc/pgtype v1.3.0 // indirect
	github.com/kr/pretty v0.2.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/magiconair/properties v1.8.6 // indirect
	github.com/moby/sys/mount v0.3.3 // indirect
	github.com/moby/sys/mountinfo v0.6.2 // indirect
	github.com/moby/term v0.0.0-20221105221325-4eb28fa6025c // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc2 // indirect
	github.com/opencontainers/runc v1.1.4 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sethvargo/go-retry v0.2.3
	github.com/sirupsen/logrus v1.9.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.uber.org/tools v0.0.0-20190618225709-2cfd321de3ee // indirect
	go.uber.org/zap v1.13.0 // indirect
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/mod v0.7.0 // indirect
	golang.org/x/sys v0.2.0 // indirect
	golang.org/x/term v0.2.0 // indirect
	golang.org/x/text v0.4.0 // indirect
	golang.org/x/tools v0.3.0 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/genproto v0.0.0-20221118155620-16455021b5e6 // indirect
	google.golang.org/grpc v1.51.0 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/djherbis/atime.v1 v1.0.0 // indirect
	gopkg.in/djherbis/stream.v1 v1.3.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	honnef.co/go/tools v0.0.1-2020.1.3 // indirect
)
