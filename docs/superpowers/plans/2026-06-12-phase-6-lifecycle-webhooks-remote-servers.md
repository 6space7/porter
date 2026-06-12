# Phase 6 Lifecycle, Webhooks, and Remote Servers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish the backend lifecycle phase with release installs/updates, signed Git webhooks, and the server registry/SSH path needed for multi-server deployments.

**Architecture:** Ship Phase 6 in three layers. First make installs and upgrades release-aware and reversible with DB backups. Then add HMAC-verified push webhooks that call the existing deployment service. Finally add a server registry with SSH validation and app/service server selection, keeping local deploys as the default and isolating remote execution behind interfaces so it can be verified with fakes and a second VPS.

**Tech Stack:** Go, chi, SQLite/sqlc, Svelte 5, GitHub Actions, shell installer scripts, `golang.org/x/crypto/ssh`, HMAC-SHA256 webhooks.

---

### Task 1: Release Lifecycle CLI and Backups

**Files:**
- Create: `internal/lifecycle/backup.go`
- Create: `internal/lifecycle/backup_test.go`
- Create: `internal/lifecycle/update.go`
- Create: `internal/lifecycle/update_test.go`
- Modify: `cmd/server/main.go`
- Modify: `README.md`

- [x] **Step 1: Add failing backup tests**

Create `internal/lifecycle/backup_test.go` with tests for:

```go
func TestBackupSQLiteCopiesDatabaseWithTimestamp(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "porter.db")
	if err := os.WriteFile(dbPath, []byte("sqlite-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := lifecycle.BackupSQLite(context.Background(), lifecycle.BackupOptions{
		DatabasePath: dbPath,
		BackupDir:    filepath.Join(dir, "backups"),
		Now:          func() time.Time { return time.Date(2026, 6, 12, 15, 30, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(result.Path) != "porter-20260612-153000.db" {
		t.Fatalf("backup path = %s", result.Path)
	}
	body, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "sqlite-bytes" {
		t.Fatalf("backup body = %q", string(body))
	}
}
```

Run:

```bash
go test ./internal/lifecycle -run TestBackupSQLiteCopiesDatabaseWithTimestamp -count=1
```

Expected: FAIL because `internal/lifecycle` does not exist.

- [x] **Step 2: Implement backup helper**

Implement `BackupSQLite(ctx, opts)` to:

```text
validate DatabasePath and BackupDir are non-empty
create BackupDir with 0700
copy the database to porter-YYYYMMDD-HHMMSS.db with 0600
return BackupResult{Path, Bytes}
```

Use `os.Open`, `os.OpenFile` with `O_CREATE|O_EXCL|O_WRONLY`, and `io.Copy`.

- [x] **Step 3: Add failing update planner tests**

Create `internal/lifecycle/update_test.go` proving:

```go
func TestPlanUpdateBuildsReleaseURL(t *testing.T) {
	plan, err := lifecycle.PlanUpdate(lifecycle.UpdateOptions{
		Repo:    "6space7/porter",
		Version: "v1.2.3",
		GOOS:    "linux",
		GOARCH:  "amd64",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "https://github.com/6space7/porter/releases/download/v1.2.3/porter-linux-amd64.tar.gz"
	if plan.URL != want {
		t.Fatalf("url = %q", plan.URL)
	}
}
```

Run:

```bash
go test ./internal/lifecycle -count=1
```

Expected: FAIL until `PlanUpdate` exists.

- [x] **Step 4: Implement update planning and CLI commands**

Add:

```go
type UpdateOptions struct {
	Repo, Version, GOOS, GOARCH string
}
type UpdatePlan struct {
	URL string
	ArchiveName string
}
func PlanUpdate(opts UpdateOptions) (UpdatePlan, error)
```

Modify `cmd/server/main.go` so:

```text
porter backup --database /path/porter.db --backup-dir /path/backups
porter update --version vX.Y.Z --repo 6space7/porter
```

parse flags, run a DB backup before update planning, and print the planned release URL. Keep actual binary replacement for Task 2 after installer/release assets exist.

- [x] **Step 5: Verify and commit**

Run:

```bash
go test ./internal/lifecycle ./cmd/server -count=1
go build ./cmd/server
```

Commit:

```bash
git add cmd/server internal/lifecycle README.md
git commit -m "feat: add lifecycle backup planning"
```

### Task 2: Release Workflow, Release Installer, and Uninstall

**Files:**
- Create: `.github/workflows/release.yml`
- Create: `uninstall.sh`
- Modify: `install.sh`
- Modify: `README.md`
- Modify: `DECISIONS.md`

- [x] **Step 1: Add installer shell tests**

Add shell syntax checks to local verification commands:

```bash
bash -n install.sh
bash -n uninstall.sh
```

Expected: `uninstall.sh` missing until implemented.

- [x] **Step 2: Add release workflow**

Create `.github/workflows/release.yml` that runs on tags `v*`, builds:

```text
linux/amd64 -> porter-linux-amd64.tar.gz
linux/arm64 -> porter-linux-arm64.tar.gz
```

