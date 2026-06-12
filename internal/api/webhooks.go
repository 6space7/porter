package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type webhookHandler struct {
	apps        AppWebhookService
	deployments DeploymentService
}

type webhookResponse struct {
	Accepted   bool                `json:"accepted"`
	Skipped    bool                `json:"skipped"`
	Reason     string              `json:"reason,omitempty"`
	Branch     string              `json:"branch,omitempty"`
	Deployment *DeploymentResponse `json:"deployment,omitempty"`
}

func mountWebhookRoutes(router chi.Router, apps AppWebhookService, deployments DeploymentService) {
	handler := webhookHandler{apps: apps, deployments: deployments}
	router.Post("/webhooks/github/{appID}", handler.github)
}

func (handler webhookHandler) github(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	if !validID(appID) {
		WriteError(w, http.StatusBadRequest, "invalid_app_id", "App id is invalid.", "Use a valid app id returned by the API.", map[string]any{"field": "app_id"})
		return
	}

	config, err := handler.apps.GetAppWebhook(r.Context(), appID)
	if err != nil {
		writeAppServiceError(w, err, "Webhook could not be loaded.")
		return
	}
	if !config.Enabled || strings.TrimSpace(config.Secret) == "" {
		WriteError(w, http.StatusNotFound, "webhook_not_configured", "Webhook is not configured for this app.", "Enable auto-deploy for the app before sending webhook events.", nil)
		return
	}

	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 5<<20))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_payload", "Webhook payload is too large or unreadable.", "Send a valid Git push payload.", nil)
		return
	}
	if !validGitHubSignature(config.Secret, raw, r.Header.Get("X-Hub-Signature-256")) {
		WriteError(w, http.StatusUnauthorized, "invalid_signature", "Webhook signature is invalid.", "Send the X-Hub-Signature-256 HMAC header.", nil)
		return
	}

	var payload struct {
		Ref        string `json:"ref"`
		Repository struct {
			CloneURL string `json:"clone_url"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_payload", "Webhook payload is not valid JSON.", "Send a valid Git push payload.", nil)
		return
	}

	branch := branchFromRef(payload.Ref)
	if branch != config.Branch {
		writeJSON(w, http.StatusAccepted, webhookResponse{
			Accepted: true,
			Skipped:  true,
			Reason:   "branch_mismatch",
			Branch:   branch,
		})
		return
	}

	deployment, err := handler.deployments.DeployApp(r.Context(), appID)
	if err != nil && deployment.ID == "" {
		WriteError(w, http.StatusInternalServerError, "internal_error", "Deployment could not be started.", "Try again or check deployment logs.", nil)
		return
	}
	writeJSON(w, http.StatusAccepted, webhookResponse{
		Accepted:   true,
		Skipped:    false,
		Branch:     branch,
		Deployment: &deployment,
	})
}

func validGitHubSignature(secret string, body []byte, header string) bool {
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(header, "sha256=") {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(header), []byte(expected)) == 1
}

func branchFromRef(ref string) string {
	return strings.TrimPrefix(strings.TrimSpace(ref), "refs/heads/")
}

func webhookURL(r *http.Request, appID string) string {
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		if r.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	if host == "" {
		return "/api/v1/webhooks/github/" + appID
	}
	return proto + "://" + host + "/api/v1/webhooks/github/" + appID
}
