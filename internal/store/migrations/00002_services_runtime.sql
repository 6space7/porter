alter table services
	add column internal_port integer not null default 0;

alter table services
	add column exposed integer not null default 0 check (exposed in (0, 1));

alter table services
	add column hostname text;

create unique index services_hostname_unique
	on services(hostname)
	where hostname is not null and hostname <> '';
