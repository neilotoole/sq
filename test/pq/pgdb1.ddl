
CREATE TABLE tblall
(
  col_id INTEGER PRIMARY KEY NOT NULL,
  col_int INTEGER NOT NULL,
  col_int_n INTEGER,
  col_varchar VARCHAR(255) NOT NULL,
  col_varchar_n VARCHAR(255),
  col_blob BYTEA,
  col_blob_n BYTEA
);
CREATE UNIQUE INDEX tblall_col_id_uindex ON tblall (col_id);
CREATE TABLE tbluser
(
  uid INTEGER PRIMARY KEY NOT NULL,
  username VARCHAR(255) NOT NULL,
  email VARCHAR(255) NOT NULL
);
CREATE UNIQUE INDEX tbluser_uid_uindex ON tbluser (uid);
CREATE UNIQUE INDEX tbluser_username_uindex ON tbluser (username);
CREATE TABLE tbladdress
(
  address_id INTEGER PRIMARY KEY NOT NULL,
  street VARCHAR(255) NOT NULL,
  city VARCHAR(255) NOT NULL,
  state VARCHAR(255) NOT NULL,
  zip VARCHAR(255),
  country VARCHAR(255) NOT NULL,
  uid INTEGER,
  CONSTRAINT tbladdress_user_uid_fk FOREIGN KEY (uid) REFERENCES tbluser (uid)
);
CREATE UNIQUE INDEX tbladdress_address_id_uindex ON tbladdress (address_id);