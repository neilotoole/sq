-- type_test.ddl exercises every DuckDB type that the driver maps to a kind.Kind.
DROP TABLE IF EXISTS type_test;

CREATE TABLE type_test (
    col_bool       BOOLEAN,
    col_tinyint    TINYINT,
    col_smallint   SMALLINT,
    col_int        INTEGER,
    col_bigint     BIGINT,
    col_hugeint    HUGEINT,
    col_utinyint   UTINYINT,
    col_usmallint  USMALLINT,
    col_uint       UINTEGER,
    col_ubigint    UBIGINT,
    col_float      FLOAT,
    col_double     DOUBLE,
    col_decimal    DECIMAL(18, 4),
    col_varchar    VARCHAR,
    col_blob       BLOB,
    col_date       DATE,
    col_time       TIME,
    col_timestamp  TIMESTAMP,
    col_timestamptz TIMESTAMPTZ,
    col_interval   INTERVAL,
    col_uuid       UUID,
    col_json       JSON,
    col_list       INTEGER[],
    col_struct     STRUCT(a INTEGER, b VARCHAR),
    col_map        MAP(VARCHAR, INTEGER),
    col_enum       VARCHAR  -- ENUM is created separately
);

INSERT INTO type_test VALUES (
    TRUE, 1, 2, 3, 4, 99999999999999999999::HUGEINT,
    1, 2, 3, 4,
    1.5, 2.5, 3.1415,
    'hello', '\x01\x02'::BLOB,
    DATE '2026-05-11', TIME '12:34:56', TIMESTAMP '2026-05-11 12:34:56', TIMESTAMPTZ '2026-05-11 12:34:56+00',
    INTERVAL 1 DAY,
    '12345678-1234-5678-1234-567812345678'::UUID,
    '{"a":1}',
    [10, 20, 30],
    {a: 1, b: 'x'},
    MAP { 'k1': 1, 'k2': 2 },
    'green'
);
