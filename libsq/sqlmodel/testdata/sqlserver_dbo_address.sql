create table address
(
    address_id int identity
        constraint address_pk
            primary key,
    street     varchar(255),
    city       varchar(255),
    state      varchar(255),
    zip        varchar(255),
    country    varchar(255)
)
go

create unique index address_address_id_uindex
    on address (address_id)
go

INSERT INTO address (street, city, state, zip, country) VALUES ('1600 Penn', 'Washington', 'DC', '12345', 'US');
INSERT INTO address (street, city, state, zip, country) VALUES ('999 Coleridge St', 'Ulan Bator', 'UB', '888', 'MN');