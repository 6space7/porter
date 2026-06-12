package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Dependencies struct {
	TokenVerifier TokenVerifier
	Projects      ProjectService
	Apps          AppService
	Domains       DomainService
	EnvVars       EnvVarService
	Deployments   DeploymentService
}

func NewRouter() http.Handler {
	return NewRouterWithDeps(Dependencies{})
}

func NewRouterWithDeps(deps Dependencies) http.Handler {
	router := chi.NewRouter()
	router.Get("/health", HealthHandler)

	if deps.TokenVerifier != nil {
		router.Route("/api/v1", func(r chi.Router) {
			r.Use(RequireBearerToken(deps.TokenVerifier))
			if deps.Projects != nil {
				mountProjectRoutes(r, deps.Projects)
			}
			if deps.Apps != nil {
				mountAppRoutes(r, deps.Apps)
			}
			if deps.Domains != nil {
				mountDomainRoutes(r, deps.Domains)
			}
			if deps.EnvVars != nil {
				mountEnvVarRoutes(r, deps.EnvVars)
			}
			if deps.Deployments != nil {
				mountDeploymentRoutes(r, deps.Deployments)
			}
		})
	}

	return router
}
