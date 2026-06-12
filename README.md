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

Phase 1 backend is complete and has been verified on a fresh Ubuntu VPS. The
current binary provides the core JSON API, SQLite store, scoped bearer-token
auth, Docker deployment pipeline, Dockerfile `EXPOSE` port detection, managed
Caddy routing, stored build logs, runtime log streaming, source install, and
secure secret handling. The Svelte UI is Phase 2.

Verified on 2026-06-12:

- source install brings the platform up over HTTPS on an sslip.io domain;
- a public Dockerfile repo deploys through the JSON API and is reachable over
  HTTPS on its generated app domain;
- Caddy routes update when app/domain/deploy settings change;
- custom-domain DNS preflight returns the required A record when DNS is wrong;
- broken Dockerfile builds return `status=failed`, `stage=building`, and a
  build log;
- env var changes apply on redeploy and secret values stay masked in API/build
  logs;
- scoped and unauthenticated requests are rejected;
- malicious Git URLs are rejected;
- Caddy's admin API is bound to localhost only.

## Source Install

Run from a checked-out repository on a Debian or Ubuntu amd64/arm64 server:

```bash
sudo ./install.sh
```

The installer:

- installs Docker if it is missing;
- installs Go 1.25 or newer from the official Go distribution when needed;
- writes config under `/etc/porter`;
- writes runtime data under `/var/lib/porter`;
- creates `/etc/porter/master.key` with `0600` permissions;
- builds `/usr/local/bin/porter` from source;
- creates and starts `porter.service`;
- prints the HTTPS sslip.io dashboard/API URL when the public IP is detected;
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
      \"git_url\":\"https://github.com/chandu-muthyala/Dockerizing-a-NodeJS-web-app.git\",
      \"branch\":\"master\",
      \"build_type\":\"dockerfile\",
      \"internal_port\":3000
    }" \
    http://127.0.0.1:8080/api/v1/apps |
    python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])'
)"
```

`internal_port` may be left at the default for many Dockerfile apps. During
deploy, porter re-detects a Dockerfile `EXPOSE` port when one is present and
updates the stored route before serving traffic.

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
