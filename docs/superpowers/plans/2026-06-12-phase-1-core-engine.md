# Phase 1 Core Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build porter Phase 1: a secure single-server API and deployment engine that can deploy a public Git repository with a Dockerfile, route it through Caddy on an sslip.io domain, expose logs, enforce auth/scopes, and install as a basic systemd service.

**Architecture:** Build a single Go binary with focused internal packages. The API is the only product surface; Docker, Caddy, auth, storage, and deployment logic are behind narrow interfaces so later phases can add UI, MCP, services, Nixpacks, and remote servers without changing endpoint contracts.

**Tech Stack:** Go, `chi`, SQLite through `modernc.org/sqlite`, `goose`, `sqlc`, Docker SDK, Caddy admin API, WebSockets, bcrypt, AES-GCM, `go test`.

---

## Scope Boundaries

Phase 1 includes:

- repository skeleton, config, SQLite migrations, health endpoint;
- JSON error envelope for all API failures;
- admin login and bearer token auth with scopes;
- project/app/domain/env/deployment APIs;
- deploy pipeline stages: clone, build Docker image, start container, record status;
- secret encryption, masking, and log redaction;
- Caddy container lifecycle and DB-to-Caddy reconciliation;
- generated sslip.io domains and custom-domain DNS preflight;
- WebSocket runtime log streaming and stored build logs;
- basic `install.sh`;
- tests for deploy state machine and auth/scopes.

Phase 1 excludes:

- Svelte UI;
- rollback;
- service catalog;
- Nixpacks;
- MCP server;
- remote servers;
- webhooks and release downloads.

## File Map

- Create `go.mod` and `go.sum`: module definition and dependencies.
- Create `cmd/server/main.go`: binary entrypoint and command wiring.
- Create `internal/config/config.go`: environment and default path parsing.
- Create `internal/api/server.go`: router setup, middleware order, versioned API mounting.
- Create `internal/api/errors.go`: JSON error envelope helpers.
- Create `internal/api/health.go`: health endpoint.
- Create `internal/api/auth.go`: login, logout, token creation, auth middleware.
- Create `internal/api/projects.go`: project CRUD handlers.
- Create `internal/api/apps.go`: app CRUD and lifecycle handlers.
- Create `internal/api/domains.go`: generated/custom domain handlers and Caddy ask endpoint.
- Create `internal/api/env_vars.go`: env var handlers with masking.
- Create `internal/api/deployments.go`: deployment list, deploy trigger, build-log retrieval.
- Create `internal/api/logs.go`: WebSocket runtime logs.
- Create `internal/api/validation.go`: request validation helpers.
- Create `internal/auth/passwords.go`: password hashing.
- Create `internal/auth/tokens.go`: token creation, hashing, constant-time checks, scope matching.
- Create `internal/auth/sessions.go`: secure session cookie helpers.
- Create `internal/crypto/secrets.go`: master-key loading, AES-GCM encryption/decryption.
- Create `internal/store/db.go`: SQLite connection and migration bootstrap.
- Create `internal/store/queries.sql`: sqlc query definitions.
- Create `internal/store/models.go`: app-facing model helpers around generated sqlc types.
- Create `internal/store/migrations/00001_initial.sql`: Phase 1 schema.
- Create `internal/deploy/pipeline.go`: deploy orchestration and stage transitions.
- Create `internal/deploy/stages.go`: clone/build/run stage implementations.
- Create `internal/deploy/git.go`: Git URL validation and safe clone command.
- Create `internal/deploy/logs.go`: secret redaction and build-log storage adapter.
- Create `internal/docker/client.go`: Docker SDK wrapper.
- Create `internal/docker/build.go`: image build.
- Create `internal/docker/container.go`: container start/stop/restart/delete/logs.
- Create `internal/docker/network.go`: per-app network management.
- Create `internal/proxy/caddy.go`: Caddy admin API client.
- Create `internal/proxy/lifecycle.go`: managed Caddy container lifecycle.
- Create `internal/proxy/reconcile.go`: DB route reconciliation.
- Create `internal/proxy/domains.go`: sslip.io generation and custom-domain DNS preflight.
- Create `internal/install/paths.go`: installer/runtime path constants shared by binary and docs.
- Create `install.sh`: basic source-build installer with Docker, master key, systemd unit, and one-time admin password.
- Create `sqlc.yaml`: sqlc configuration.
- Create `Makefile`: common `test`, `build`, `generate`, and `migrate` commands.
- Modify `README.md`: Phase 1 usage and curl path.
- Modify `ARCHITECTURE.md`: Phase 1 package and request-flow details.
- Modify `DECISIONS.md`: decisions made while implementing Phase 1.

