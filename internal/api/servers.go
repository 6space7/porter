package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/6space7/porter/internal/remote"
	"github.com/go-chi/chi/v5"
)

type ServerService interface {
	ListServers(ctx context.Context) ([]ServerResponse, error)
	CreateServer(ctx context.Context, input CreateServerInput) (ServerResponse, error)
}

type CreateServerInput struct {
	Name          string
	Host          string
	SSHUser       string
	PrivateKeyPEM []byte
}

type ServerResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Host          string `json:"host"`
	Status        string `json:"status"`
	DockerVersion string `json:"docker_version,omitempty"`
	OS            string `json:"os,omitempty"`
}

type serverHandler struct {
	servers ServerService
}

func mountServerRoutes(router chi.Router, servers ServerService) {
	handler := serverHandler{servers: servers}
	router.With(RequireScope("servers:read")).Get("/servers", handler.list)
	router.With(RequireScope("servers:write")).Post("/servers", handler.create)
}

func (handler serverHandler) list(w http.ResponseWriter, r *http.Request) {
	servers, err := handler.servers.ListServers(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "Servers could not be loaded.", "Try again or check server logs.", nil)
		return
	}
	writeJSON(w, http.StatusOK, servers)
}

func (handler serverHandler) create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name       string `json:"name"`
		Host       string `json:"host"`
		SSHUser    string `json:"ssh_user"`
		PrivateKey string `json:"private_key"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.", "Send a JSON object with server settings.", nil)
		return
	}
	serverInput := CreateServerInput{
		Name:          strings.TrimSpace(input.Name),
		Host:          strings.TrimSpace(input.Host),
		SSHUser:       strings.TrimSpace(input.SSHUser),
		PrivateKeyPEM: []byte(strings.TrimSpace(input.PrivateKey)),
	}
	if err := validateCreateServerInput(serverInput); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_server", "Server settings are invalid.", err.Error(), nil)
		return
	}

	server, err := handler.servers.CreateServer(r.Context(), serverInput)
	if err != nil {
		if errors.Is(err, remote.ErrDockerMissing) {
			WriteError(w, http.StatusBadRequest, "docker_missing", "Docker is missing on the server.", "Install Docker on the server and try again.", nil)
			return
		}
		WriteError(w, http.StatusBadRequest, "server_check_failed", "Server could not be validated over SSH.", "Check host, user, key, and Docker, then try again.", nil)
		return
	}
	writeJSON(w, http.StatusCreated, server)
}

func validateCreateServerInput(input CreateServerInput) error {
	if input.Name == "" {
		return errors.New("name is required")
	}
	if input.Host == "" {
		return errors.New("host is required")
	}
	if input.SSHUser == "" {
		return errors.New("ssh user is required")
	}
	if len(input.PrivateKeyPEM) == 0 {
		return errors.New("private key is required")
	}
	return nil
}
