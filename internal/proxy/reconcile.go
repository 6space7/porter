package proxy

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	containerNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)
	hostnameLabelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
)

type Route struct {
	Hostname      string
	ContainerName string
	InternalPort  int64
}

type RouteSource interface {
	ListRoutes(ctx context.Context) ([]Route, error)
}

type RouteSourceFunc func(ctx context.Context) ([]Route, error)

func (fn RouteSourceFunc) ListRoutes(ctx context.Context) ([]Route, error) {
	return fn(ctx)
}

type CaddyAdmin interface {
	ApplyConfig(ctx context.Context, config CaddyConfig) error
}

type CaddyConfig struct {
	HTTP CaddyHTTPConfig `json:"http"`
}

type CaddyHTTPConfig struct {
	AskURL string       `json:"ask_url"`
	Routes []CaddyRoute `json:"routes"`
}

type CaddyRoute struct {
	Hostname     string `json:"hostname"`
	UpstreamDial string `json:"upstream_dial"`
}

type Reconciler struct {
	Source RouteSource
	Admin  CaddyAdmin
	AskURL string
}

func (reconciler Reconciler) Reconcile(ctx context.Context) error {
	if reconciler.Source == nil {
		return fmt.Errorf("route source is required")
	}
	if reconciler.Admin == nil {
		return fmt.Errorf("caddy admin is required")
	}

	routes, err := reconciler.Source.ListRoutes(ctx)
	if err != nil {
		return err
	}
	config, err := BuildCaddyConfig(routes, reconciler.AskURL)
	if err != nil {
		return err
	}
	return reconciler.Admin.ApplyConfig(ctx, config)
}

func BuildCaddyConfig(routes []Route, askURL string) (CaddyConfig, error) {
	caddyRoutes := make([]CaddyRoute, 0, len(routes))
	for _, route := range routes {
		if err := validateRoute(route); err != nil {
			return CaddyConfig{}, err
		}
		caddyRoutes = append(caddyRoutes, CaddyRoute{
			Hostname:     route.Hostname,
			UpstreamDial: fmt.Sprintf("%s:%d", route.ContainerName, route.InternalPort),
		})
	}
	sort.Slice(caddyRoutes, func(i, j int) bool {
		return caddyRoutes[i].Hostname < caddyRoutes[j].Hostname
	})

	return CaddyConfig{
		HTTP: CaddyHTTPConfig{
			AskURL: askURL,
			Routes: caddyRoutes,
		},
	}, nil
}

func validateRoute(route Route) error {
	if err := validateHostname(route.Hostname); err != nil {
		return err
	}
	if !containerNamePattern.MatchString(route.ContainerName) {
		return fmt.Errorf("container name is invalid")
	}
	if route.InternalPort < 1 || route.InternalPort > 65535 {
		return fmt.Errorf("internal port is invalid")
	}
	return nil
}

func validateHostname(hostname string) error {
	if len(hostname) == 0 || len(hostname) > 253 {
		return fmt.Errorf("hostname length is invalid")
	}
	if hostname != strings.ToLower(hostname) || strings.HasPrefix(hostname, ".") || strings.HasSuffix(hostname, ".") || !strings.Contains(hostname, ".") {
		return fmt.Errorf("hostname is invalid")
	}
	for _, label := range strings.Split(hostname, ".") {
		if !hostnameLabelPattern.MatchString(label) {
			return fmt.Errorf("hostname label is invalid")
		}
	}
	return nil
}
