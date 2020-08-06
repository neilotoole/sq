create table address
(
    address_id int auto_increment,
    street     varchar(255) not null,
    city       varchar(255) not null,
    state      varchar(255) not null,
    zip        varchar(255) null,
    country    varchar(255) not null,
    constraint address_address_id_uindex
        unique (address_id)
);

alter table address
    add primary key (address_id);

INSERT INTO sqtest.address (street, city, state, zip, country) VALUES ('1600 Penn', 'Washington', 'DC', '12345', 'US');
INSERT INTO sqtest.address (street, city, state, zip, country) VALUES ('999 Coleridge St', 'Ulan Bator', 'UB', '888', 'MN');