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
}

type LoginResponse struct {
	Token   string   `json:"token"`
	TokenID string   `json:"token_id"`
	Scopes  []string `json:"scopes"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authHandler struct {
	auth AuthService
}

func mountAuthRoutes(router chi.Router, auth AuthService) {
	handler := authHandler{auth: auth}
	router.Post("/auth/login", handler.login)
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

func AdminScopes() []string {
	return []string{"projects:read", "projects:write", "apps:read", "apps:write", "apps:deploy"}
}
