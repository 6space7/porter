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
- prints an initial admin password and stores it once at `/etc/porter/initial-password`.

Save the initial password, then remove `/etc/porter/initial-password`.

## API Smoke Test

```bash
PASSWORD="$(sudo cat /etc/porter/initial-password)"

curl http://127.0.0.1:8080/health

TOKEN="$(
  curl -sS -H "Content-Type: application/json" \
    -d "{\"email\":\"admin@porter.local\",\"password\":\"${PASSWORD}\"}" \
    http://127.0.0.1:8080/api/v1/auth/login |
    python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])'
)"

PROJECT_ID="$(
  curl -sS -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"name":"demo"}' \
    http://127.0.0.1:8080/api/v1/projects |
    python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])'
)"
```

Create an app from a public repository with a Dockerfile:

```bash
APP_ID="$(
  curl -sS -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{
      \"project_id\":\"${PROJECT_ID}\",
      \"name\":\"web\",
      \"git_url\":\"https://github.com/dockersamples/helloworld-demo-node.git\",
      \"branch\":\"main\",
      \"build_type\":\"dockerfile\",
      \"internal_port\":8080
    }" \
    http://127.0.0.1:8080/api/v1/apps |
    python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])'
)"
```

Deploy and inspect logs:

```bash
DEPLOYMENT_ID="$(
  curl -sS -X POST -H "Authorization: Bearer ${TOKEN}" \
    http://127.0.0.1:8080/api/v1/apps/${APP_ID}/deploy |
    python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])'
)"

curl -sS -H "Authorization: Bearer ${TOKEN}" \
  http://127.0.0.1:8080/api/v1/deployments/${DEPLOYMENT_ID}/build-log

curl -sS -H "Authorization: Bearer ${TOKEN}" \
  http://127.0.0.1:8080/api/v1/apps/${APP_ID}/domains
```

Check expected security failures:

```bash
READ_TOKEN="$(
  curl -sS -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"name":"read-only","scopes":["apps:read"]}' \
    http://127.0.0.1:8080/api/v1/auth/tokens |
    python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])'
)"

curl -i -X POST -H "Authorization: Bearer ${READ_TOKEN}" \
  http://127.0.0.1:8080/api/v1/apps/${APP_ID}/deploy

curl -i -X POST \
  http://127.0.0.1:8080/api/v1/apps/${APP_ID}/deploy

curl -i -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{
    \"project_id\":\"${PROJECT_ID}\",
    \"name\":\"bad\",
    \"git_url\":\"file:///etc/passwd\",
    \"branch\":\"main\",
    \"build_type\":\"dockerfile\",
    \"internal_port\":8080
  }" \
  http://127.0.0.1:8080/api/v1/apps

curl -i -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"hostname":"not-pointing.example.com"}' \
  http://127.0.0.1:8080/api/v1/apps/${APP_ID}/domains
```
