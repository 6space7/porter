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
