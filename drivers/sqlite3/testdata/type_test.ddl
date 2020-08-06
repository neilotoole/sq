CREATE TABLE type_test
(
    col_id       	INTEGER NOT NULL PRIMARY KEY,
    col_int      	INT NOT NULL,
    col_int_n    	INT,
    col_double   	REAL NOT NULL,
    col_double_n 	REAL,
	col_boolean		BOOLEAN DEFAULT FALSE NOT NULL,
	col_boolean_n 	BOOLEAN,
    col_text     	TEXT NOT NULL,
    col_text_n   	TEXT,
    col_blob     	BLOB NOT NULL,
    col_blob_n   	BLOB,
	col_datetime	DATETIME DEFAULT '1970-01-01 00:00:00' NOT NULL,
	col_datetime_n	DATETIME,
	col_date		DATE DEFAULT '1970-01-01' NOT NULL,
	col_date_n		DATE,
	col_time		TIME NOT NULL,
	col_time_n		TIME,
	col_decimal		DECIMAL DEFAULT 0 NOT NULL,
	col_decimal_n	DECIMAL
)