Each archive must contain `porter`, `install.sh`, `uninstall.sh`, `README.md`, `ARCHITECTURE.md`, and `DECISIONS.md`. Use `actions/setup-go`, `actions/setup-node`, `npm ci`, `npm run build`, and `go build`.

- [x] **Step 3: Upgrade installer release download path**

Modify `install.sh` so default install downloads:

```text
https://github.com/${PORTER_REPO:-6space7/porter}/releases/latest/download/porter-linux-${arch}.tar.gz
```

and extracts `porter` into `/usr/local/bin/porter`. Preserve source-build mode with:

```bash
PORTER_INSTALL_FROM_SOURCE=1 sudo ./install.sh
```

Keep Go install only for source mode. Keep Nixpacks, Docker, env, systemd, and one-time password behavior unchanged.

- [x] **Step 4: Add uninstall script**

Create `uninstall.sh` with:

```text
--purge removes /etc/porter and /var/lib/porter
default stops/disables porter.service and removes /usr/local/bin/porter only
never removes user app/service containers unless --purge is provided
```

Use root checks and explicit path variables.

- [x] **Step 5: Verify and commit**

Run:

```bash
bash -n install.sh
bash -n uninstall.sh
go test ./... -count=1
```

Commit:

```bash
git add .github/workflows/release.yml install.sh uninstall.sh README.md DECISIONS.md
git commit -m "feat: add release installer lifecycle"
```

### Task 3: HMAC-Verified Push Webhooks

**Files:**
- Create: `internal/api/webhooks.go`
- Create: `internal/api/webhooks_test.go`
- Modify: `internal/api/server.go`
- Modify: `internal/api/apps.go`
- Modify: `internal/api/apps_store.go`
- Modify: `internal/store/migrations/00003_webhooks.sql`
- Modify: `internal/store/queries.sql`
- Regenerate: `internal/store/queries.sql.go`, `internal/store/db.go`, `internal/store/models.go`
- Modify: `frontend/src/components/AppDetail.svelte`
- Modify: `frontend/src/lib/api.ts`
- Modify: `frontend/src/lib/types.ts`

- [x] **Step 1: Add webhook schema**

Add migration:

```sql
alter table apps add column auto_deploy_branch text not null default '';
alter table apps add column webhook_secret text not null default '';
```

Add queries:

```sql
-- name: UpdateAppWebhook :one
update apps
set auto_deploy_branch = ?, webhook_secret = ?, updated_at = current_timestamp
where id = ?
returning id, project_id, server_id, name, git_url, branch, build_type, internal_port, status, auto_deploy_branch, webhook_secret, created_at, updated_at;

-- name: GetAppByWebhookID :one
select id, project_id, server_id, name, git_url, branch, build_type, internal_port, status, auto_deploy_branch, webhook_secret, created_at, updated_at
from apps
where id = ?;
```

Run sqlc through:

```bash
go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate
```

- [x] **Step 2: Add failing webhook tests**

Create tests proving:

```text
POST /api/v1/webhooks/github/{appID} without X-Hub-Signature-256 returns 401
wrong signature returns 401
push to non-matching branch returns 202 with skipped=true and does not deploy
push to matching branch deploys once
```

Use a fake `DeploymentService` and a known payload:

```json
{"ref":"refs/heads/main","repository":{"clone_url":"https://github.com/example/app.git"}}
```

- [x] **Step 3: Implement webhook handler**

Mount the webhook route outside bearer auth but under `/api/v1/webhooks/github/{appID}`. Load the app, require a non-empty `webhook_secret`, verify:

```text
X-Hub-Signature-256 == "sha256=" + hex(hmac_sha256(secret, raw_body))
```

using constant-time comparison, parse `ref`, compare it to `auto_deploy_branch`, then call `DeployApp`.

- [x] **Step 4: Add API/UI webhook settings**

Expose authenticated endpoint:

```text
PUT /api/v1/apps/{appID}/webhook
body: {"branch":"main","enabled":true}
response: {"webhook_url":"...","secret":"shown-once","branch":"main","enabled":true}
```

Store a new generated secret when enabling. Clear branch/secret when disabling. Add App Detail UI controls to enable/disable and show webhook URL + secret once.

- [x] **Step 5: Verify and commit**

Run:

```bash
go test ./internal/store ./internal/api ./internal/runtime -count=1
cd frontend && npm run check && npm run build
cd ..
go test ./internal/frontend ./internal/runtime -count=1
```

Commit:

```bash
git add internal/api internal/store frontend README.md
git commit -m "feat: add signed git webhooks"
```

### Task 4: Server Registry and SSH Validation

**Files:**
- Create: `internal/remote/ssh.go`
- Create: `internal/remote/ssh_test.go`
- Create: `internal/api/servers.go`
- Create: `internal/api/servers_store.go`
- Create: `internal/api/servers_test.go`
- Modify: `internal/api/server.go`
- Modify: `internal/store/queries.sql`
- Regenerate: `internal/store/queries.sql.go`, `internal/store/db.go`
- Modify: `frontend/src/components/SettingsPanel.svelte`
- Modify: `frontend/src/lib/api.ts`
- Modify: `frontend/src/lib/types.ts`

