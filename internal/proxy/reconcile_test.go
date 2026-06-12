package proxy_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/6space7/porter/internal/proxy"
)

func TestReconcilerAppliesRoutesToCaddyConfig(t *testing.T) {
	source := proxy.RouteSourceFunc(func(context.Context) ([]proxy.Route, error) {
		return []proxy.Route{
			{Hostname: "web.example.com", ContainerName: "porter-app_web", InternalPort: 3000},
			{Hostname: "api.example.com", ContainerName: "porter-app_api", InternalPort: 8080},
		}, nil
	})
	admin := &fakeCaddyAdmin{}
	reconciler := proxy.Reconciler{
		Source: source,
		Admin:  admin,
		AskURL: "http://127.0.0.1:8080/api/v1/caddy/ask",
	}

	if err := reconciler.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if !admin.called {
		t.Fatal("admin was not called")
	}
	gotHosts := []string{
		admin.config.HTTP.Routes[0].Hostname,
		admin.config.HTTP.Routes[1].Hostname,
	}
	if !reflect.DeepEqual(gotHosts, []string{"api.example.com", "web.example.com"}) {
		t.Fatalf("hosts = %#v", gotHosts)
	}
	if admin.config.HTTP.Routes[0].UpstreamDial != "porter-app_api:8080" {
		t.Fatalf("upstream = %q", admin.config.HTTP.Routes[0].UpstreamDial)
	}
	if admin.config.HTTP.AskURL != "http://127.0.0.1:8080/api/v1/caddy/ask" {
		t.Fatalf("ask url = %q", admin.config.HTTP.AskURL)
	}
}

func TestReconcilerRejectsInvalidRoutesBeforeApplying(t *testing.T) {
	source := proxy.RouteSourceFunc(func(context.Context) ([]proxy.Route, error) {
		return []proxy.Route{
			{Hostname: "bad host", ContainerName: "porter-app_web", InternalPort: 3000},
		}, nil
	})
	admin := &fakeCaddyAdmin{}
	reconciler := proxy.Reconciler{
		Source: source,
		Admin:  admin,
		AskURL: "http://127.0.0.1:8080/api/v1/caddy/ask",
	}

	if err := reconciler.Reconcile(context.Background()); err == nil {
		t.Fatal("expected invalid route to fail")
	}
	if admin.called {
		t.Fatal("admin should not be called after invalid route")
	}
}

type fakeCaddyAdmin struct {
	called bool
	config proxy.CaddyConfig
}

func (admin *fakeCaddyAdmin) ApplyConfig(_ context.Context, config proxy.CaddyConfig) error {
	admin.called = true
	admin.config = config
	return nil
}
