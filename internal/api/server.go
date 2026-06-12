package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Dependencies struct {
	Auth          AuthService
	TokenVerifier TokenVerifier
	Projects      ProjectService
	Apps          AppService
	Domains       DomainService
	EnvVars       EnvVarService
	Deployments   DeploymentService
	Logs          LogService
	CaddyAsk      CaddyAskService
}

func NewRouter() http.Handler {
	return NewRouterWithDeps(Dependencies{})
}

func NewRouterWithDeps(deps Dependencies) http.Handler {
	router := chi.NewRouter()
	router.Get("/health", HealthHandler)
	if deps.CaddyAsk != nil {
		mountCaddyAskRoutes(router, deps.CaddyAsk)
	}

	router.Route("/api/v1", func(r chi.Router) {
		if deps.Auth != nil {
			mountAuthRoutes(r, deps.Auth)
		}
		if deps.TokenVerifier != nil {
			r.Group(func(protected chi.Router) {
				protected.Use(RequireBearerToken(deps.TokenVerifier))
				if deps.Projects != nil {
					mountProjectRoutes(protected, deps.Projects)
				}
				if deps.Apps != nil {
					mountAppRoutes(protected, deps.Apps)
				}
				if deps.Domains != nil {
					mountDomainRoutes(protected, deps.Domains)
				}
				if deps.EnvVars != nil {
					mountEnvVarRoutes(protected, deps.EnvVars)
				}
				if deps.Deployments != nil {
					mountDeploymentRoutes(protected, deps.Deployments)
				}
				if deps.Logs != nil {
					mountLogRoutes(protected, deps.Logs)
				}
			})
		}
	})

	return router
}
