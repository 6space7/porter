package runtime

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	uifrontend "github.com/6space7/porter/frontend"
	"github.com/6space7/porter/internal/api"
	"github.com/6space7/porter/internal/auth"
	"github.com/6space7/porter/internal/config"
	secretcrypto "github.com/6space7/porter/internal/crypto"
	"github.com/6space7/porter/internal/deploy"
	dockerstage "github.com/6space7/porter/internal/docker"
	frontendhandler "github.com/6space7/porter/internal/frontend"
	portermcp "github.com/6space7/porter/internal/mcp"
	"github.com/6space7/porter/internal/proxy"
	"github.com/6space7/porter/internal/services"
	"github.com/6space7/porter/internal/store"
	servicetemplates "github.com/6space7/porter/templates"
	mcpsdkserver "github.com/mark3labs/mcp-go/server"
)

type Options struct {
	Resolver       proxy.Resolver
	SecretBox      *secretcrypto.SecretBox
	Cloner         deploy.Cloner
	Builder        deploy.Builder
	Runner         deploy.Runner
	AppRuntime     api.AppRuntime
	RuntimeLogs    api.RuntimeLogStreamer
	ImagePruner    api.ImagePruner
	ServiceRuntime api.ServiceRuntime
	CaddyRuntime   proxy.CaddyRuntime
	CaddyAdmin     proxy.CaddyAdmin
}

func NewHandler(ctx context.Context, cfg config.Config) (*store.DB, http.Handler, error) {
	return NewHandlerWithOptions(ctx, cfg, Options{})
}

