create table address
(
    address_id integer      not null
        constraint address_pk
            primary key,
    street     varchar(255) not null,
    city       varchar(255) not null,
    state      varchar(255) not null,
    zip        varchar(255),
    country    varchar(255) not null
);

alter table address
    owner to sq;

create unique index address_address_id_uindex
    on address (address_id);

INSERT INTO public.address (address_id, street, city, state, zip, country) VALUES (1, '1600 Penn', 'Washington', 'DC', '12345', 'US');
INSERT INTO public.address (address_id, street, city, state, zip, country) VALUES (2, '999 Coleridge St', 'Ulan Bator', 'UB', '888', 'MN');