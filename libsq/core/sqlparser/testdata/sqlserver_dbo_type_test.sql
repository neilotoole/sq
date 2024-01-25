create table type_test
(
    col_id              int identity
        constraint type_test_pk
            primary key nonclustered,
    col_int             int           not null,
    col_int_n           int,
    col_bool            bit           not null,
    col_bool_n          bit,
    col_decimal         decimal(18)   not null,
    col_decimal_n       decimal(18),
    col_float           float         not null,
    col_float_n         float,
    col_date            date          not null,
    col_date_n          date,
    col_datetime        datetime      not null,
    col_datetime_n      datetime,
    col_datetime2       datetime2     not null,
    col_datetime2_n     datetime2,
    col_smalldatetime   smalldatetime not null,
    col_smalldatetime_n smalldatetime,
    col_time            time          not null,
    col_time_n          time,
    col_bigint          bigint        not null,
    col_bigint_n        bigint,
    col_smallint        smallint      not null,
    col_smallint_n      smallint,
    col_tinyint         tinyint       not null,
    col_tinyint_n       tinyint,
    col_money           money         not null,
    col_money_n         money,
    col_smallmoney      smallmoney    not null,
    col_smallmoney_n    smallmoney,
    col_char            char(8)       not null,
    col_char_n          char(8),
    col_varchar         varchar(8)    not null,
    col_varchar_n       varchar(8),
    col_nchar           nchar(8)      not null,
    col_nchar_n         nchar(8),
    col_nvarchar        nvarchar(8)   not null,
    col_nvarchar_n      nvarchar(8),
    col_binary          binary(8)     not null,
    col_binary_n        binary(8),
    col_varbinary       varbinary(8)  not null,
    col_varbinary_n     varbinary(8)
)
go


create unique index type_test_col_id_uindex
    on type_test (col_id)
go

INSERT INTO sqtype.dbo.type_test (col_int, col_int_n, col_bool, col_bool_n, col_decimal, col_decimal_n, col_float, col_float_n, col_date, col_date_n, col_datetime, col_datetime_n, col_datetime2, col_datetime2_n, col_smalldatetime, col_smalldatetime_n, col_time, col_time_n, col_bigint, col_bigint_n, col_smallint, col_smallint_n, col_tinyint, col_tinyint_n, col_money, col_money_n, col_smallmoney, col_smallmoney_n, col_char, col_char_n, col_varchar, col_varchar_n, col_nchar, col_nchar_n, col_nvarchar, col_nvarchar_n, col_binary, col_binary_n, col_varbinary, col_varbinary_n) VALUES (7, 7, 1, 1, 7, 7, 7.7, 7.7, '2016-07-31', '2016-07-31', '2016-07-31 00:00:00.000', '2016-07-31 00:00:00.000', '2016-07-31 00:00:00.0000000', '2016-07-31 00:00:00.0000000', '2016-07-31 00:00:00', '2016-07-31 00:00:00', '12:00:00', '12:00:00', 77, 77, 77, 77, 77, 77, 77.7700, 77.7700, 77.7700, 77.7700, 'a       ', 'a       ', 'a', 'a', 'a       ', 'a       ', 'a', 'a', 0x3100000000000000, 0x3100000000000000, 0x31, 0x31);
INSERT INTO sqtype.dbo.type_test (col_int, col_int_n, col_bool, col_bool_n, col_decimal, col_decimal_n, col_float, col_float_n, col_date, col_date_n, col_datetime, col_datetime_n, col_datetime2, col_datetime2_n, col_smalldatetime, col_smalldatetime_n, col_time, col_time_n, col_bigint, col_bigint_n, col_smallint, col_smallint_n, col_tinyint, col_tinyint_n, col_money, col_money_n, col_smallmoney, col_smallmoney_n, col_char, col_char_n, col_varchar, col_varchar_n, col_nchar, col_nchar_n, col_nvarchar, col_nvarchar_n, col_binary, col_binary_n, col_varbinary, col_varbinary_n) VALUES (7, null, 1, null, 7, null, 7.7, null, '2016-07-31', null, '2016-07-31 00:00:00.000', null, '2016-07-31 00:00:00.0000000', null, '2016-07-31 00:00:00', null, '12:00:00', null, 77, null, 77, null, 77, null, 77.7700, null, 77.7700, null, 'a       ', null, 'a', null, 'a       ', null, 'a', null, 0x3100000000000000, null, 0x31, null);
INSERT INTO sqtype.dbo.type_test (col_int, col_int_n, col_bool, col_bool_n, col_decimal, col_decimal_n, col_float, col_float_n, col_date, col_date_n, col_datetime, col_datetime_n, col_datetime2, col_datetime2_n, col_smalldatetime, col_smalldatetime_n, col_time, col_time_n, col_bigint, col_bigint_n, col_smallint, col_smallint_n, col_tinyint, col_tinyint_n, col_money, col_money_n, col_smallmoney, col_smallmoney_n, col_char, col_char_n, col_varchar, col_varchar_n, col_nchar, col_nchar_n, col_nvarchar, col_nvarchar_n, col_binary, col_binary_n, col_varbinary, col_varbinary_n) VALUES (0, 0, 0, 0, 0, 0, 0, 0, '2016-07-31', '2016-07-31', '2016-07-31 00:00:00.000', '2016-07-31 00:00:00.000', '2016-07-31 00:00:00.0000000', '2016-07-31 00:00:00.0000000', '2016-07-31 00:00:00', '2016-07-31 00:00:00', '00:00:00', '00:00:00', 0, 0, 0, 0, 0, 0, 0.0000, 0.0000, 0.0000, 0.0000, '        ', '        ', '', '', '        ', '        ', '', '', 0x0000000000000000, 0x0000000000000000, 0x00, 0x00);