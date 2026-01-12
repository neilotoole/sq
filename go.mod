module github.com/neilotoole/sq

// NOTE: Some of these deps are marked with "BRITTLE". That means that extra
// care needs to be taken when upgrading those versions, for various reasons.

go 1.25.5

// godebug x509negativeserial=1 is set here because of an issue with older
// SQL Server versions not doing the right thing with X509 certs (see RFC 5280).
// This has been an issue since Go 1.23 became stricter about certs.
// See:
// - https://pkg.go.dev/crypto/x509#ParseCertificate
//   Before Go 1.23, ParseCertificate accepted certificates with negative serial
//   numbers. This behavior can be restored by including "x509negativeserial=1"
//   in the GODEBUG environment variable.
// - https://github.com/burningalchemist/sql_exporter/issues/729
// - https://github.com/influxdata/telegraf/issues/16309#issuecomment-2612865201
godebug x509negativeserial=1

require (
	al.essio.dev/pkg/shellescape v1.6.0
	github.com/Masterminds/sprig/v3 v3.3.0
	github.com/a8m/tree v0.0.0-20240104212747-2c8764a5f17e
	github.com/antlr4-go/antlr/v4 v4.13.1
	github.com/c2h5oh/datasize v0.0.0-20231215233829-aa82cc1e6500
	github.com/djherbis/buffer v1.2.0
	github.com/dustin/go-humanize v1.0.1
	github.com/ecnepsnai/osquery v1.0.1
	github.com/emirpasic/gods v1.18.1
	github.com/fatih/color v1.18.0
	github.com/go-sql-driver/mysql v1.9.3 // BRITTLE
	github.com/goccy/go-yaml v1.19.1
	github.com/google/renameio/v2 v2.0.1
	github.com/google/uuid v1.6.0
	github.com/h2non/filetype v1.1.3
	github.com/itchyny/gojq v0.12.18
	github.com/jackc/pgx/v5 v5.8.0 // BRITTLE
	github.com/mattn/go-colorable v0.1.14
	github.com/mattn/go-runewidth v0.0.19
	github.com/mattn/go-sqlite3 v1.14.32 // BRITTLE
	github.com/microsoft/go-mssqldb v1.9.5 // BRITTLE
	github.com/mitchellh/go-wordwrap v1.0.1
	github.com/muesli/mango-cobra v1.3.0
	github.com/muesli/roff v0.1.0
	github.com/ncruces/go-strftime v1.0.0
	github.com/neilotoole/oncecache v0.0.1
	github.com/neilotoole/shelleditor v0.4.1
	github.com/neilotoole/slogt v1.1.0
	github.com/neilotoole/streamcache v0.3.5
	github.com/neilotoole/tailbuf v0.0.4
	github.com/nightlyone/lockfile v1.0.0
	github.com/otiai10/copy v1.14.1
	github.com/pkg/profile v1.7.0
	github.com/ryboe/q v1.0.25
	github.com/samber/lo v1.52.0
	github.com/segmentio/encoding v0.5.3
	github.com/sethvargo/go-retry v0.3.0
	github.com/shopspring/decimal v1.4.0
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/stretchr/testify v1.11.1
	github.com/vbauerster/mpb/v8 v8.11.3 // BRITTLE
	github.com/xo/dburl v0.24.2
	github.com/xuri/excelize/v2 v2.10.0 // BRITTLE
	go.uber.org/atomic v1.11.0
	golang.org/x/exp v0.0.0-20251219203646-944ab1f22d93
	golang.org/x/mod v0.31.0
	golang.org/x/sync v0.19.0
	golang.org/x/sys v0.39.0
	golang.org/x/term v0.38.0
	golang.org/x/text v0.32.0
)

require github.com/godror/godror v0.40.3

require (
	dario.cat/mergo v1.0.1 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.3.0 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/clipperhouse/stringish v0.1.1 // indirect
	github.com/clipperhouse/uax29/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/felixge/fgprof v0.9.5 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/godror/knownpb v0.1.1 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/google/pprof v0.0.0-20240227163752-401108e1b7e7 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/itchyny/timefmt-go v0.1.7 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/muesli/mango v0.2.0 // indirect
	github.com/muesli/mango-pflag v0.1.0 // indirect
	github.com/neilotoole/fifomu v0.1.2 // indirect
	github.com/otiai10/mint v1.6.3 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/richardlehane/mscfb v1.0.4 // indirect
	github.com/richardlehane/msoleps v1.0.4 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/segmentio/asm v1.2.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/tiendc/go-deepcopy v1.7.1 // indirect
	github.com/xuri/efp v0.0.1 // indirect
	github.com/xuri/nfp v0.0.2-0.20250530014748-2ddeb826f9a9 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
