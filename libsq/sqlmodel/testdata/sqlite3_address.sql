create table address
(
    address_id INTEGER not null
        primary key,
    street     TEXT    not null,
    city       TEXT    not null,
    state      TEXT,
    zip        TEXT,
    country    TEXT    not null
);

INSERT INTO address (address_id, street, city, state, zip, country) VALUES (1, '1600 Penn', 'Washington', 'DC', '12345', 'US');
INSERT INTO address (address_id, street, city, state, zip, country) VALUES (2, '999 Coleridge', 'Ulan Bator', 'UB', '888', 'MN');
INSERT INTO address (address_id, street, city, state, zip, country) VALUES (8, 'street', 'thecity', 'CO', '80302', 'USA');