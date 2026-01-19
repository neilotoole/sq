# Architecture

This document provides high-level guidance on the key `sq` concepts.

This is effectively an ERD (Entity Relationship Diagram).

---

```mermaid
classDiagram
    namespace cli {
        class `config.Config` {
            +string Version
            +source.Collection Collection
            +options.Options Options
        }

        class `options.Options` {
            <<typedef>>
            map~string,any~
        }

        class `run.Run` {
            +config.Config Config
            +driver.Grips Grips
            +driver.Registry Registry
            +output.Writers Writers
            +source.Collection Collection
        }

        class `output.Writers` {
            +RecordWriter Record
            +MetadataWriter Metadata
            +ErrorWriter Error
        }
    }

    namespace source {
        class `source.Collection` {
            -[]*Source sources
            -string ActiveSrc
            -string ActiveGroup
            +Sources() []*Source
            +Add(*Source)
            +Get(handle) *Source
            +Active() *Source
        }

        class `source.Source` {
            +string Handle
            +drivertype.Type Type
            +string Location
            +string Catalog
            +string Schema
            +options.Options Options
        }

        class `drivertype.Type` {
            <<enum>>
            string
        }
    }

    namespace driver {
        class `driver.Registry` {
            -map~Type,Provider~ providers
            +AddProvider(Type, Provider)
            +DriverFor(Type) Driver
            +SQLDriverFor(Type) SQLDriver
        }

        class `driver.Provider` {
            <<interface>>
            +DriverFor(Type) Driver
        }

        class `driver.Driver` {
            <<interface>>
            +Open(ctx, *Source) Grip
            +Ping(ctx, *Source) error
            +DriverMetadata() Metadata
            +ValidateSource(*Source) *Source
        }

        class `driver.SQLDriver` {
            <<interface>>
            +Dialect() Dialect
            +Renderer() *Renderer
            +CurrentSchema(ctx, db) string
            +ListSchemas(ctx, db) []string
        }

        class `driver.Grips` {
            -map~string,Grip~ grips
            -Provider drvrs
            +Open(ctx, *Source) Grip
            +Close() error
        }

        class `driver.Grip` {
            <<interface>>
            +DB(ctx) *sql.DB
            +Source() *Source
            +SQLDriver() SQLDriver
            +Close() error
        }
    }

    namespace drivers {
        class `postgres.driveri` {
            <<SQLDriver>>
        }
        class `mysql.driveri` {
            <<SQLDriver>>
        }
        class `sqlite3.driveri` {
            <<SQLDriver>>
        }
        class `sqlserver.driveri` {
            <<SQLDriver>>
        }
        class `clickhouse.driveri` {
            <<SQLDriver>>
        }
        class `csv.driveri` {
            <<Driver>>
        }
        class `json.driveri` {
            <<Driver>>
        }
        class `xlsx.Driver` {
            <<Driver>>
        }
    }

    namespace libsq {
        class `libsq.QueryContext` {
            +source.Collection Collection
            +driver.Grips Grips
            +*ast.AST AST
        }

        class `ast.AST` {
            +*ast.Segment Root
            +Segments() []*Segment
            +Tables() []*TblSelector
        }

        class `render.Renderer` {
            +Render(ctx, *AST) string
            +dialect Dialect
        }

        class `libsq.RecordWriter` {
            <<interface>>
            +Open(ctx, cancelFn, Meta) chan Record
            +Wait() written, error
        }
    }

    namespace metadata {
        class `metadata.Source` {
            +string Handle
            +drivertype.Type Driver
            +[]*Table Tables
            +string DBVersion
        }

        class `metadata.Table` {
            +string Name
            +string TableType
            +[]*Column Columns
            +int64 RowCount
        }

        class `metadata.Column` {
            +string Name
            +string ColumnType
            +kind.Kind Kind
            +bool Nullable
        }
    }

    namespace record {
        class `record.Record` {
            <<typedef>>
            []any
        }

        class `record.Meta` {
            <<typedef>>
            []*FieldMeta
            +Names() []string
            +Kinds() []kind.Kind
        }

        class `record.FieldMeta` {
            -ColumnTypeData data
            -string mungedName
            +Name() string
            +Kind() kind.Kind
            +Nullable() bool
        }
    }

    %% Notes (must be outside namespace blocks)
    note for `config.Config` "Core app config containing
     version, options, source collection"
    note for `options.Options` "Key-value pairs for
     CLI and driver settings"
    note for `run.Run` "CLI execution context with
     all injectable resources"
    note for `output.Writers` "Container for format-specific
     output writers (json, table, csv, etc.)"
    note for `drivertype.Type` "Driver type enum,
     e.g. postgres, mysql, sqlite3,
     sqlserver, csv, xlsx, json"
    note for `libsq.QueryContext` "Encapsulates context for
     SLQ query execution"
    note for `ast.AST` "Root of parsed SLQ
     query syntax tree"
    note for `render.Renderer` "Renders AST to SQL,
     customizable per dialect"
    note for `libsq.RecordWriter` "Interface for async record
     output via channels"
    note for `metadata.Source` "Database-level metadata
     (name, driver, tables)"
    note for `metadata.Table` "Table metadata
     (name, columns, row count)"
    note for `metadata.Column` "Column metadata
     (name, type, nullable)"
    note for `record.Record` "A Record represents a row
     of data from a query result"
    note for `record.Meta` "Meta holds column metadata
     for the columns of a Record"
    note for `record.FieldMeta` "FieldMeta provides metadata
     about a result column"
    note for `postgres.driveri` "SQL drivers implement SQLDriver;
     document drivers implement Driver only"

    %% ===== RELATIONSHIPS =====
    %% Configuration relationships
    `config.Config` *-- `source.Collection` : contains
    `config.Config` *-- `options.Options` : contains

    %% CLI relationships
    `run.Run` *-- `config.Config` : contains
    `run.Run` *-- `driver.Grips` : contains
    `run.Run` *-- `driver.Registry` : contains
    `run.Run` *-- `output.Writers` : contains

    %% Source relationships
    `source.Collection` "1" *-- "*" `source.Source` : contains
    `source.Source` --> "1" `drivertype.Type` : has

    %% Driver relationships
    `driver.Registry` --> `drivertype.Type` : indexes by
    `driver.Registry` ..|> `driver.Provider` : implements
    `driver.Registry` --> `driver.Driver` : creates
    `driver.SQLDriver` --|> `driver.Driver` : extends
    `driver.SQLDriver` ..> `record.Meta` : returns via RecordMeta()
    `driver.SQLDriver` --> `render.Renderer` : uses
    `driver.Driver` ..> `source.Source` : receives
    `driver.Driver` ..> `driver.Grip` : returns
    `driver.Grips` --> `driver.Provider` : uses
    `driver.Grips` --o `driver.Grip` : caches
    `driver.Grip` ..> `source.Source` : references
    `driver.Grip` ..> `driver.SQLDriver` : references
    `driver.Grip` ..> `metadata.Source` : returns

    %% Driver implementation relationships
    `postgres.driveri` ..|> `driver.SQLDriver` : implements
    `mysql.driveri` ..|> `driver.SQLDriver` : implements
    `sqlite3.driveri` ..|> `driver.SQLDriver` : implements
    `sqlserver.driveri` ..|> `driver.SQLDriver` : implements
    `clickhouse.driveri` ..|> `driver.SQLDriver` : implements
    `csv.driveri` ..|> `driver.Driver` : implements
    `json.driveri` ..|> `driver.Driver` : implements
    `xlsx.Driver` ..|> `driver.Driver` : implements

    %% Query execution relationships
    `libsq.QueryContext` *-- `source.Collection` : contains
    `libsq.QueryContext` *-- `driver.Grips` : contains
    `libsq.QueryContext` --> `ast.AST` : uses
    `ast.AST` <.. `render.Renderer` : rendered by

    %% Metadata relationships
    `metadata.Source` "1" *-- "*" `metadata.Table` : contains
    `metadata.Table` "1" *-- "*" `metadata.Column` : contains

    %% Record relationships
    `record.Meta` "1" *-- "*" `record.FieldMeta` : contains
    `record.Meta` ..> `record.Record` : describes columns of

    %% Output relationships
    `libsq.RecordWriter` ..> `record.Record` : receives
    `libsq.RecordWriter` ..> `record.Meta` : uses
    `output.Writers` ..> `libsq.RecordWriter` : consumes
```
