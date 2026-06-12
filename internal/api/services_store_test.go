package api_test

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/6space7/porter/internal/api"
	secretcrypto "github.com/6space7/porter/internal/crypto"
	"github.com/6space7/porter/internal/services"
	"github.com/6space7/porter/internal/store"
)

func TestStoreServiceManagerCreatesServiceAndAttachesProvides(t *testing.T) {
	ctx := context.Background()
	queries, envVars, box, closeDB := setupServiceManagerStoreTest(t, ctx)
	defer closeDB()
	catalog := loadServiceTestCatalog(t)
	runtime := &fakeServiceRuntime{}
	manager := api.NewStoreServiceManagerWithOptions(queries, catalog, envVars, box, api.StoreServiceManagerOptions{
		Runtime:         runtime,
		NewServiceID:    func() string { return "svc_1" },
		SecretGenerator: services.StaticSecretGenerator("fixed-secret"),
	})

	created, err := manager.CreateService(ctx, api.CreateServiceInput{
		ProjectID:    "proj_1",
		TemplateSlug: "postgres",
		Name:         "db",
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	if created.Service.ID != "svc_1" || created.Service.Status != "running" || created.Service.Exposed {
		t.Fatalf("created service = %#v", created.Service)
	}
	if created.Credentials["POSTGRES_PASSWORD"] != "fixed-secret" {
		t.Fatalf("credentials = %#v", created.Credentials)
	}
	if created.Provides["DATABASE_URL"] != "postgres://porter:fixed-secret@porter-svc-svc_1:5432/app" {
		t.Fatalf("provides = %#v", created.Provides)
	}
	if runtime.request.Image != "postgres:16-alpine" || runtime.request.ContainerName != "porter-svc-svc_1" {
		t.Fatalf("runtime request = %#v", runtime.request)
	}
	if runtime.request.Env["POSTGRES_PASSWORD"] != "fixed-secret" || runtime.request.InternalPort != 5432 {
		t.Fatalf("runtime env/port = %#v", runtime.request)
	}

	row, err := queries.GetService(ctx, "svc_1")
	if err != nil {
		t.Fatalf("get stored service: %v", err)
	}
	if strings.Contains(row.GeneratedSecrets, "fixed-secret") {
		t.Fatalf("generated secrets stored in plaintext: %s", row.GeneratedSecrets)
	}

	attached, err := manager.AttachService(ctx, "svc_1", "app_1")
	if err != nil {
		t.Fatalf("attach service: %v", err)
	}
	if attached.Env["DATABASE_URL"] != "postgres://porter:fixed-secret@porter-svc-svc_1:5432/app" {
		t.Fatalf("attached = %#v", attached)
	}
	env, err := envVars.ListEnvVars(ctx, "app_1")
	if err != nil {
		t.Fatalf("list app env: %v", err)
	}
	if len(env) != 1 || env[0].Key != "DATABASE_URL" || env[0].Value != attached.Env["DATABASE_URL"] || !env[0].IsSecret {
		t.Fatalf("env vars = %#v", env)
	}
}

func TestStoreServiceManagerCreatesExposedServiceHostname(t *testing.T) {
	ctx := context.Background()
	queries, envVars, box, closeDB := setupServiceManagerStoreTest(t, ctx)
	defer closeDB()
	catalog := loadServiceTestCatalog(t)
	routeUpdater := &fakeRouteUpdater{}
	manager := api.NewStoreServiceManagerWithOptions(queries, catalog, envVars, box, api.StoreServiceManagerOptions{
		Runtime:         &fakeServiceRuntime{},
		NewServiceID:    func() string { return "svc_2" },
		SecretGenerator: services.StaticSecretGenerator("secret"),
		PublicIP:        "203.0.113.10",
		RouteUpdater:    routeUpdater,
	})

	created, err := manager.CreateService(ctx, api.CreateServiceInput{
		ProjectID:    "proj_1",
		TemplateSlug: "n8n",
		Name:         "flows",
		Exposed:      true,
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	if created.Service.Hostname != "flows.203-0-113-10.sslip.io" || !created.Service.Exposed {
		t.Fatalf("created service = %#v", created.Service)
	}
	if created.Provides["SERVICE_URL"] != "https://flows.203-0-113-10.sslip.io" {
		t.Fatalf("provides = %#v", created.Provides)
	}
	if routeUpdater.calls != 1 {
		t.Fatalf("route updater calls = %d, want 1", routeUpdater.calls)
	}
}

func setupServiceManagerStoreTest(t *testing.T, ctx context.Context) (*store.Queries, api.EnvVarService, *secretcrypto.SecretBox, func()) {
	t.Helper()

	db, err := store.Open(ctx, store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	queries := store.New(db.SQL())
	if _, err := queries.CreateProject(ctx, store.CreateProjectParams{ID: "proj_1", Name: "demo"}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := queries.CreateApp(ctx, store.CreateAppParams{
		ID:           "app_1",
		ProjectID:    "proj_1",
		ServerID:     "local",
		Name:         "web",
		GitUrl:       "https://github.com/example/web.git",
		Branch:       "main",
		BuildType:    "dockerfile",
		InternalPort: 3000,
		Status:       "created",
	}); err != nil {
		t.Fatalf("create app: %v", err)
	}
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	box, err := secretcrypto.NewSecretBox(key)
	if err != nil {
		t.Fatalf("new secret box: %v", err)
	}
	envVars := api.NewStoreEnvVarService(queries, box)
	return queries, envVars, box, func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	}
}

func loadServiceTestCatalog(t *testing.T) services.Catalog {
	t.Helper()
	catalog, err := services.LoadCatalog(fstest.MapFS{
		"postgres.yaml": {Data: []byte(`
slug: postgres
name: PostgreSQL
image: postgres:16-alpine
internal_port: 5432
variables:
  POSTGRES_DB: app
  POSTGRES_USER: porter
  POSTGRES_PASSWORD: ${SERVICE_PASSWORD_POSTGRES}
provides:
  DATABASE_URL: postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${SERVICE_HOST}:5432/${POSTGRES_DB}
`)},
		"n8n.yaml": {Data: []byte(`
slug: n8n
name: n8n
image: n8nio/n8n:latest
internal_port: 5678
exposed: true
variables:
  N8N_HOST: ${SERVICE_DOMAIN}
  WEBHOOK_URL: ${SERVICE_URL}
provides:
  SERVICE_URL: ${SERVICE_URL}
`)},
	})
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	return catalog
}

type fakeServiceRuntime struct {
	request services.DeployRequest
}

func (runtime *fakeServiceRuntime) DeployService(_ context.Context, req services.DeployRequest) (string, error) {
	runtime.request = req
	return "service started\n", nil
}