func NewHandlerWithOptions(ctx context.Context, cfg config.Config, opts Options) (*store.DB, http.Handler, error) {
	db, err := store.Open(ctx, store.Config{Path: cfg.DatabasePath})
	if err != nil {
		return nil, nil, err
	}

	queries := store.New(db.SQL())
	secretBox := opts.SecretBox
	if secretBox == nil && cfg.MasterKeyPath != "" {
		secretBox, err = secretcrypto.LoadSecretBox(cfg.MasterKeyPath)
		if err != nil {
			_ = db.Close()
			return nil, nil, err
		}
	}
	envVars := api.NewStoreEnvVarService(queries, secretBox)
	serviceCatalog, err := services.LoadCatalog(servicetemplates.FS())
	if err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	defaultStages, err := defaultDeploymentStages(cfg)
	if err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	if err := bootstrapAdminToken(ctx, queries, cfg.BootstrapTokenHash); err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	if err := bootstrapAdminUser(ctx, queries, cfg.BootstrapAdminEmail, cfg.BootstrapAdminPasswordFile); err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	caddyAdmin := opts.CaddyAdmin
	if cfg.ManageCaddy {
		caddyRuntime := opts.CaddyRuntime
		if caddyRuntime == nil {
			caddyRuntime, err = proxy.NewDockerCaddyRuntime()
			if err != nil {
				_ = db.Close()
				return nil, nil, err
			}
		}
		if err := (proxy.CaddyManager{Runtime: caddyRuntime}).Ensure(ctx); err != nil {
			_ = db.Close()
			return nil, nil, err
		}
		if caddyAdmin == nil {
			caddyAdmin = proxy.CaddyAdminClient{}
		}
	}
	var routeUpdater api.RouteUpdater
	if caddyAdmin != nil {
		reconciler := proxy.Reconciler{
			Source: routeSource(queries, cfg),
			Admin:  caddyAdmin,
			AskURL: cfg.CaddyAskURL,
		}
		if err := reconciler.Reconcile(ctx); err != nil {
			_ = db.Close()
			return nil, nil, err
		}
		routeUpdater = reconciler
	}
	pipeline := deploy.Pipeline{
		Store:        deploy.NewStoreDeploymentStore(queries, nil),
		Cloner:       chooseCloner(opts.Cloner, defaultStages.Cloner),
		PortDetector: deploy.DockerfilePortDetector{},
		Builder:      chooseBuilder(opts.Builder, defaultStages.Builder),
		Runner:       chooseRunner(opts.Runner, defaultStages.Runner),
	}
	caddyAsk := api.NewStoreCaddyAskService(queries)
	if strings.TrimSpace(cfg.PlatformDomain) != "" {
		caddyAsk = api.NewStaticDomainCaddyAskService(caddyAsk, cfg.PlatformDomain)
	}
	apiHandler := api.NewRouterWithDeps(api.Dependencies{
		Auth:          api.NewStoreAuthService(queries),
		TokenVerifier: api.NewStoreTokenVerifier(queries),
		Projects:      api.NewStoreProjectService(queries, nil),
		Apps: api.NewStoreAppServiceWithOptions(queries, api.StoreAppServiceOptions{
			PublicIP:     cfg.PublicIP,
			RouteUpdater: routeUpdater,
			Runtime:      chooseAppRuntime(opts.AppRuntime, defaultStages.AppRuntime),
		}),
		Domains: api.NewStoreDomainService(queries, api.StoreDomainServiceOptions{
			Resolver:     opts.Resolver,
			ServerIP:     cfg.PublicIP,
			RouteUpdater: routeUpdater,
		}),
		EnvVars: envVars,
		Deployments: api.NewStoreDeploymentServiceWithOptions(queries, pipeline, envVars, api.StoreDeploymentServiceOptions{
			RouteUpdater: routeUpdater,
			ImagePruner:  chooseImagePruner(opts.ImagePruner, defaultStages.ImagePruner),
		}),
		Logs:     api.NewStoreLogService(queries, chooseRuntimeLogs(opts.RuntimeLogs, defaultStages.RuntimeLogs)),
		CaddyAsk: caddyAsk,
		Services: api.NewStoreServiceManagerWithOptions(queries, serviceCatalog, envVars, secretBox, api.StoreServiceManagerOptions{
			Runtime:      chooseServiceRuntime(opts.ServiceRuntime, defaultStages.ServiceRuntime),
			RouteUpdater: routeUpdater,
			PublicIP:     cfg.PublicIP,
		}),
		MCP: mcpsdkserver.NewStreamableHTTPServer(portermcp.NewServer(portermcp.Dependencies{})),
	})
	handler := frontendhandler.NewHandler(apiHandler, uifrontend.Dist())
	return db, handler, nil
}

func routeSource(queries *store.Queries, cfg config.Config) proxy.RouteSource {
	source := proxy.NewStoreRouteSource(queries)
	if strings.TrimSpace(cfg.PlatformDomain) == "" {
		return source
	}
	return platformRouteSource{
		next:     source,
		hostname: cfg.PlatformDomain,
		upstream: defaultString(cfg.PlatformUpstream, "host.docker.internal:8080"),
	}
}

type platformRouteSource struct {
	next     proxy.RouteSource
	hostname string
	upstream string
}

func (source platformRouteSource) ListRoutes(ctx context.Context) ([]proxy.Route, error) {
	routes, err := source.next.ListRoutes(ctx)
	if err != nil {
		return nil, err
	}
	host, port, err := splitUpstream(source.upstream)
	if err != nil {
		return nil, err
	}
	return append(routes, proxy.Route{
		Hostname:      source.hostname,
		ContainerName: host,
		InternalPort:  port,
	}), nil
}

