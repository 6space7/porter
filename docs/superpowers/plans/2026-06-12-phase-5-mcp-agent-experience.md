# Phase 5 MCP Agent Experience Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an authenticated MCP HTTP server, machine-readable docs, and an onboarding UI so AI agents can operate porter without using the web dashboard.

**Architecture:** Mount a mark3labs streamable HTTP MCP server from the same Go binary under `/api/v1/mcp`. Tool handlers reuse the existing API service interfaces so the JSON API remains the product source of truth. The frontend onboarding page calls the existing token API, displays scoped MCP config snippets, and links to platform-served `llms.txt` and JSON docs.

**Tech Stack:** Go, chi, `github.com/mark3labs/mcp-go`, SQLite/sqlc-backed services, Svelte 5, Vite, TypeScript.

---

### Task 1: MCP Server Package

**Files:**
- Create: `internal/mcp/server.go`
- Create: `internal/mcp/tools.go`
- Create: `internal/mcp/results.go`
- Create: `internal/mcp/server_test.go`
- Modify: `go.mod`, `go.sum`

- [x] **Step 1: Add failing tests for tool registration**

Create `internal/mcp/server_test.go` with a test that builds an MCP handler from fake app/project/service/log dependencies, initializes it through an HTTP test server, and confirms `tools/list` includes:

```text
porter_list_apps
porter_list_projects
porter_create_project
porter_create_app
porter_deploy_app
porter_list_deployments
porter_get_build_log
porter_get_runtime_logs
porter_list_env_vars
porter_set_env_var
porter_rollback_app
porter_search_service_templates
porter_deploy_service
porter_attach_service
porter_diagnose_latest_deployment
```

Run:

```bash
go test ./internal/mcp -run TestServerListsPorterTools -count=1
```

Expected: fails because `internal/mcp` does not exist.

- [x] **Step 2: Add dependency and minimal server**

Run:

```bash
go get github.com/mark3labs/mcp-go@v0.54.1
```

Implement `NewServer(deps Dependencies) *server.MCPServer` using:

```go
server.NewMCPServer("porter", "0.5.0", server.WithToolCapabilities(false), server.WithRecovery())
```

Add each tool with `mcp.NewTool` and a stub handler returning `mcp.NewToolResultError("not implemented")`.

- [x] **Step 3: Verify registration passes**

Run:

```bash
go test ./internal/mcp -run TestServerListsPorterTools -count=1
```

Expected: PASS.

### Task 2: MCP Authentication and Mounting

**Files:**
- Create: `internal/api/mcp.go`
- Modify: `internal/api/server.go`
- Modify: `internal/runtime/server.go`
- Create: `internal/api/mcp_test.go`

- [x] **Step 1: Add failing auth tests**

Create tests proving:

```text
POST /api/v1/mcp without Authorization returns 401
POST /api/v1/mcp with a read token can initialize and list read tools
POST /api/v1/mcp with a token missing services:write cannot call porter_deploy_service
```

Run:

```bash
go test ./internal/api -run TestMCPRoutesRequireBearerAuth -count=1
```

Expected: FAIL because the route is not mounted.

- [x] **Step 2: Mount MCP handler behind existing auth**

Add `MCP http.Handler` to `api.Dependencies`. In `NewRouterWithDeps`, mount it inside the protected group at `/mcp/*` after `RequireAuth`. Use token scopes inside MCP handlers by reading the existing principal from request context and passing it into the MCP request context.

- [x] **Step 3: Wire runtime dependencies**

In `runtime.NewHandlerWithOptions`, build an MCP server with the same store-backed services already used by the API and mount `server.NewStreamableHTTPServer(mcpServer, server.WithEndpointPath("/api/v1/mcp"))`.

- [x] **Step 4: Verify auth tests**

Run:

```bash
go test ./internal/api ./internal/runtime ./internal/mcp -count=1
```

Expected: PASS.

### Task 3: Implement MCP Tool Behavior

**Files:**
- Modify: `internal/mcp/tools.go`
- Modify: `internal/mcp/results.go`
- Modify: `internal/mcp/server_test.go`

- [x] **Step 1: Add failing tool behavior tests**

Add tests for:

```text
porter_list_apps returns JSON app list
porter_list_projects and porter_create_project return JSON project responses
porter_create_app validates required fields and creates an app
porter_deploy_app invokes deploy service and returns deployment JSON
porter_get_build_log returns build log JSON
porter_list_env_vars and porter_set_env_var mask secret values
porter_search_service_templates filters templates
porter_deploy_service requires services:write and returns service plus one-time credentials
porter_attach_service writes service env vars to an app
porter_diagnose_latest_deployment summarizes latest failed deployment stage, log tail, and next hints
```

