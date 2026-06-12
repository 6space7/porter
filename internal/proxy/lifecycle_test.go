package proxy_test

import (
	"context"
	"testing"

	"github.com/6space7/porter/internal/proxy"
)

func TestCaddyManagerEnsuresDefaultCaddyContainer(t *testing.T) {
	runtime := &fakeCaddyRuntime{}
	manager := proxy.CaddyManager{Runtime: runtime}

	if err := manager.Ensure(context.Background()); err != nil {
		t.Fatalf("ensure caddy: %v", err)
	}

	spec := runtime.spec
	if spec.Name != "porter-caddy" {
		t.Fatalf("name = %q", spec.Name)
	}
	if spec.Image != "caddy:2-alpine" {
		t.Fatalf("image = %q", spec.Image)
	}
	if spec.HTTPPort != 80 || spec.HTTPSPort != 443 {
		t.Fatalf("ports = %d/%d", spec.HTTPPort, spec.HTTPSPort)
	}
	if spec.AdminHost != "127.0.0.1" || spec.AdminPort != 2019 {
		t.Fatalf("admin bind = %s:%d", spec.AdminHost, spec.AdminPort)
	}
	if spec.ConfigVolume != "porter-caddy-config" || spec.DataVolume != "porter-caddy-data" {
		t.Fatalf("volumes = %q/%q", spec.ConfigVolume, spec.DataVolume)
	}
	if spec.NetworkName != "porter-proxy" {
		t.Fatalf("network = %q", spec.NetworkName)
	}
}

func TestCaddyManagerRejectsNetworkExposedAdmin(t *testing.T) {
	runtime := &fakeCaddyRuntime{}
	manager := proxy.CaddyManager{
		Runtime: runtime,
		Spec: proxy.CaddyContainerSpec{
			Name:         "porter-caddy",
			Image:        "caddy:2-alpine",
			HTTPPort:     80,
			HTTPSPort:    443,
			AdminHost:    "0.0.0.0",
			AdminPort:    2019,
			ConfigVolume: "porter-caddy-config",
			DataVolume:   "porter-caddy-data",
			NetworkName:  "porter-proxy",
		},
	}

	if err := manager.Ensure(context.Background()); err == nil {
		t.Fatal("expected network-exposed admin host to fail")
	}
	if runtime.called {
		t.Fatal("runtime should not be called for invalid spec")
	}
}

type fakeCaddyRuntime struct {
	called bool
	spec   proxy.CaddyContainerSpec
}

func (runtime *fakeCaddyRuntime) EnsureCaddy(ctx context.Context, spec proxy.CaddyContainerSpec) error {
	runtime.called = true
	runtime.spec = spec
	return nil
}
