-- name: CreateProject :one
insert into projects (id, name)
values (?, ?)
returning id, name, created_at, updated_at;

-- name: ListProjects :many
select id, name, created_at, updated_at
from projects
order by created_at desc, name asc;

-- name: GetProject :one
select id, name, created_at, updated_at
from projects
where id = ?;

-- name: DeleteProject :exec
delete from projects
where id = ?;

-- name: CreateApp :one
insert into apps (
	id,
	project_id,
	server_id,
	name,
	git_url,
	branch,
	build_type,
	internal_port,
	status
)
values (?, ?, ?, ?, ?, ?, ?, ?, ?)
returning id, project_id, server_id, name, git_url, branch, build_type, internal_port, status, created_at, updated_at;

-- name: GetApp :one
select id, project_id, server_id, name, git_url, branch, build_type, internal_port, status, created_at, updated_at
from apps
where id = ?;

-- name: ListApps :many
select id, project_id, server_id, name, git_url, branch, build_type, internal_port, status, created_at, updated_at
from apps
order by created_at desc, name asc;

-- name: ListAppsByProject :many
select id, project_id, server_id, name, git_url, branch, build_type, internal_port, status, created_at, updated_at
from apps
where project_id = ?
order by created_at desc, name asc;

-- name: UpdateAppStatus :exec
update apps
set status = ?, updated_at = current_timestamp
where id = ?;

-- name: DeleteApp :exec
delete from apps
where id = ?;

-- name: CreateDomain :one
insert into domains (id, app_id, hostname, type, verified)
values (?, ?, ?, ?, ?)
returning id, app_id, hostname, type, verified, created_at;

-- name: GetDomainByHostname :one
select id, app_id, hostname, type, verified, created_at
from domains
where hostname = ?;

-- name: ListDomainsByApp :many
select id, app_id, hostname, type, verified, created_at
from domains
where app_id = ?
order by created_at asc;

-- name: DeleteDomain :exec
delete from domains
where id = ?;

-- name: CreateDeployment :one
insert into deployments (id, app_id, status, stage, build_log, image_tag)
values (?, ?, ?, ?, ?, ?)
returning id, app_id, status, stage, build_log, image_tag, created_at;

-- name: UpdateDeploymentStatus :exec
update deployments
set status = ?, stage = ?, build_log = ?, image_tag = ?
where id = ?;

-- name: GetDeployment :one
select id, app_id, status, stage, build_log, image_tag, created_at
from deployments
where id = ?;

-- name: ListDeploymentsByApp :many
select id, app_id, status, stage, build_log, image_tag, created_at
from deployments
where app_id = ?
order by created_at desc;

-- name: UpsertEnvVar :one
insert into env_vars (app_id, key, value, is_secret)
values (?, ?, ?, ?)
on conflict(app_id, key) do update set
	value = excluded.value,
	is_secret = excluded.is_secret,
	updated_at = current_timestamp
returning app_id, key, value, is_secret, created_at, updated_at;

-- name: ListEnvVarsByApp :many
select app_id, key, value, is_secret, created_at, updated_at
from env_vars
where app_id = ?
order by key asc;

-- name: DeleteEnvVar :exec
delete from env_vars
where app_id = ? and key = ?;

-- name: CreateToken :one
insert into tokens (id, name, hash, scopes)
values (?, ?, ?, ?)
returning id, name, hash, scopes, created_at, last_used_at;

-- name: GetTokenByHash :one
select id, name, hash, scopes, created_at, last_used_at
from tokens
where hash = ?;

-- name: CreateUser :one
insert into users (id, email, password_hash)
values (?, ?, ?)
returning id, email, password_hash, created_at, updated_at;

-- name: GetUserByEmail :one
select id, email, password_hash, created_at, updated_at
from users
where email = ?;