Run:

```bash
go test ./internal/mcp -count=1
```

Expected: FAIL on stubbed tool handlers.

- [x] **Step 2: Implement typed argument helpers**

Use `request.RequireString`, `request.GetString`, and `request.GetBool` helpers where available. Return `mcp.NewToolResultError(...)` for validation failures rather than Go errors so agents receive machine-readable tool failures.

- [x] **Step 3: Implement JSON text results**

Marshal API responses to indented JSON and return them with `mcp.NewToolResultText`. Masked API responses stay masked because the MCP package uses existing service interfaces.

- [x] **Step 4: Verify tool behavior tests**

Run:

```bash
go test ./internal/mcp -count=1
```

Expected: PASS.

### Task 4: Machine-Readable Docs

**Files:**
- Create: `internal/docs/handler.go`
- Create: `internal/docs/handler_test.go`
- Modify: `internal/runtime/server.go`
- Modify: `README.md`

- [x] **Step 1: Add failing docs tests**

Test:

```text
GET /llms.txt returns text/plain and mentions /api/v1/mcp
GET /api/v1/docs returns application/json with api_base, mcp_endpoint, auth, tools, and examples
```

Run:

```bash
go test ./internal/docs ./internal/runtime -count=1
```

Expected: FAIL because docs package does not exist.

- [x] **Step 2: Implement static docs handler**

Serve concise content generated from a fixed `Docs` struct. Include:

```text
Authorization: Bearer <porter token>
MCP endpoint: https://<platform-domain>/api/v1/mcp
Core JSON API base: /api/v1
Tool names and required scopes
```

- [x] **Step 3: Mount before SPA fallback**

Wrap docs, API, MCP, and frontend under one top-level handler so `/llms.txt` is not swallowed by the SPA.

- [x] **Step 4: Verify docs tests**

Run:

```bash
go test ./internal/docs ./internal/runtime -count=1
```

Expected: PASS.

### Task 5: Agent Onboarding UI

**Files:**
- Modify: `frontend/src/components/SettingsPanel.svelte`
- Modify: `frontend/src/lib/api.ts`
- Modify: `frontend/src/lib/types.ts`
- Modify: `frontend/src/app.css`

- [x] **Step 1: Add TypeScript/API support**

Add `docs()` and `createToken("agent", scopes)` usage. The settings panel should generate a scoped token with:

```text
projects:read
projects:write
apps:read
apps:write
apps:deploy
services:read
services:write
```

- [x] **Step 2: Add onboarding panel**

In Settings, add a "Connect your AI agent" section with:

```text
MCP endpoint
Generate agent token button
Claude Code JSON snippet
Cursor JSON snippet
Link to /llms.txt
```

Only show the plaintext token once after creation.

- [x] **Step 3: Verify frontend**

Run:

```bash
cd frontend
npm run check
npm run build
cd ..
go test ./internal/frontend ./internal/runtime -count=1
```

Expected: PASS.

### Task 6: Phase 5 Verification

**Files:**
- Modify: `README.md`
- Modify: `ARCHITECTURE.md`
- Modify: `DECISIONS.md`

- [x] **Step 1: Full local verification**

Run:

```bash
npm --prefix frontend run check
npm --prefix frontend run build
go test ./... -count=1
go build ./cmd/server
bash -n install.sh
git diff --check
```

Expected: all commands exit 0.

- [x] **Step 2: VPS verification**

On the test VPS:

```text
git pull --ff-only
go test ./... -count=1
bash install.sh
call MCP initialize/tools/list over /api/v1/mcp with a bearer token
call MCP deploy service with a token missing services:write and confirm forbidden/tool error
open Settings and confirm onboarding snippet renders
```

- [x] **Step 3: Commit and push**

Use small conventional commits during implementation and a final docs commit:

```bash
git push
```

Expected: `main` and the VPS checkout point at the final Phase 5 commit.

---

## Self-Review

- Spec coverage: covers MCP HTTP transport, scoped auth, API-mirroring tools, failure diagnosis, platform docs, and UI onboarding from Phase 5.
- Placeholder scan: no TBD/TODO placeholders remain.
- Type consistency: uses existing API service names and mark3labs `mcp`/`server` packages consistently.
