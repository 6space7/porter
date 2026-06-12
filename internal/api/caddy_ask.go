package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type CaddyAskService interface {
	IsDomainAllowed(ctx context.Context, hostname string) (bool, error)
}

type caddyAskHandler struct {
	service CaddyAskService
}

type staticDomainCaddyAskService struct {
	next      CaddyAskService
	hostnames map[string]struct{}
}

func NewStaticDomainCaddyAskService(next CaddyAskService, hostnames ...string) CaddyAskService {
	allowed := make(map[string]struct{}, len(hostnames))
	for _, hostname := range hostnames {
		hostname = strings.ToLower(strings.TrimSpace(hostname))
		if hostname != "" {
			allowed[hostname] = struct{}{}
		}
	}
	return staticDomainCaddyAskService{next: next, hostnames: allowed}
}

func (service staticDomainCaddyAskService) IsDomainAllowed(ctx context.Context, hostname string) (bool, error) {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if _, ok := service.hostnames[hostname]; ok {
		return true, nil
	}
	if service.next == nil {
		return false, nil
	}
	return service.next.IsDomainAllowed(ctx, hostname)
}

func mountCaddyAskRoutes(router chi.Router, service CaddyAskService) {
	handler := caddyAskHandler{service: service}
	router.Get("/api/v1/caddy/ask", handler.ask)
}

func (handler caddyAskHandler) ask(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("domain")
	if hostname == "" {
		hostname = r.URL.Query().Get("host")
	}
	if err := ValidateDomainName(hostname); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_domain", "Domain is invalid.", "Send a lowercase fully qualified domain name.", map[string]any{"field": "domain"})
		return
	}

	allowed, err := handler.service.IsDomainAllowed(r.Context(), hostname)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "Domain approval could not be checked.", "Try again or check server logs.", nil)
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "domain_not_allowed", "Domain is not registered in porter.", "Add and verify this domain before requesting TLS.", map[string]any{"hostname": hostname})
		return
	}
	w.WriteHeader(http.StatusOK)
}
