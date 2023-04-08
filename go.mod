module github.com/neilotoole/sq

go 1.20

require (
	github.com/antlr/antlr4/runtime/Go/antlr/v4 v4.0.0-20230305170008-8188dc5388df
	github.com/c2h5oh/datasize v0.0.0-20220606134207-859f65c6625b
	github.com/djherbis/fscache v0.10.1
	github.com/emirpasic/gods v1.18.1
	github.com/fatih/color v1.15.0
	github.com/go-sql-driver/mysql v1.7.0
	github.com/google/uuid v1.3.0
	github.com/h2non/filetype v1.1.3
	github.com/magefile/mage v1.9.0
	github.com/mattn/go-colorable v0.1.13
	github.com/mattn/go-isatty v0.0.18
	github.com/mattn/go-runewidth v0.0.14
	github.com/mattn/go-sqlite3 v1.14.16
	github.com/microsoft/go-mssqldb v0.19.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/pkg/errors v0.9.1 // indirect
	github.com/ryboe/q v1.0.18
	// Be very careful changing pkg segmentio/encoding. A specific version is by our json encoder.
	github.com/segmentio/encoding v0.1.14
	github.com/shopspring/decimal v1.3.1
	github.com/spf13/cobra v1.7.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.1
	github.com/tealeg/xlsx/v2 v2.0.1 // TODO: This package is no longer supported; switch to a different impl
	github.com/testcontainers/testcontainers-go v0.17.0
	github.com/xo/dburl v0.13.0
	go.uber.org/atomic v1.10.0
	go.uber.org/multierr v1.9.0
	golang.org/x/net v0.9.0
	golang.org/x/sync v0.1.0
	gopkg.in/yaml.v3 v3.0.1
)

// https://golang.testcontainers.org/quickstart/#2-install-testcontainers-for-go
replace github.com/docker/docker => github.com/docker/docker v20.10.3-0.20221013203545-33ab36d6b304+incompatible

require (
	github.com/jackc/pgconn v1.14.0
	github.com/jackc/pgx/v5 v5.3.1
	github.com/muesli/mango-cobra v1.2.0
	github.com/muesli/roff v0.1.0
	github.com/neilotoole/slogt v0.0.0-20230402033048-91bc251cef6e
	github.com/samber/lo v1.37.0
	golang.org/x/exp v0.0.0-20230321023759-10a507213a29
//golang.org/x/exp v0.0.0-20221012211006-4de253d81b95 // older version that had slog.FromContext
//golang.org/x/exp v0.0.0-20230321023759-10a507213a29
)

require (
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.2 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/klauspost/compress v1.11.13 // indirect
	github.com/moby/patternmatcher v0.5.0 // indirect
	github.com/moby/sys/sequential v0.5.0 // indirect
	github.com/muesli/mango v0.1.0 // indirect
	github.com/muesli/mango-pflag v0.1.0 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/rogpeppe/go-internal v1.9.0 // indirect
	golang.org/x/crypto v0.8.0 // indirect

)

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Microsoft/go-winio v0.6.0 // indirect
	github.com/cenkalti/backoff/v4 v4.2.0 // indirect
	github.com/containerd/containerd v1.6.14 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/docker v20.10.22+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/moby/term v0.0.0-20221205130635-1aeaba878587 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc2 // indirect
	github.com/opencontainers/runc v1.1.4 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sethvargo/go-retry v0.2.3
	github.com/sirupsen/logrus v1.9.0 // indirect
	golang.org/x/mod v0.9.0
	golang.org/x/sys v0.7.0 // indirect
	golang.org/x/term v0.7.0
	golang.org/x/text v0.9.0 // indirect
	golang.org/x/tools v0.6.0 // indirect
	google.golang.org/genproto v0.0.0-20221207170731-23e4bf6bdc37 // indirect
	google.golang.org/grpc v1.51.0 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/djherbis/atime.v1 v1.0.0 // indirect
	gopkg.in/djherbis/stream.v1 v1.3.1 // indirect
)