## Task 1: Go Module And API Error Foundation

**Files:**
- Create: `go.mod`
- Create: `cmd/server/main.go`
- Create: `internal/config/config.go`
- Create: `internal/api/errors.go`
- Create: `internal/api/server.go`
- Create: `internal/api/health.go`
- Test: `internal/api/errors_test.go`
- Test: `internal/api/health_test.go`

- [ ] **Step 1: Write the failing error-envelope test**

```go
func TestWriteErrorUsesMachineLegibleEnvelope(t *testing.T) {
	rr := httptest.NewRecorder()
	api.WriteError(rr, http.StatusBadRequest, "invalid_name", "Name is invalid.", "Use lowercase letters, numbers, and hyphens.", map[string]any{"field": "name"})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var body struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Hint    string         `json:"hint"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Error.Code != "invalid_name" || body.Error.Hint == "" || body.Error.Details["field"] != "name" {
		t.Fatalf("unexpected error body: %#v", body.Error)
	}
}
```

- [ ] **Step 2: Run the test to verify RED**

Run: `go test ./internal/api -run TestWriteErrorUsesMachineLegibleEnvelope -count=1`

Expected: FAIL because the module/package does not exist yet.

- [ ] **Step 3: Implement minimal module, server, health, and error helpers**

Create a Go module, define `api.ErrorBody`, `api.WriteError`, `api.NewRouter`, and `api.HealthHandler`. Keep the code small and do not add auth, storage, Docker, or Caddy behavior in this task.

- [ ] **Step 4: Run GREEN verification**

Run: `go test ./internal/api -count=1`

Expected: PASS.

- [ ] **Step 5: Commit and push**

```bash
git add go.mod go.sum cmd/server/main.go internal/config internal/api
git commit -m "feat: add api foundation"
git push
```

## Task 2: SQLite Schema And Store Bootstrap

**Files:**
- Create: `internal/store/migrations/00001_initial.sql`
- Create: `internal/store/db.go`
- Create: `internal/store/queries.sql`
- Create: `internal/store/models.go`
- Create: `sqlc.yaml`
- Modify: `go.mod`
- Test: `internal/store/db_test.go`
- Test: `internal/store/schema_test.go`

- [ ] **Step 1: Write failing schema tests**

Test that an in-memory SQLite database migrates successfully and exposes `servers`, `projects`, `apps`, `domains`, `deployments`, `env_vars`, `services`, `tokens`, and `users`.

- [ ] **Step 2: Run RED verification**

Run: `go test ./internal/store -run 'TestMigrate|TestInitialSchema' -count=1`

Expected: FAIL because the store package and migrations are missing.

- [ ] **Step 3: Implement migration bootstrap**

Use `database/sql`, `modernc.org/sqlite`, and embedded migration files. Add a default local server row during initialization using an idempotent statement.

- [ ] **Step 4: Add initial queries**

Define sqlc queries for users, tokens, projects, apps, domains, deployments, env vars, and server lookup. Use static SQL only.

- [ ] **Step 5: Generate code and verify**

Run: `go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate`

Run: `go test ./internal/store -count=1`

Expected: PASS.

- [ ] **Step 6: Commit and push**

```bash
git add go.mod go.sum sqlc.yaml internal/store
git commit -m "feat: add sqlite store"
git push
```

## Task 3: Auth, Sessions, Tokens, And Scopes

**Files:**
- Create: `internal/auth/passwords.go`
- Create: `internal/auth/tokens.go`
- Create: `internal/auth/sessions.go`
- Modify: `internal/api/server.go`
- Create: `internal/api/auth.go`
- Test: `internal/auth/tokens_test.go`
- Test: `internal/api/auth_test.go`

- [ ] **Step 1: Write failing token tests**

Test that generated API tokens are shown once, only SHA-256 hashes are stored, token comparison is constant-time, and scope checks reject missing scopes.

- [ ] **Step 2: Write failing API auth/scope tests**

Use protected test routes requiring `apps:read` and `apps:deploy`; verify unauthenticated requests return 401, a read token can read, and the same token cannot deploy.

- [ ] **Step 3: Run RED verification**

Run: `go test ./internal/auth ./internal/api -run 'TestToken|TestScope|TestProtected' -count=1`

Expected: FAIL because auth code is missing.

- [ ] **Step 4: Implement minimal auth**

Use bcrypt for admin passwords, SHA-256 token hashes, constant-time hash comparison, bearer auth middleware, secure session cookie creation, and scope middleware.

- [ ] **Step 5: Run GREEN verification**

Run: `go test ./internal/auth ./internal/api -count=1`

Expected: PASS.

- [ ] **Step 6: Commit and push**

```bash
git add go.mod go.sum internal/auth internal/api
git commit -m "feat: add scoped api auth"
git push
```

## Task 4: Secret Encryption, Masking, And Log Redaction

**Files:**
- Create: `internal/crypto/secrets.go`
- Create: `internal/deploy/logs.go`
- Modify: `internal/api/env_vars.go`
- Test: `internal/crypto/secrets_test.go`
- Test: `internal/deploy/logs_test.go`

- [ ] **Step 1: Write failing encryption and masking tests**

Test AES-GCM round trips with a master key, ciphertext differs from plaintext, secret env vars serialize as `••••`, and known secret values are redacted from build logs.

- [ ] **Step 2: Run RED verification**

Run: `go test ./internal/crypto ./internal/deploy -run 'TestEncrypt|TestRedact' -count=1`

Expected: FAIL because packages are missing.

- [ ] **Step 3: Implement secret helpers**

Use AES-GCM with random nonces. Load a hex or base64 master key from disk with permission checks documented in code comments and installer docs.

- [ ] **Step 4: Implement redaction**

Replace exact known secret values in logs with `[REDACTED]`. Skip empty values.

- [ ] **Step 5: Run GREEN verification**

Run: `go test ./internal/crypto ./internal/deploy -count=1`

Expected: PASS.

- [ ] **Step 6: Commit and push**

```bash
git add internal/crypto internal/deploy/logs.go internal/deploy/logs_test.go internal/api/env_vars.go
git commit -m "feat: protect secrets"
git push
```

## Task 5: Projects, Apps, Env Vars, Deployments, And Domains API

**Files:**
- Create: `internal/api/projects.go`
- Create: `internal/api/apps.go`
- Create: `internal/api/env_vars.go`
- Create: `internal/api/deployments.go`
- Create: `internal/api/domains.go`
- Create: `internal/api/validation.go`
- Modify: `internal/api/server.go`
- Test: `internal/api/projects_test.go`
- Test: `internal/api/apps_test.go`
- Test: `internal/api/env_vars_test.go`
- Test: `internal/api/domains_test.go`

- [ ] **Step 1: Write failing validation tests**

Test app names, domain names, branch names, build type enums, URL lengths, and JSON size limits. Include malicious Git URL payloads such as `file:///etc/passwd` and `--upload-pack=touch`.

