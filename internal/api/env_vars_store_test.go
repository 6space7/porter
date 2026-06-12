package api_test

import (
	"context"
	"strings"
	"testing"

	"github.com/6space7/porter/internal/api"
	secretcrypto "github.com/6space7/porter/internal/crypto"
	"github.com/6space7/porter/internal/store"
)

func TestStoreEnvVarServiceEncryptsSecretValuesAtRest(t *testing.T) {
	ctx := context.Background()
	queries, closeDB := setupAppForEnvTest(t, ctx)
	defer closeDB()

	key, err := secretcrypto.GenerateMasterKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	box, err := secretcrypto.NewSecretBox(key)
	if err != nil {
		t.Fatalf("new secret box: %v", err)
	}

	service := api.NewStoreEnvVarService(queries, box)
	envVar, err := service.SetEnvVar(ctx, "app_1", api.SetEnvVarInput{
		Key:      "DATABASE_URL",
		Value:    "postgres://porter:secret@db:5432/app",
		IsSecret: true,
	})
	if err != nil {
		t.Fatalf("set env var: %v", err)
	}
	if envVar.Value != "postgres://porter:secret@db:5432/app" || !envVar.IsSecret {
		t.Fatalf("env var = %#v", envVar)
	}

	rows, err := queries.ListEnvVarsByApp(ctx, "app_1")
	if err != nil {
		t.Fatalf("list raw env vars: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %#v", rows)
	}
	if strings.Contains(rows[0].Value, "postgres://porter:secret@db:5432/app") {
		t.Fatalf("raw stored value leaked plaintext: %q", rows[0].Value)
	}

	listed, err := service.ListEnvVars(ctx, "app_1")
	if err != nil {
		t.Fatalf("list env vars: %v", err)
	}
	if len(listed) != 1 || listed[0].Value != "postgres://porter:secret@db:5432/app" {
		t.Fatalf("listed env vars = %#v", listed)
	}
}

func setupAppForEnvTest(t *testing.T, ctx context.Context) (*store.Queries, func()) {
	t.Helper()

	db, err := store.Open(ctx, store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	queries := store.New(db.SQL())
	_, err = queries.CreateProject(ctx, store.CreateProjectParams{ID: "proj_1", Name: "demo"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	_, err = queries.CreateApp(ctx, store.CreateAppParams{
		ID:           "app_1",
		ProjectID:    "proj_1",
		ServerID:     "local",
		Name:         "web",
		GitUrl:       "https://github.com/example/web.git",
		Branch:       "main",
		BuildType:    "dockerfile",
		InternalPort: 3000,
		Status:       "created",
	})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}

	return queries, func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	}
}
