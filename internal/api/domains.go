package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/6space7/porter/internal/proxy"
	"github.com/go-chi/chi/v5"
)

type DomainService interface {
	AddCustomDomain(ctx context.Context, appID, hostname string) (DomainResponse, error)
	ListDomains(ctx context.Context, appID string) ([]DomainResponse, error)
}

type DomainResponse struct {
	ID       string `json:"id"`
	AppID    string `json:"app_id"`
	Hostname string `json:"hostname"`
	Type     string `json:"type"`
	Verified bool   `json:"verified"`
}

type domainHandler struct {
	domains DomainService
}

func mountDomainRoutes(router chi.Router, domains DomainService) {
	handler := domainHandler{domains: domains}
	router.With(RequireScope("apps:read")).Get("/apps/{appID}/domains", handler.list)
	router.With(RequireScope("apps:write")).Post("/apps/{appID}/domains", handler.create)
}

func (handler domainHandler) list(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	if !validID(appID) {
		WriteError(w, http.StatusBadRequest, "invalid_app_id", "App id is invalid.", "Use a valid app id returned by the API.", map[string]any{"field": "app_id"})
		return
	}

	domains, err := handler.domains.ListDomains(r.Context(), appID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "Domains could not be loaded.", "Try again or check server logs.", nil)
		return
	}
	writeJSON(w, http.StatusOK, domains)
}

func (handler domainHandler) create(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	if !validID(appID) {
		WriteError(w, http.StatusBadRequest, "invalid_app_id", "App id is invalid.", "Use a valid app id returned by the API.", map[string]any{"field": "app_id"})
		return
	}

	var input struct {
		Hostname string `json:"hostname"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.", "Send a JSON object with a hostname field.", nil)
		return
	}
	if err := ValidateDomainName(input.Hostname); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_domain", "Domain is invalid.", "Use a lowercase fully qualified domain name.", map[string]any{"field": "hostname"})
		return
	}

	domain, err := handler.domains.AddCustomDomain(r.Context(), appID, input.Hostname)
	if err != nil {
		var preflightErr *proxy.DNSPreflightError
		if errors.As(err, &preflightErr) {
			WriteError(w, http.StatusBadRequest, "dns_preflight_failed", "Domain DNS does not point at this server.", "Create the required A record and retry verification.", map[string]any{
				"hostname":          preflightErr.Hostname,
				"required_a_record": preflightErr.RequiredARecord,
				"current_records":   preflightErr.CurrentRecords,
			})
			return
		}
		WriteError(w, http.StatusInternalServerError, "internal_error", "Domain could not be added.", "Try again or check server logs.", nil)
		return
	}
	writeJSON(w, http.StatusCreated, domain)
}
