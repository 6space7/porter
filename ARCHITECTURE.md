# Architecture

porter is a single Go binary. It serves the JSON API, runs SQLite migrations,
talks to Docker, manages Caddy, exposes deployment logs, and serves the
embedded Svelte web UI, machine-readable agent docs, and an authenticated MCP
HTTP server. Later phases add broader deployment automation.

## Fixed Stack

- Backend: Go with `chi`
- Database: SQLite through `modernc.org/sqlite` and `sqlc`
- Docker: official Go Docker SDK
- Proxy: Caddy managed through its admin API
- Frontend: Svelte 5, Vite, and TypeScript embedded with `//go:embed`
- MCP: Go server served from the same binary

## Package Layout

The codebase uses domain-based packages:

- `cmd/server/main.go`
- `internal/api`
- `internal/auth`
- `internal/config`
- `internal/crypto`
- `internal/deploy`
- `internal/docs`
- `internal/docker`
- `internal/frontend`
- `internal/install`
- `internal/mcp`
- `internal/proxy`
- `internal/runtime`
- `internal/store`
- `frontend`

## Design Notes

All product behavior must be reachable through `/api/v1/...`; the web UI,
future CLI, and MCP server are clients of that API.

The MCP server is mounted at `/api/v1/mcp` behind the same bearer-token
middleware as the JSON API. Tool handlers reuse the API service interfaces for
projects, apps, deployments, logs, env vars, services, rollback, and diagnosis.
`/llms.txt` and `/api/v1/docs` are public machine-readable docs that describe
auth, endpoints, tools, scopes, and example MCP payloads.

The UI is built in `frontend/` with Svelte 5, Vite, and TypeScript. `frontend/embed.go`
embeds `frontend/dist`, and `internal/frontend` serves static assets with an SPA
fallback while preserving API routes for `internal/api`.

Runtime startup opens the SQLite database, loads the master key when configured,
bootstraps an initial hashed admin bearer token when provided, prepares Docker
deployment/log backends, optionally ensures the managed Caddy container exists,
reconciles verified domains into Caddy, and then serves the API.

Route reconciliation uses SQLite as the source of truth. Startup reconciles all
verified domains, and app creation, custom-domain creation, and deploy-time
port changes trigger another Caddy load through the admin API.

Deployments clone the target Git repo into the workspace, detect a Dockerfile
`EXPOSE` port when present, build through the Docker SDK, run the app container
on the shared `porter-proxy` Docker network, update the stored internal port
when detection changes it, and record staged deployment logs with known secret
values redacted. Successful deployments and rollbacks retain Docker image tags
for rollback. Porter keeps the newest five distinct successful image tags per
app, prunes older images from Docker, and clears old deployment `image_tag`
values so the API/UI can show history without advertising unavailable rollback
targets.

When an app is explicitly set to `nixpacks`, or when a Dockerfile app has no
Dockerfile in the cloned source, the deployment builder shells out to the host
`nixpacks` CLI and records the detected build type. The installer provisions
Nixpacks on Debian/Ubuntu hosts so Dockerfile-less apps can build on fresh VPS
installs.

The service catalog is embedded from `templates/*.yaml`. Creating a service
renders template variables, generates required secrets, encrypts generated
outputs in SQLite, pulls the service image, creates named Docker volumes, and
runs the service container on the shared `porter-proxy` network. Internal
services expose their container DNS name to attached apps through generated env
vars such as `DATABASE_URL`; exposed services also receive generated sslip.io
hostnames and are included in Caddy route reconciliation.

Caddy's on-demand TLS ask endpoint allows the same verified proxy route set
used for config generation: verified app domains, the platform domain, and
exposed service hostnames. Internal service hostnames are not authorized for
public TLS.

Production source installs use:

- `/etc/porter` for config and the master key;
- `/var/lib/porter` for the SQLite database and workspaces;
- `/usr/local/bin/porter` for the built binary;
- `porter.service` for systemd supervision.
