# Decisions

This log records product and engineering decisions that are not already fixed by the project brief.

## 2026-06-12

- The project is named `porter`.
- The initial repository uses `main` as the default branch.
- Phase 1 source installs use `/etc/porter` for config and `/var/lib/porter`
  for data so the binary, installer, and docs share one production path model.
- The installer generates an initial admin password at
  `/etc/porter/initial-password`; runtime hashes it into the `users` table on
  first startup, and `/api/v1/auth/login` mints scoped bearer tokens. The
  password file is root-readable and should be removed after saving the
  password.
- Managed Caddy is enabled by default for installed servers and can be disabled
  with `PORTER_MANAGE_CADDY=false` for local development or external proxy
  setups.
- Managed Caddy runs in Docker, so its on-demand TLS `ask` URL and porter
  platform route use `host.docker.internal:8080` with Docker's
  `host-gateway` mapping instead of `127.0.0.1`.
- Docker image builds inspect the JSON build stream for embedded `error`
  events; the Docker SDK can return a successful API response while the build
  itself failed.
- Phase 6 installers default to downloading GitHub release archives, while
  `PORTER_INSTALL_FROM_SOURCE=1` keeps the source-build path for development
  and for bootstrapping before the first tagged release exists. Go is installed
  only for that explicit source-build path.
- Caddy full-config updates use `POST /load`, not `PUT /config`, and the admin
  load call retries briefly because the managed Caddy container can accept a TCP
  connection before the admin API is ready.
- Caddy on-demand TLS is configured with the current HTTP permission module
  shape, while the admin listener stays enabled in the loaded config.
- Missing DNS for a custom domain is treated as a structured DNS preflight
  failure with an empty current-record list, not an internal server error.
- Dockerfile apps re-detect the first valid `EXPOSE` port on every deploy and
  update the stored app route when it differs from the previous/default port.
- Phase 3 Docker image retention keeps the newest five distinct successful
  image tags per app. Older deployment history remains in SQLite, but pruned
  records have `image_tag` cleared so they are visibly unavailable for
  rollback.
- Phase 3 verification covers both API and browser paths: deploy a working v1,
  deploy a broken v2, then roll back to v1 without running a new Docker build.
- Nixpacks runs as a host CLI installed by `install.sh`, because the published
  Nixpacks container image is not a drop-in `nixpacks build` command runner.
- Dockerfile builds fall back to Nixpacks only when no Dockerfile exists, and a
  successful fallback persists the app `build_type` as `nixpacks`.
- Service containers stay non-privileged but use Docker's default capability
  set; dropping all capabilities prevented official database images such as
  PostgreSQL from switching to their runtime user.
- Public service hostnames share the same Caddy route source as verified app
  domains, and Caddy's TLS ask endpoint authorizes that combined proxy route
  set.
- Deployment failures caused by request cancellation are still persisted as
  failed deployment records so history does not get stuck at `running` with no
  image tag.
- The MCP endpoint uses streamable HTTP under `/api/v1/mcp` and is protected by
  the existing bearer-token middleware. Tool handlers call the same API service
  interfaces as the dashboard, while `/llms.txt` and `/api/v1/docs` stay public
  because they contain endpoint and scope metadata but no secrets.
- Git push webhooks stay unauthenticated at the bearer-token layer so hosted Git
  providers can call them directly, but each enabled app stores its own generated
  HMAC secret. Porter verifies the raw request body before parsing JSON and
  skips branch mismatches without starting a deployment.
- Remote server private keys are write-only in the API. The server registry
  persists a key reference, not key material in JSON responses, and the runtime
  stores key files under the data directory for future remote deployment work.
