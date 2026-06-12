# Architecture

porter is a single Go binary. It serves the JSON API, runs SQLite migrations,
talks to Docker, manages Caddy, exposes deployment logs, and serves the
embedded Svelte web UI. Later phases add the MCP server and broader deployment
automation.

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
- `internal/docker`
- `internal/frontend`
- `internal/install`
- `internal/proxy`
- `internal/runtime`
- `internal/store`
- `frontend`

## Design Notes

All product behavior must be reachable through `/api/v1/...`; the web UI,
future CLI, and MCP server are clients of that API.

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
values redacted.

Production source installs use:

- `/etc/porter` for config and the master key;
- `/var/lib/porter` for the SQLite database and workspaces;
- `/usr/local/bin/porter` for the built binary;
- `porter.service` for systemd supervision.
