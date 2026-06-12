package api

import (
	"context"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"nhooyr.io/websocket"
)

type LogService interface {
	GetBuildLog(ctx context.Context, deploymentID string) (BuildLogResponse, error)
	StreamRuntimeLogs(ctx context.Context, appID string) (io.ReadCloser, error)
}

type BuildLogResponse struct {
	DeploymentID string `json:"deployment_id"`
	AppID        string `json:"app_id"`
	Status       string `json:"status"`
	Stage        string `json:"stage"`
	BuildLog     string `json:"build_log"`
}

type logHandler struct {
	logs LogService
}

func mountLogRoutes(router chi.Router, logs LogService) {
	handler := logHandler{logs: logs}
	router.With(RequireScope("apps:read")).Get("/deployments/{deploymentID}/build-log", handler.buildLog)
	router.With(RequireScope("apps:read")).Get("/apps/{appID}/logs", handler.runtimeLogs)
}

func (handler logHandler) buildLog(w http.ResponseWriter, r *http.Request) {
	deploymentID := chi.URLParam(r, "deploymentID")
	if !validID(deploymentID) {
		WriteError(w, http.StatusBadRequest, "invalid_deployment_id", "Deployment id is invalid.", "Use a valid deployment id returned by the API.", map[string]any{"field": "deployment_id"})
		return
	}

	buildLog, err := handler.logs.GetBuildLog(r.Context(), deploymentID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "Build log could not be loaded.", "Try again or check server logs.", nil)
		return
	}
	writeJSON(w, http.StatusOK, buildLog)
}

func (handler logHandler) runtimeLogs(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	if !validID(appID) {
		WriteError(w, http.StatusBadRequest, "invalid_app_id", "App id is invalid.", "Use a valid app id returned by the API.", map[string]any{"field": "app_id"})
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	stream, err := handler.logs.StreamRuntimeLogs(r.Context(), appID)
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "runtime logs unavailable")
		return
	}
	defer stream.Close()

	buf := make([]byte, 32*1024)
	for {
		n, readErr := stream.Read(buf)
		if n > 0 {
			if err := conn.Write(r.Context(), websocket.MessageText, buf[:n]); err != nil {
				return
			}
		}
		if readErr == io.EOF {
			return
		}
		if readErr != nil {
			_ = conn.Close(websocket.StatusInternalError, "runtime log stream failed")
			return
		}
	}
}
