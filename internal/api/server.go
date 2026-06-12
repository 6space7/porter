package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Dependencies struct {
	TokenVerifier TokenVerifier
	Projects      ProjectService
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
		})
	}

	return router
}
