package runtime

import (
	"context"
	"net/http"

	"github.com/6space7/porter/internal/api"
	"github.com/6space7/porter/internal/config"
	secretcrypto "github.com/6space7/porter/internal/crypto"
	"github.com/6space7/porter/internal/deploy"
	"github.com/6space7/porter/internal/proxy"
	"github.com/6space7/porter/internal/store"
)

type Options struct {
	Resolver  proxy.Resolver
	SecretBox *secretcrypto.SecretBox
	Cloner    deploy.Cloner
	Builder   deploy.Builder
	Runner    deploy.Runner
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
	pipeline := deploy.Pipeline{
		Store:   deploy.NewStoreDeploymentStore(queries, nil),
		Cloner:  opts.Cloner,
		Builder: opts.Builder,
		Runner:  opts.Runner,
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
	})
	return db, handler, nil
}
