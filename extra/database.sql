create table if not exists auth
(
    container_id text not null,
    user         text,
    password     blob
);

create unique index if not exists auth_container_id_uindex
    on auth (container_id);

create table if not exists settings
(
    container_id        text not null,
    network_mode        enum default 3 not null,
    configurable        bool default 0 not null,
    run_level           enum default 1 not null,
    startup_information bool default 1 not null,
    exit_after          text default '' not null,
    keep_on_exit        bool default 0 not null,
    check (configurable IN (0, 1)),
    check (keep_on_exit IN (0, 1)),
    check (network_mode IN (1, 2, 3, 4, 5)),
    check (run_level IN (1, 2, 3)),
    check (startup_information IN (0, 1))
);

create unique index if not exists settings_container_id_uindex
    on settings (container_id);
