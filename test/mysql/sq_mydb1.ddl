CREATE TABLE item
(
  item_id INT(11) PRIMARY KEY NOT NULL AUTO_INCREMENT,
  name VARCHAR(255) NOT NULL,
  price FLOAT NOT NULL
);
CREATE UNIQUE INDEX item_iid_uindex ON item (item_id);
CREATE UNIQUE INDEX item_name_uindex ON item (name);
CREATE TABLE `order`
(
  oid INT(11) PRIMARY KEY NOT NULL AUTO_INCREMENT,
  uid INT(11) NOT NULL,
  item_id INT(11) NOT NULL,
  quantity INT(11) NOT NULL,
  CONSTRAINT order_user_uid_fk FOREIGN KEY (uid) REFERENCES user (uid),
  CONSTRAINT order_item_iid_fk FOREIGN KEY (item_id) REFERENCES item (item_id)
);
CREATE INDEX order_item_iid_fk ON `order` (item_id);
CREATE UNIQUE INDEX order_oid_uindex ON `order` (oid);
CREATE INDEX order_user_uid_fk ON `order` (uid);
CREATE TABLE user
(
  uid INT(11) PRIMARY KEY NOT NULL AUTO_INCREMENT,
  username VARCHAR(64) NOT NULL,
  email VARCHAR(128) NOT NULL
);
CREATE UNIQUE INDEX user_uid_uindex ON user (uid);
CREATE UNIQUE INDEX user_username_uindex ON user (username);
CREATE TABLE address
(
  address_id INT(11) PRIMARY KEY NOT NULL AUTO_INCREMENT,
  street VARCHAR(255) NOT NULL,
  city VARCHAR(255) NOT NULL,
  state VARCHAR(255) NOT NULL,
  zip VARCHAR(255),
  country VARCHAR(255) NOT NULL,
  uid INT(11) NOT NULL,
  CONSTRAINT address_user_uid_fk FOREIGN KEY (uid) REFERENCES user (uid)
);
CREATE UNIQUE INDEX address_address_id_uindex ON address (address_id);
CREATE INDEX address_user_uid_fk ON address (uid);
CREATE TABLE tblall
(
  col_id INT(11) PRIMARY KEY NOT NULL AUTO_INCREMENT,
  col_int INT(11) NOT NULL,
  col_int_n INT(11),
  col_bool TINYINT(1) NOT NULL,
  col_bool_n TINYINT(1),
  col_decimal DECIMAL(10) NOT NULL,
  col_decimal_n DECIMAL(10),
  col_tiny TINYINT(4) NOT NULL,
  col_tiny_n TINYINT(4),
  col_short SMALLINT(6) NOT NULL,
  col_short_n SMALLINT(6),
  col_long MEDIUMTEXT NOT NULL,
  col_long_n MEDIUMTEXT,
  col_float FLOAT NOT NULL,
  col_float_n FLOAT,
  col_double DOUBLE NOT NULL,
  col_double_n DOUBLE,
  col_timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
  col_timestamp_n TIMESTAMP,
  col_longlong MEDIUMTEXT NOT NULL,
  col_longlong_n MEDIUMTEXT,
  col_int24 INT(24) NOT NULL,
  col_int24_n INT(24),
  col_date DATE NOT NULL,
  col_date_n DATE,
  col_time TIME NOT NULL,
  col_time_n TIME,
  col_datetime DATETIME NOT NULL,
  col_datetime_n DATETIME,
  col_year YEAR(4) NOT NULL,
  col_year_n YEAR(4),
  col_varchar VARCHAR(255) NOT NULL,
  col_varchar_n VARCHAR(255),
  col_json JSON NOT NULL,
  col_json_n JSON,
  col_enum ENUM('a', 'b', 'c') NOT NULL,
  col_enum_n ENUM('a', 'b', 'c'),
  col_binary BINARY(8) NOT NULL,
  col_binary_n BINARY(8),
  col_varbinary VARBINARY(64) NOT NULL,
  col_varbinary_n VARBINARY(64),
  col_blob BLOB NOT NULL,
  col_blob_n BLOB,
  col_tinyblob TINYBLOB NOT NULL,
  col_tinyblob_n TINYBLOB,
  col_mediumblob MEDIUMBLOB NOT NULL,
  col_mediumblob_n MEDIUMBLOB,
  col_longblob LONGBLOB NOT NULL,
  col_longblob_n LONGBLOB,
  col_text TEXT NOT NULL,
  col_text_n TEXT,
  col_longtext LONGTEXT NOT NULL,
  col_longtext_n LONGTEXT
);
CREATE UNIQUE INDEX tbl_all_types_col_id_uindex ON tblall (col_id);