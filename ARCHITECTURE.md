# Architecture

porter is planned as a single Go binary that serves the API, embedded frontend, migrations, and later the MCP server.

## Fixed Stack

- Backend: Go with `chi`
- Database: SQLite through `modernc.org/sqlite`, `sqlc`, and `goose`
- Docker: official Go Docker SDK
- Proxy: Caddy managed through its admin API
- Frontend: Svelte 5, Vite, and TypeScript embedded with `//go:embed`
- MCP: Go server served from the same binary

## Package Layout

The codebase will use domain-based packages:

- `cmd/server/main.go`
- `internal/api`
- `internal/deploy`
- `internal/docker`
- `internal/proxy`
- `internal/store`
- `internal/services`
- `internal/mcp`
- `frontend`

## Design Notes

All product behavior must be reachable through `/api/v1/...`; the web UI, future CLI, and MCP server are clients of that API.
