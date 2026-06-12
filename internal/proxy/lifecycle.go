package proxy

import (
	"context"
	"fmt"
)

type CaddyRuntime interface {
	EnsureCaddy(ctx context.Context, spec CaddyContainerSpec) error
}

type CaddyContainerSpec struct {
	Name         string
	Image        string
	HTTPPort     int
	HTTPSPort    int
	AdminHost    string
	AdminPort    int
	ConfigVolume string
	DataVolume   string
	NetworkName  string
}

type CaddyManager struct {
	Runtime CaddyRuntime
	Spec    CaddyContainerSpec
}

func (manager CaddyManager) Ensure(ctx context.Context) error {
	if manager.Runtime == nil {
		return fmt.Errorf("caddy runtime is required")
	}

	spec := manager.Spec
	if spec == (CaddyContainerSpec{}) {
		spec = DefaultCaddyContainerSpec()
	}
	if err := validateCaddySpec(spec); err != nil {
		return err
	}
	return manager.Runtime.EnsureCaddy(ctx, spec)
}

func DefaultCaddyContainerSpec() CaddyContainerSpec {
	return CaddyContainerSpec{
		Name:         "porter-caddy",
		Image:        "caddy:2-alpine",
		HTTPPort:     80,
		HTTPSPort:    443,
		AdminHost:    "127.0.0.1",
		AdminPort:    2019,
		ConfigVolume: "porter-caddy-config",
		DataVolume:   "porter-caddy-data",
		NetworkName:  "porter-proxy",
	}
}

func validateCaddySpec(spec CaddyContainerSpec) error {
	if spec.Name == "" || spec.Image == "" || spec.ConfigVolume == "" || spec.DataVolume == "" || spec.NetworkName == "" {
		return fmt.Errorf("caddy spec has missing required fields")
	}
	if spec.HTTPPort != 80 || spec.HTTPSPort != 443 {
		return fmt.Errorf("caddy must listen on host ports 80 and 443")
	}
	if spec.AdminHost != "127.0.0.1" && spec.AdminHost != "localhost" {
		return fmt.Errorf("caddy admin API must bind to localhost")
	}
	if spec.AdminPort != 2019 {
		return fmt.Errorf("caddy admin API must use port 2019")
	}
	return nil
}