- [ ] **Step 2: Write failing CRUD tests**

Use an in-memory store. Verify CRUD routes return JSON, require the right scopes, mask secret env vars, and return structured validation errors.

- [ ] **Step 3: Run RED verification**

Run: `go test ./internal/api -run 'TestProject|TestApp|TestEnv|TestDomain|TestValidation' -count=1`

Expected: FAIL because handlers are missing.

- [ ] **Step 4: Implement handlers**

Implement one handler group per file, use only store methods, and keep all errors in the standard envelope.

- [ ] **Step 5: Run GREEN verification**

Run: `go test ./internal/api -count=1`

Expected: PASS.

- [ ] **Step 6: Commit and push**

```bash
git add internal/api internal/store
git commit -m "feat: add phase one api resources"
git push
```

## Task 6: Docker Deployment Pipeline State Machine

**Files:**
- Create: `internal/deploy/pipeline.go`
- Create: `internal/deploy/stages.go`
- Create: `internal/deploy/git.go`
- Create: `internal/docker/client.go`
- Create: `internal/docker/build.go`
- Create: `internal/docker/container.go`
- Create: `internal/docker/network.go`
- Test: `internal/deploy/pipeline_test.go`
- Test: `internal/deploy/git_test.go`

- [ ] **Step 1: Write failing deploy state-machine tests**