func splitUpstream(upstream string) (string, int64, error) {
	host, portString, err := net.SplitHostPort(upstream)
	if err != nil {
		return "", 0, fmt.Errorf("platform upstream must be host:port: %w", err)
	}
	port, err := strconv.ParseInt(portString, 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("platform upstream port is invalid: %w", err)
	}
	return host, port, nil
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func bootstrapAdminUser(ctx context.Context, queries *store.Queries, email, passwordFile string) error {
	email = strings.TrimSpace(email)
	passwordFile = strings.TrimSpace(passwordFile)
	if passwordFile == "" {
		return nil
	}
	if email == "" {
		return fmt.Errorf("bootstrap admin email is required")
	}
	if _, err := queries.GetUserByEmail(ctx, email); err == nil {
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	raw, err := os.ReadFile(passwordFile)
	if err != nil {
		return fmt.Errorf("read bootstrap admin password: %w", err)
	}
	password := strings.TrimSpace(string(raw))
	if password == "" {
		return fmt.Errorf("bootstrap admin password is empty")
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash bootstrap admin password: %w", err)
	}
	_, err = queries.CreateUser(ctx, store.CreateUserParams{
		ID:           "usr_admin",
		Email:        email,
		PasswordHash: hash,
	})
	if err != nil {
		return fmt.Errorf("create bootstrap admin user: %w", err)
	}
	return nil
}

type deploymentStages struct {
	Cloner         deploy.Cloner
	Builder        deploy.Builder
	Runner         deploy.Runner
	AppRuntime     api.AppRuntime
	RuntimeLogs    api.RuntimeLogStreamer
	ImagePruner    api.ImagePruner
	ServiceRuntime api.ServiceRuntime
}

func defaultDeploymentStages(cfg config.Config) (deploymentStages, error) {
	backend, err := dockerstage.NewSDKBackend()
	if err != nil {
		return deploymentStages{}, err
	}
	return deploymentStages{
		Cloner:         deploy.GitCloner{Root: cfg.WorkspacePath},
		Builder:        dockerstage.Builder{Images: backend, Nixpacks: dockerstage.NixpacksCLI{}},
		Runner:         dockerstage.Runner{Containers: backend},
		AppRuntime:     dockerstage.AppController{Containers: backend},
		RuntimeLogs:    dockerstage.RuntimeLogs{Containers: backend},
		ImagePruner:    backend,
		ServiceRuntime: dockerstage.ServiceRunner{Backend: backend},
	}, nil
}

func chooseCloner(override deploy.Cloner, fallback deploy.Cloner) deploy.Cloner {
	if override != nil {
		return override
	}
	return fallback
}

func chooseBuilder(override deploy.Builder, fallback deploy.Builder) deploy.Builder {
	if override != nil {
		return override
	}
	return fallback
}

func chooseRunner(override deploy.Runner, fallback deploy.Runner) deploy.Runner {
	if override != nil {
		return override
	}
	return fallback
}

func chooseAppRuntime(override api.AppRuntime, fallback api.AppRuntime) api.AppRuntime {
	if override != nil {
		return override
	}
	return fallback
}

func chooseRuntimeLogs(override api.RuntimeLogStreamer, fallback api.RuntimeLogStreamer) api.RuntimeLogStreamer {
	if override != nil {
		return override
	}
	return fallback
}

func chooseImagePruner(override api.ImagePruner, fallback api.ImagePruner) api.ImagePruner {
	if override != nil {
		return override
	}
	return fallback
}

func chooseServiceRuntime(override api.ServiceRuntime, fallback api.ServiceRuntime) api.ServiceRuntime {
	if override != nil {
		return override
	}
	return fallback
}

func bootstrapAdminToken(ctx context.Context, queries *store.Queries, hash string) error {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return nil
	}
	if len(hash) != 64 {
		return fmt.Errorf("bootstrap token hash must be a sha256 hex digest")
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return fmt.Errorf("bootstrap token hash must be a sha256 hex digest")
	}
	if _, err := queries.GetTokenByHash(ctx, hash); err == nil {
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	_, err := queries.CreateToken(ctx, store.CreateTokenParams{
		ID:     "tok_bootstrap",
		Name:   "bootstrap",
		Hash:   hash,
		Scopes: "projects:read,projects:write,apps:read,apps:write,apps:deploy,services:read,services:write",
	})
	if err != nil {
		return fmt.Errorf("create bootstrap token: %w", err)
	}
	return nil
}
