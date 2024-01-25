create table person
(
    uid INTEGER PRIMARY KEY AUTOINCREMENT,
    username   TEXT not null,
    email      TEXT not null,
    address_id int
        references address
);

create unique index person_uid_uindex
    on person (uid);

create unique index person_username_uindex
    on person (username);

INSERT INTO person (uid, username, email, address_id) VALUES (1, 'neilotoole', 'neilotoole@apache.org', null);
INSERT INTO person (uid, username, email, address_id) VALUES (2, 'ksoze', 'kaiser@soze.net', null);
INSERT INTO person (uid, username, email, address_id) VALUES (3, 'kubla', 'kubla@khan.mn', null);
INSERT INTO person (uid, username, email, address_id) VALUES (4, 'tesla', 'tesla@tesla.rs', null);
INSERT INTO person (uid, username, email, address_id) VALUES (5, 'augustus', 'augustus@caesar.org', null);
INSERT INTO person (uid, username, email, address_id) VALUES (6, 'julius', 'julius@caesar.org', null);
INSERT INTO person (uid, username, email, address_id) VALUES (7, 'plato', 'plato@athens.gr', null);