Test success transitions `queued -> cloning -> building -> starting -> running`, failure records the failed stage and full redacted build log, and env var changes are read fresh on every deploy.

- [ ] **Step 2: Write failing Git URL validation tests**

Test `https://github.com/example/repo.git` and `git@github.com:example/repo.git` are accepted, while `file:///etc/passwd`, local paths, unsupported schemes, and argument-looking URLs are rejected.

- [ ] **Step 3: Run RED verification**

Run: `go test ./internal/deploy -run 'TestPipeline|TestGitURL' -count=1`

Expected: FAIL because deploy code is missing.

- [ ] **Step 4: Implement pipeline interfaces**

Define `Cloner`, `Builder`, `Runner`, `DeploymentStore`, and `SecretSource` interfaces so tests do not need Docker. Implement transition recording before each stage.

- [ ] **Step 5: Implement Docker-backed stages**

Use Docker SDK for build/run/network/log operations. Use `exec.Command` argument arrays for Git clone and never concatenate shell strings.

- [ ] **Step 6: Run GREEN verification**

Run: `go test ./internal/deploy ./internal/docker -count=1`

Expected: PASS or SKIP Docker integration tests when Docker is unavailable; unit tests must pass.

- [ ] **Step 7: Commit and push**

```bash
git add go.mod go.sum internal/deploy internal/docker
git commit -m "feat: add docker deployment pipeline"
git push
```

## Task 7: Caddy Lifecycle, Reconcile, sslip.io, And Custom Domains

**Files:**
- Create: `internal/proxy/caddy.go`
- Create: `internal/proxy/lifecycle.go`
- Create: `internal/proxy/reconcile.go`
- Create: `internal/proxy/domains.go`
- Modify: `internal/api/domains.go`
- Test: `internal/proxy/domains_test.go`
- Test: `internal/proxy/reconcile_test.go`

- [ ] **Step 1: Write failing sslip.io and preflight tests**

Test generated domains use the app name and public IP in sslip.io format. Test custom-domain preflight returns a structured error containing the required A record when DNS does not point to the server IP.

- [ ] **Step 2: Write failing Caddy reconcile tests**

Use a fake Caddy admin client and verify DB domains become routes to `container-name:internal-port`, with unknown domains rejected by the on-demand TLS ask path.

- [ ] **Step 3: Run RED verification**

Run: `go test ./internal/proxy ./internal/api -run 'TestSSLIP|TestPreflight|TestReconcile|TestCaddyAsk' -count=1`

Expected: FAIL because proxy code is missing.

- [ ] **Step 4: Implement proxy code**

Manage Caddy through the Docker wrapper, bind admin API to localhost, generate config from SQLite state, and expose only registered domains through the ask endpoint.

- [ ] **Step 5: Run GREEN verification**

Run: `go test ./internal/proxy ./internal/api -count=1`

Expected: PASS.

- [ ] **Step 6: Commit and push**

```bash
git add internal/proxy internal/api/domains.go
git commit -m "feat: manage caddy routing"
git push
```

## Task 8: Runtime Log WebSocket And Build Log API

**Files:**
- Create: `internal/api/logs.go`
- Modify: `internal/api/deployments.go`
- Modify: `internal/docker/container.go`
- Test: `internal/api/logs_test.go`
- Test: `internal/api/deployments_test.go`

- [ ] **Step 1: Write failing log tests**

