# porter

porter is an open source, self-hosted deployment platform for running AI-built apps on a VPS without requiring the user to understand Docker, reverse proxies, or TLS.

The product is built phase by phase from the project brief:

1. Phase 0: repository and GitHub setup.
2. Phase 1: core API and deployment engine for one server.
3. Phase 2: embedded Svelte web UI.
4. Phase 3: deployment history and rollback.
5. Phase 4: service catalog and Nixpacks.
6. Phase 5: MCP server and AI-agent workflow.
7. Phase 6: multi-server deploys, GitHub auto-deploy, and release lifecycle.

## Principles

- The JSON API is the product.
- Everything must be machine-legible for AI agents.
- Secure defaults come first.
- Deployment detection happens on every deploy.
- A production install should be one binary and one command.

## Status

Phase 1 is in progress. The current binary provides the core JSON API, SQLite
store, scoped bearer-token auth, Docker deployment pipeline, managed Caddy
routing, stored build logs, and runtime log streaming. The Svelte UI is not
part of Phase 1.

## Source Install

Run from a checked-out repository on a Debian or Ubuntu amd64/arm64 server with
Go 1.25 or newer installed:

```bash
sudo ./install.sh
```

The installer:

- installs Docker if it is missing;
- writes config under `/etc/porter`;
- writes runtime data under `/var/lib/porter`;
- creates `/etc/porter/master.key` with `0600` permissions;
- builds `/usr/local/bin/porter` from source;
- creates and starts `porter.service`;
- prints an initial bearer token and stores it once at `/etc/porter/initial-token`.

Save the initial token, then remove `/etc/porter/initial-token`.

## API Smoke Test

```bash
TOKEN="$(sudo cat /etc/porter/initial-token)"

curl http://127.0.0.1:8080/health

curl -sS -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"name":"demo"}' \
  http://127.0.0.1:8080/api/v1/projects
```

Create an app from a public repository with a Dockerfile:

```bash
curl -sS -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "project_id":"proj_REPLACE_ME",
    "name":"web",
    "git_url":"https://github.com/example/web.git",
    "branch":"main",
    "build_type":"dockerfile",
    "internal_port":3000
  }' \
  http://127.0.0.1:8080/api/v1/apps
```

Deploy and inspect logs:

```bash
curl -sS -X POST -H "Authorization: Bearer ${TOKEN}" \
  http://127.0.0.1:8080/api/v1/apps/app_REPLACE_ME/deploy

curl -sS -H "Authorization: Bearer ${TOKEN}" \
  http://127.0.0.1:8080/api/v1/deployments/dep_REPLACE_ME/build-log
```
