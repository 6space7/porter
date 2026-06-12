package runtime

import (
	"context"
	"net/http"

	"github.com/6space7/porter/internal/api"
	"github.com/6space7/porter/internal/config"
	secretcrypto "github.com/6space7/porter/internal/crypto"
	"github.com/6space7/porter/internal/deploy"
	dockerstage "github.com/6space7/porter/internal/docker"
	"github.com/6space7/porter/internal/proxy"
	"github.com/6space7/porter/internal/store"
)

type Options struct {
	Resolver     proxy.Resolver
	SecretBox    *secretcrypto.SecretBox
	Cloner       deploy.Cloner
	Builder      deploy.Builder
	Runner       deploy.Runner
	RuntimeLogs  api.RuntimeLogStreamer
	CaddyRuntime proxy.CaddyRuntime
	CaddyAdmin   proxy.CaddyAdmin
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
	envVars := api.NewStoreEnvVarService(queries, opts.SecretBox)
	defaultStages, err := defaultDeploymentStages(cfg)
	if err != nil {
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
	if caddyAdmin != nil {
		reconciler := proxy.Reconciler{
			Source: proxy.NewStoreRouteSource(queries),
			Admin:  caddyAdmin,
			AskURL: cfg.CaddyAskURL,
		}
		if err := reconciler.Reconcile(ctx); err != nil {
			_ = db.Close()
			return nil, nil, err
		}
	}
	pipeline := deploy.Pipeline{
		Store:   deploy.NewStoreDeploymentStore(queries, nil),
		Cloner:  chooseCloner(opts.Cloner, defaultStages.Cloner),
		Builder: chooseBuilder(opts.Builder, defaultStages.Builder),
		Runner:  chooseRunner(opts.Runner, defaultStages.Runner),
	}
	handler := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: api.NewStoreTokenVerifier(queries),
		Projects:      api.NewStoreProjectService(queries, nil),
		Apps: api.NewStoreAppServiceWithOptions(queries, api.StoreAppServiceOptions{
			PublicIP: cfg.PublicIP,
		}),
		Domains: api.NewStoreDomainService(queries, api.StoreDomainServiceOptions{
			Resolver: opts.Resolver,
			ServerIP: cfg.PublicIP,
		}),
		EnvVars:     envVars,
		Deployments: api.NewStoreDeploymentService(queries, pipeline, envVars),
		Logs:        api.NewStoreLogService(queries, chooseRuntimeLogs(opts.RuntimeLogs, defaultStages.RuntimeLogs)),
		CaddyAsk:    api.NewStoreCaddyAskService(queries),
	})
	return db, handler, nil
}

type deploymentStages struct {
	Cloner      deploy.Cloner
	Builder     deploy.Builder
	Runner      deploy.Runner
	RuntimeLogs api.RuntimeLogStreamer
}

func defaultDeploymentStages(cfg config.Config) (deploymentStages, error) {
	backend, err := dockerstage.NewSDKBackend()
	if err != nil {
		return deploymentStages{}, err
	}
	return deploymentStages{
		Cloner:      deploy.GitCloner{Root: cfg.WorkspacePath},
		Builder:     dockerstage.Builder{Images: backend},
		Runner:      dockerstage.Runner{Containers: backend},
		RuntimeLogs: dockerstage.RuntimeLogs{Containers: backend},
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

func chooseRuntimeLogs(override api.RuntimeLogStreamer, fallback api.RuntimeLogStreamer) api.RuntimeLogStreamer {
	if override != nil {
		return override
	}
	return fallback
}