- [x] **Step 1: Add store queries**

Add:

```sql
-- name: CreateServer :one
insert into servers (id, name, host, ssh_key_ref, status)
values (?, ?, ?, ?, ?)
returning id, name, host, ssh_key_ref, status, created_at, updated_at;

-- name: ListServers :many
select id, name, host, ssh_key_ref, status, created_at, updated_at
from servers
order by created_at asc, name asc;

-- name: UpdateServerStatus :one
update servers
set status = ?, updated_at = current_timestamp
where id = ?
returning id, name, host, ssh_key_ref, status, created_at, updated_at;
```

- [x] **Step 2: Add SSH validation abstraction**

Implement `remote.Validator`:

```go
type CheckRequest struct {
	Host string
	User string
	PrivateKeyPEM []byte
}
type CheckResult struct {
	DockerVersion string `json:"docker_version"`
	OS            string `json:"os"`
}
type Validator interface {
	Check(ctx context.Context, req CheckRequest) (CheckResult, error)
}
```

Production validator uses `golang.org/x/crypto/ssh`, runs `uname -a` and `docker --version`, and returns a typed error when Docker is missing.

- [x] **Step 3: Add server API tests**

Test:

```text
GET /api/v1/servers requires servers:read
POST /api/v1/servers requires servers:write
POST validates host/user/private key are non-empty
POST calls Validator.Check and stores status=healthy
```

Add `servers:read` and `servers:write` to `AdminScopes`.

- [x] **Step 4: Implement server API and settings UI**

Add settings form fields:

```text
name
host
ssh user
private key
```

Submit to `POST /servers`. Show servers list with status. Keep private key write-only and never echo it back.

- [x] **Step 5: Verify and commit**

Run:

```bash
go test ./internal/remote ./internal/api ./internal/store ./internal/runtime -count=1
cd frontend && npm run check && npm run build
cd ..
go test ./internal/frontend ./internal/runtime -count=1
```

Commit:

```bash
git add internal/remote internal/api internal/store frontend README.md
git commit -m "feat: add ssh server registry"
```

### Task 5: App and Service Server Selection

**Files:**
- Modify: `internal/api/apps.go`
- Modify: `internal/api/apps_store.go`
- Modify: `internal/api/services.go`
- Modify: `internal/api/services_store.go`
- Modify: `internal/deploy/pipeline.go`
- Modify: `frontend/src/components/CreateAppPanel.svelte`
- Modify: `frontend/src/components/ServicesPanel.svelte`
- Modify: `frontend/src/lib/types.ts`

- [ ] **Step 1: Add failing API tests**

Test:

```text
POST /api/v1/apps accepts server_id and defaults to srv_local
POST /api/v1/services accepts server_id and defaults to srv_local
invalid server_id returns a structured error
```

- [ ] **Step 2: Wire server_id through app/service creation**

Add `ServerID` to `CreateAppInput` and `CreateServiceInput`. Default blank input to `srv_local`. Validate the server exists before insert.

- [ ] **Step 3: Add frontend selectors**

Load servers in `App.svelte`, pass them to create/services panels, and add a native select. Default selected server is `srv_local`.

- [ ] **Step 4: Verify and commit**

Run:

```bash
go test ./internal/api ./internal/runtime -count=1
cd frontend && npm run check && npm run build
cd ..
go test ./internal/frontend ./internal/runtime -count=1
```

Commit:

```bash
git add internal/api internal/deploy frontend README.md
git commit -m "feat: select deployment server"
```

### Task 6: Phase 6 Verification

**Files:**
- Modify: `README.md`
- Modify: `ARCHITECTURE.md`
- Modify: `DECISIONS.md`
- Modify: this plan file

- [ ] **Step 1: Full local verification**

Run:

```bash
cd frontend && npm run check && npm run build
cd ..
go test ./... -count=1
go build ./cmd/server
bash -n install.sh
bash -n uninstall.sh
```

- [ ] **Step 2: VPS verification**

On the current VPS:

```text
git pull --ff-only
go test ./... -count=1
go build -o /usr/local/bin/porter ./cmd/server
systemctl restart porter
curl /health
enable webhook for a test app and send one invalid and one valid HMAC payload
run porter backup against the production DB and verify a backup file exists
```

For true multi-server verification, obtain a second clean VPS and:

```text
add it through Settings -> Servers
create an app targeting that server
verify generated HTTPS route on that second server
```

- [ ] **Step 3: Commit and push**

Commit docs and verification updates:

```bash
git add README.md ARCHITECTURE.md DECISIONS.md docs/superpowers/plans/2026-06-12-phase-6-lifecycle-webhooks-remote-servers.md
git commit -m "docs: mark phase 6 verified"
git push origin main
```

---

## Open Verification Need

True Phase 6 multi-server verification needs a second VPS with root SSH access. Until that is available, implementation can be completed and verified with unit/integration tests plus SSH validation against the existing test VPS, but the "deploy an app to a second VPS" acceptance test remains blocked.
