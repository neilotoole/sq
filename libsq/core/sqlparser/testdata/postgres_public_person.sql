create table person
(
    uid        serial       not null
        constraint person_pk
            primary key,
    username   varchar(255) not null,
    email      varchar(255) not null,
    address_id integer
        constraint person_address_address_id_fk
            references address
);

alter table person
    owner to sq;

create unique index person_uid_uindex
    on person (uid);

create unique index person_username_uindex
    on person (username);

INSERT INTO public.person (uid, username, email, address_id) VALUES (3, 'kubla', 'kubla@khan.mn', null);
INSERT INTO public.person (uid, username, email, address_id) VALUES (6, 'julius', 'julius@caesar.org', null);
INSERT INTO public.person (uid, username, email, address_id) VALUES (7, 'plato', 'plato@athens.gr', 1);
INSERT INTO public.person (uid, username, email, address_id) VALUES (2, 'ksoze', 'kaiser@soze.org', 2);
INSERT INTO public.person (uid, username, email, address_id) VALUES (4, 'tesla', 'nikola@tesla.rs', 1);
INSERT INTO public.person (uid, username, email, address_id) VALUES (1, 'neilotoole', 'neilotoole@apache.org', 1);
INSERT INTO public.person (uid, username, email, address_id) VALUES (5, 'augustus', 'augustus@caesar.org', 2);