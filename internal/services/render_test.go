package services_test

import (
	"reflect"
	"testing"

	"github.com/6space7/porter/internal/services"
)

func TestRenderTemplateGeneratesSecretsAndProvidesConnectionValues(t *testing.T) {
	tmpl := services.Template{
		Slug:         "postgres",
		Name:         "PostgreSQL",
		Image:        "postgres:16-alpine",
		InternalPort: 5432,
		Variables: map[string]string{
			"POSTGRES_DB":       "app",
			"POSTGRES_USER":     "porter",
			"POSTGRES_PASSWORD": "${SERVICE_PASSWORD_POSTGRES}",
		},
		Provides: map[string]string{
			"DATABASE_URL": "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${SERVICE_HOST}:5432/${POSTGRES_DB}",
		},
	}

	rendered, err := services.RenderTemplate(tmpl, services.RenderInput{
		ServiceID:     "svc_1",
		ContainerName: "porter-svc-svc_1",
	}, services.SecretGeneratorFunc(func(label string) (string, error) {
		return "secret-for-" + label, nil
	}))
	if err != nil {
		t.Fatalf("render template: %v", err)
	}

	wantEnv := map[string]string{
		"POSTGRES_DB":       "app",
		"POSTGRES_USER":     "porter",
		"POSTGRES_PASSWORD": "secret-for-POSTGRES",
	}
	if !reflect.DeepEqual(rendered.Env, wantEnv) {
		t.Fatalf("env = %#v, want %#v", rendered.Env, wantEnv)
	}
	if rendered.Provides["DATABASE_URL"] != "postgres://porter:secret-for-POSTGRES@porter-svc-svc_1:5432/app" {
		t.Fatalf("provides = %#v", rendered.Provides)
	}
	if rendered.Generated["POSTGRES_PASSWORD"] != "secret-for-POSTGRES" {
		t.Fatalf("generated = %#v", rendered.Generated)
	}
}

func TestRenderTemplateExpandsPublicServiceURL(t *testing.T) {
	tmpl := services.Template{
		Slug:         "n8n",
		Name:         "n8n",
		Image:        "n8nio/n8n:latest",
		InternalPort: 5678,
		Exposed:      true,
		Variables: map[string]string{
			"N8N_HOST":    "${SERVICE_DOMAIN}",
			"WEBHOOK_URL": "${SERVICE_URL}",
		},
		Provides: map[string]string{
			"SERVICE_URL": "${SERVICE_URL}",
		},
	}

	rendered, err := services.RenderTemplate(tmpl, services.RenderInput{
		ServiceID:     "svc_1",
		ContainerName: "porter-svc-svc_1",
		Hostname:      "n8n.203-0-113-10.sslip.io",
	}, services.StaticSecretGenerator("unused"))
	if err != nil {
		t.Fatalf("render template: %v", err)
	}

	if rendered.Env["N8N_HOST"] != "n8n.203-0-113-10.sslip.io" || rendered.Env["WEBHOOK_URL"] != "https://n8n.203-0-113-10.sslip.io" {
		t.Fatalf("env = %#v", rendered.Env)
	}
	if rendered.Provides["SERVICE_URL"] != "https://n8n.203-0-113-10.sslip.io" {
		t.Fatalf("provides = %#v", rendered.Provides)
	}
}
