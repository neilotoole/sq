create table person
(
    uid        int auto_increment,
    username   varchar(64)  not null,
    email      varchar(128) not null,
    address_id int          null,
    constraint person_uid_uindex
        unique (uid),
    constraint person_username_uindex
        unique (username),
    constraint person_address_address_id_fk
        foreign key (address_id) references address (address_id)
);

alter table person
    add primary key (uid);

INSERT INTO sqtest.person (username, email, address_id) VALUES ('neilotoole', 'neilotoole@apache.org', 1);
INSERT INTO sqtest.person (username, email, address_id) VALUES ('ksoze', 'kaiser@soze.org', 2);
INSERT INTO sqtest.person (username, email, address_id) VALUES ('kubla', 'kubla@khan.mn', null);
INSERT INTO sqtest.person (username, email, address_id) VALUES ('tesla', 'nikola@tesla.rs', 1);
INSERT INTO sqtest.person (username, email, address_id) VALUES ('augustus', 'augustus@caesar.org', 2);
INSERT INTO sqtest.person (username, email, address_id) VALUES ('julius', 'julius@caesar.org', null);
INSERT INTO sqtest.person (username, email, address_id) VALUES ('plato', 'plato@athens.gr', 1);