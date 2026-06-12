create table servers (
	id text primary key,
	name text not null,
	host text not null,
	ssh_key_ref text,
	status text not null,
	created_at text not null default current_timestamp,
	updated_at text not null default current_timestamp
);

create table projects (
	id text primary key,
	name text not null unique,
	created_at text not null default current_timestamp,
	updated_at text not null default current_timestamp
);

create table apps (
	id text primary key,
	project_id text not null references projects(id) on delete cascade,
	server_id text not null references servers(id),
	name text not null,
	git_url text not null,
	branch text not null default 'main',
	build_type text not null check (build_type in ('dockerfile', 'nixpacks')),
	internal_port integer not null default 3000,
	status text not null default 'created',
	created_at text not null default current_timestamp,
	updated_at text not null default current_timestamp,
	unique(project_id, name)
);

create table domains (
	id text primary key,
	app_id text not null references apps(id) on delete cascade,
	hostname text not null unique,
	type text not null check (type in ('generated', 'custom')),
	verified integer not null default 0 check (verified in (0, 1)),
	created_at text not null default current_timestamp
);

create table deployments (
	id text primary key,
	app_id text not null references apps(id) on delete cascade,
	status text not null,
	stage text not null,
	build_log text not null default '',
	image_tag text,
	created_at text not null default current_timestamp
);

create table env_vars (
	app_id text not null references apps(id) on delete cascade,
	key text not null,
	value text not null,
	is_secret integer not null default 0 check (is_secret in (0, 1)),
	created_at text not null default current_timestamp,
	updated_at text not null default current_timestamp,
	primary key (app_id, key)
);

create table services (
	id text primary key,
	project_id text not null references projects(id) on delete cascade,
	server_id text not null references servers(id),
	template_slug text not null,
	name text not null,
	status text not null,
	generated_secrets text not null default '{}',
	created_at text not null default current_timestamp,
	updated_at text not null default current_timestamp,
	unique(project_id, name)
);

create table tokens (
	id text primary key,
	name text not null,
	hash text not null unique,
	scopes text not null,
	created_at text not null default current_timestamp,
	last_used_at text
);

create table users (
	id text primary key,
	email text not null unique,
	password_hash text not null,
	created_at text not null default current_timestamp,
	updated_at text not null default current_timestamp
);
