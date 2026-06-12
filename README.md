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

Phases 0 through 4 are complete and verified on a fresh Ubuntu VPS. The current
binary provides the core JSON API, SQLite store, scoped bearer-token auth,
Docker deployment pipeline, Dockerfile `EXPOSE` port detection, managed Caddy
routing, stored build logs, runtime log streaming, deployment history with
one-click rollback/image retention, source install, secure secret handling, and
an embedded Svelte web UI for the core one-server workflows. It also supports
Dockerfile-less Nixpacks builds, service catalog templates, internal service
attachment, public HTTPS service routes, an authenticated MCP HTTP endpoint,
machine-readable agent docs, and dashboard onboarding for AI agents. Phases 0
through 5 are verified; later phases add multi-server deploys, GitHub
auto-deploy, and release lifecycle automation.

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
- Caddy's admin API is bound to localhost only;
- the embedded UI is served by the installed binary over local HTTP and the
  HTTPS sslip.io platform URL;
- rollback targets retain the newest five successful Docker image tags per app,
  and older deployment records are kept without rollback image tags.
- API and browser rollback checks deploy a working v1, deploy an intentionally
  broken v2, and restore v1 without a rebuild.
- Dockerfile-less Node apps fall back to Nixpacks, build on the VPS, persist
  `build_type=nixpacks`, and serve HTTPS traffic on generated app domains.
- the service catalog exposes PostgreSQL, MySQL, Redis, MongoDB, n8n, Uptime
  Kuma, WordPress, Ghost, MinIO, and Plausible templates through the API/UI.
- an internal PostgreSQL service can be created, attached to an app as
  `DATABASE_URL`, and reached from the deployed app container.
- an exposed n8n service receives a generated sslip.io hostname and serves the
  editor over HTTPS through Caddy.
- `/api/v1/mcp` is mounted behind the same bearer-token auth as the JSON API,
  with tools for projects, apps, env vars, deployments/logs, rollback,
  service catalog operations, service attach, and latest-deployment diagnosis.
- `/llms.txt` and `/api/v1/docs` expose concise machine-readable agent
  onboarding docs, including auth, endpoint, tool, scope, and example payloads.
- the public HTTPS MCP endpoint initializes successfully and can call
  `porter_list_projects` through the generated platform domain.

Local browser checks also cover the embedded UI login/logout flow, apps
dashboard, app creation form, app detail actions, domains, environment editor,
deployment/log panes, token settings, services catalog/running/attach views,
and desktop/mobile layouts.

## Web UI

The Svelte 5 UI lives in `frontend/` and is embedded into the Go binary from
`frontend/dist`.

```bash
cd frontend
npm install
npm run check
npm run build
```

The top-level verification target runs the frontend checks and rebuilds the
embedded assets before testing and building the Go server:

```bash
make verify
```

At runtime, open the install URL in a browser to use the dashboard. The UI is a
client of the public `/api/v1` API and does not use private in-process hooks.

## Agent Docs and MCP

porter serves agent-readable docs from the same binary:

- `GET /llms.txt`
- `GET /api/v1/docs`
- `POST /api/v1/mcp`

MCP requests use the same scoped porter tokens as the JSON API:

```text
Authorization: Bearer <porter token>
```

Core tools include `porter_list_projects`, `porter_create_app`,
`porter_deploy_app`, `porter_set_env_var`, `porter_get_build_log`,
`porter_get_runtime_logs`, `porter_rollback_app`,
`porter_search_service_templates`, `porter_deploy_service`,
`porter_attach_service`, and `porter_diagnose_latest_deployment`.

## Source Install

Run from a checked-out repository on a Debian or Ubuntu amd64/arm64 server:

```bash
sudo ./install.sh
```

The installer:

- installs Docker if it is missing;
- installs Go 1.25 or newer from the official Go distribution when needed;
- installs Nixpacks when it is missing;
- writes config under `/etc/porter`;
- writes runtime data under `/var/lib/porter`;
- creates `/etc/porter/master.key` with `0600` permissions;
- builds `/usr/local/bin/porter` from source;
- creates and starts `porter.service`;
- prints the HTTPS sslip.io dashboard/API URL when the public IP is detected;
- prints an initial admin password and stores it once at `/etc/porter/initial-password`.

Save the initial password, then remove `/etc/porter/initial-password`.

## Lifecycle Commands

The installed binary includes lifecycle helpers:

```bash
sudo porter backup \
  --database /var/lib/porter/porter.db \
  --backup-dir /var/lib/porter/backups

sudo porter update \
  --version vX.Y.Z \
  --database /var/lib/porter/porter.db \
  --backup-dir /var/lib/porter/backups
```

`backup` writes a timestamped SQLite snapshot. `update` currently creates the
same backup and prints the release archive URL that the release installer path
will use.

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
