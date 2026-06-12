package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/6space7/porter/internal/auth"
	"github.com/6space7/porter/internal/store"
	"github.com/go-chi/chi/v5"
)

var ErrInvalidLogin = errors.New("invalid login")

type AuthService interface {
	Login(ctx context.Context, email, password string) (LoginResponse, error)
	Logout(ctx context.Context, tokenID string) error
	CreateToken(ctx context.Context, name string, scopes []string) (CreateTokenResponse, error)
}

type LoginResponse struct {
	Token   string   `json:"token"`
	TokenID string   `json:"token_id"`
	Scopes  []string `json:"scopes"`
}

type CreateTokenResponse struct {
	Token   string   `json:"token"`
	TokenID string   `json:"token_id"`
	Name    string   `json:"name"`
	Scopes  []string `json:"scopes"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type createTokenRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

type authHandler struct {
	auth AuthService
}

func mountAuthRoutes(router chi.Router, auth AuthService) {
	handler := authHandler{auth: auth}
	router.Post("/auth/login", handler.login)
}

func mountAuthSessionRoutes(router chi.Router, auth AuthService) {
	handler := authHandler{auth: auth}
	router.Delete("/auth/session", handler.logout)
	router.With(RequireScope("tokens:write")).Post("/auth/tokens", handler.createToken)
}

func (handler authHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.", "Send an object with email and password.", nil)
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" || req.Password == "" {
		WriteError(w, http.StatusBadRequest, "invalid_credentials", "Email and password are required.", "Send the admin email and password.", nil)
		return
	}

	response, err := handler.auth.Login(r.Context(), email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidLogin) {
			WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication failed.", "Check the admin email and password.", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "internal_error", "Login could not be completed.", "Try again or check server logs.", nil)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (handler authHandler) logout(w http.ResponseWriter, r *http.Request) {
	principal, ok := PrincipalFromContext(r.Context())
	if !ok || strings.TrimSpace(principal.TokenID) == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required.", "Send a bearer token in the Authorization header.", nil)
		return
	}
	if err := handler.auth.Logout(r.Context(), principal.TokenID); err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "Logout could not be completed.", "Try again or check server logs.", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler authHandler) createToken(w http.ResponseWriter, r *http.Request) {
	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.", "Send an object with token name and scopes.", nil)
		return
	}
	name := strings.TrimSpace(req.Name)
	scopes, ok := normalizeRequestedScopes(req.Scopes)
	if name == "" || !ok {
		WriteError(w, http.StatusBadRequest, "invalid_token", "Token name and scopes are invalid.", "Send a non-empty name and known scopes.", nil)
		return
	}

	response, err := handler.auth.CreateToken(r.Context(), name, scopes)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "Token could not be created.", "Try again or check server logs.", nil)
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

type storeAuthService struct {
	queries *store.Queries
}

func NewStoreAuthService(queries *store.Queries) AuthService {
	return storeAuthService{queries: queries}
}

func (service storeAuthService) Login(ctx context.Context, email, password string) (LoginResponse, error) {
	user, err := service.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoginResponse{}, ErrInvalidLogin
		}
		return LoginResponse{}, err
	}
	if !auth.VerifyPassword(user.PasswordHash, password) {
		return LoginResponse{}, ErrInvalidLogin
	}

	scopes := AdminScopes()
	plaintext, record, err := auth.NewToken("admin login", scopes)
	if err != nil {
		return LoginResponse{}, err
	}
	if _, err := service.queries.CreateToken(ctx, store.CreateTokenParams{
		ID:     record.ID,
		Name:   record.Name,
		Hash:   record.Hash,
		Scopes: strings.Join(record.Scopes, ","),
	}); err != nil {
		return LoginResponse{}, err
	}
	return LoginResponse{
		Token:   plaintext,
		TokenID: record.ID,
		Scopes:  record.Scopes,
	}, nil
}

func (service storeAuthService) Logout(ctx context.Context, tokenID string) error {
	return service.queries.DeleteToken(ctx, tokenID)
}

func (service storeAuthService) CreateToken(ctx context.Context, name string, scopes []string) (CreateTokenResponse, error) {
	plaintext, record, err := auth.NewToken(name, scopes)
	if err != nil {
		return CreateTokenResponse{}, err
	}
	if _, err := service.queries.CreateToken(ctx, store.CreateTokenParams{
		ID:     record.ID,
		Name:   record.Name,
		Hash:   record.Hash,
		Scopes: strings.Join(record.Scopes, ","),
	}); err != nil {
		return CreateTokenResponse{}, err
	}
	return CreateTokenResponse{
		Token:   plaintext,
		TokenID: record.ID,
		Name:    record.Name,
		Scopes:  record.Scopes,
	}, nil
}

func AdminScopes() []string {
	return []string{"projects:read", "projects:write", "apps:read", "apps:write", "apps:deploy", "tokens:write"}
}

func normalizeRequestedScopes(input []string) ([]string, bool) {
	allowed := map[string]bool{}
	for _, scope := range AdminScopes() {
		allowed[scope] = true
	}
	scopes := make([]string, 0, len(input))
	seen := map[string]bool{}
	for _, raw := range input {
		scope := strings.TrimSpace(raw)
		if scope == "" || !allowed[scope] {
			return nil, false
		}
		if !seen[scope] {
			scopes = append(scopes, scope)
			seen[scope] = true
		}
	}
	return scopes, len(scopes) > 0
}
