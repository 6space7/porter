package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Dependencies struct {
	Auth                AuthService
	TokenVerifier       TokenVerifier
	LoginFailureLimiter FailureLimiter
	TokenFailureLimiter FailureLimiter
	Projects            ProjectService
	Apps                AppService
	Domains             DomainService
	EnvVars             EnvVarService
	Deployments         DeploymentService
	Logs                LogService
	CaddyAsk            CaddyAskService
	Services            ServiceManager
	Webhooks            AppWebhookService
	MCP                 http.Handler
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
		webhooks := deps.Webhooks
		if webhooks == nil && deps.Apps != nil {
			if appWebhooks, ok := deps.Apps.(AppWebhookService); ok {
				webhooks = appWebhooks
			}
		}
		if webhooks != nil && deps.Deployments != nil {
			mountWebhookRoutes(r, webhooks, deps.Deployments)
		}
		if deps.Auth != nil {
			loginLimiter := deps.LoginFailureLimiter
			if loginLimiter == nil {
				loginLimiter = NewDefaultFailureLimiter()
			}
			mountAuthRoutes(r, deps.Auth, loginLimiter)
		}
		if deps.TokenVerifier != nil {
			tokenLimiter := deps.TokenFailureLimiter
			if tokenLimiter == nil {
				tokenLimiter = NewDefaultFailureLimiter()
			}
			r.Group(func(protected chi.Router) {
				protected.Use(RequireAuth(deps.TokenVerifier, tokenLimiter))
				protected.Use(RequireCSRFForSessionAuth)
				if deps.Auth != nil {
					mountAuthSessionRoutes(protected, deps.Auth)
				}
				if deps.Projects != nil {
					mountProjectRoutes(protected, deps.Projects)
				}
				if deps.Apps != nil {
					mountAppRoutes(protected, deps.Apps, webhooks)
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
				if deps.Services != nil {
					mountServiceRoutes(protected, deps.Services)
				}
				if deps.MCP != nil {
					protected.Mount("/mcp", deps.MCP)
				}
			})
		}
	})

	return router
}
