create table person
(
    uid        int identity
        constraint person_pk
            primary key,
    username   varchar(255) not null,
    email      varchar(255) not null,
    address_id int
        constraint person_address_address_id_fk
            references address
)
go

create unique index person_uid_uindex
    on person (uid)
go

create unique index person_username_uindex
    on person (username)
go

INSERT INTO person (username, email, address_id)
VALUES ('neilotoole', 'neilotoole@apache.org', 1);
INSERT INTO person (username, email, address_id)
VALUES ('ksoze', 'kaiser@soze.org', 2);
INSERT INTO person (username, email, address_id)
VALUES ('kubla', 'kublah@khan.mn', null);
INSERT INTO person (username, email, address_id)
VALUES ('tesla', 'nikola@tesla.rs', 1);
INSERT INTO person (username, email, address_id)
VALUES ('augustus', 'augustus@caesar.org', 2);
INSERT INTO person (username, email, address_id)
VALUES ('julius', 'julius@caesar.org', null);
INSERT INTO person (username, email, address_id)
VALUES ('plato', 'plato@athens.gr', 1);