Test build logs are returned for failed deployments with status and stage, runtime logs require `apps:read`, and WebSocket stream setup rejects unauthenticated callers.

- [ ] **Step 2: Run RED verification**

Run: `go test ./internal/api -run 'TestBuildLog|TestRuntimeLogs' -count=1`

Expected: FAIL because log endpoints are missing.

- [ ] **Step 3: Implement log endpoints**

Read stored build logs from SQLite, stream runtime logs from Docker over WebSocket, and apply redaction before writing log bytes to clients.

- [ ] **Step 4: Run GREEN verification**

Run: `go test ./internal/api -count=1`

Expected: PASS.

- [ ] **Step 5: Commit and push**

```bash
git add internal/api/logs.go internal/api/deployments.go internal/docker/container.go
git commit -m "feat: stream deployment logs"
git push
```

## Task 9: Installer, Runtime Bootstrap, And Docs

**Files:**
- Create: `install.sh`
- Create: `internal/install/paths.go`
- Modify: `cmd/server/main.go`
- Modify: `README.md`
- Modify: `ARCHITECTURE.md`
- Modify: `DECISIONS.md`
- Test: `internal/install/paths_test.go`

- [ ] **Step 1: Write failing path tests**

Test default config, data, database, and master-key paths are `/etc/porter`, `/var/lib/porter`, `/var/lib/porter/porter.db`, and `/etc/porter/master.key`.

- [ ] **Step 2: Run RED verification**

Run: `go test ./internal/install -run TestDefaultPaths -count=1`

Expected: FAIL because install paths are missing.

- [ ] **Step 3: Implement runtime bootstrap**

Wire config loading, DB migration, router startup, Caddy reconciliation, and admin bootstrap into `cmd/server/main.go`.

- [ ] **Step 4: Implement basic installer**

Create an idempotent Bash installer that checks root, checks Ubuntu/Debian and amd64/arm64, installs Docker if missing using the official Docker script, builds from source until release artifacts exist, writes a 0600 master key, creates a systemd unit, starts porter, and prints the dashboard URL and one-time admin password.

- [ ] **Step 5: Run verification**

Run: `go test ./... -count=1`

Run: `go build ./cmd/server`

Run: `bash -n install.sh`

Expected: all commands pass.

- [ ] **Step 6: Commit and push**

```bash
git add install.sh internal/install cmd/server/main.go README.md ARCHITECTURE.md DECISIONS.md
git commit -m "feat: add basic installer"
git push
```

## Task 10: Fresh VM Verification Gate

**Files:**
- Modify: `README.md`
- Modify: `DECISIONS.md`

- [ ] **Step 1: Run automated local verification**

Run: `go test ./... -count=1`

Run: `go build ./cmd/server`

Run: `bash -n install.sh`

Expected: all commands pass.

- [ ] **Step 2: Run manual VM verification**

On a fresh Ubuntu VM, run the documented `install.sh` command, deploy a public Dockerfile repo through curl, verify generated sslip.io HTTPS, test custom-domain DNS failure, test broken Dockerfile response with stage and build log, test scoped-token denial, test unauthenticated 401s, test secret masking, test malicious Git URL rejection, and confirm Caddy admin is not externally reachable.

- [ ] **Step 3: Document results**

Update `README.md` with the exact curl sequence and `DECISIONS.md` with any security or operational decisions discovered during VM testing.

- [ ] **Step 4: Commit and push**

```bash
git add README.md DECISIONS.md
git commit -m "docs: record phase one verification"
git push
```

## Self-Review

- Spec coverage: The plan covers the Phase 1 API, auth, store, deploy pipeline, Docker, Caddy, domains, logs, installer, docs, and verification gates. Later-phase work is intentionally excluded.
- Placeholder scan: No task contains a placeholder for behavior; implementation details are either specified directly or constrained by explicit files, tests, commands, and expected outcomes.
- Type consistency: The plan consistently separates API handlers, auth, store, deployment, Docker, proxy, crypto, and install packages.
- Risk: Phase 1 is large enough that each task must be committed and pushed independently. Do not combine tasks unless the diff remains small and tests verify the combined slice.
