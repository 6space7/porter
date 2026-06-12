package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/6space7/porter/internal/api"
	"github.com/6space7/porter/internal/remote"
	"github.com/6space7/porter/internal/store"
)

func TestServerRoutesRequireAuthAndScopes(t *testing.T) {
	servers := &fakeServerService{}
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: serverTestVerifier(),
		Servers:       servers,
	})

	assertStatusAndCode(t, router, http.MethodGet, "/api/v1/servers", "", http.StatusUnauthorized, "unauthorized")
	assertStatusAndCode(t, router, http.MethodGet, "/api/v1/servers", "Bearer apps-read-token", http.StatusForbidden, "forbidden")
	assertStatusAndCode(t, router, http.MethodPost, "/api/v1/servers", "Bearer server-read-token", http.StatusForbidden, "forbidden")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	req.Header.Set("Authorization", "Bearer server-read-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestCreateServerValidatesRequiredFields(t *testing.T) {
	servers := &fakeServerService{}
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: serverTestVerifier(),
		Servers:       servers,
	})

	for _, body := range []string{
		`{"name":"","host":"203.0.113.10","ssh_user":"root","private_key":"key"}`,
		`{"name":"edge","host":"","ssh_user":"root","private_key":"key"}`,
		`{"name":"edge","host":"203.0.113.10","ssh_user":"","private_key":"key"}`,
		`{"name":"edge","host":"203.0.113.10","ssh_user":"root","private_key":""}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/servers", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer server-write-token")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assertErrorCode(t, rr, http.StatusBadRequest, "invalid_server")
	}
	if servers.createCalls != 0 {
		t.Fatalf("create calls = %d, want 0", servers.createCalls)
	}
}

func TestCreateServerChecksSSHAndStoresHealthyServer(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	queries := store.New(db.SQL())
	validator := &fakeServerValidator{result: remote.CheckResult{
		DockerVersion: "Docker version 27.0.0",
		OS:            "Linux porter-test",
	}}
	keyStore := &fakeServerKeyStore{ref: "/var/lib/porter/ssh-keys/srv_test.pem"}
	servers := api.NewStoreServerServiceWithOptions(queries, api.StoreServerServiceOptions{
		NewServerID: func() string {
			return "srv_test"
		},
		Validator: validator,
		KeyStore:  keyStore,
	})
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: serverTestVerifier(),
		Servers:       servers,
	})

	body := `{"name":"edge","host":"203.0.113.10","ssh_user":"root","private_key":"PRIVATE KEY"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/servers", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer server-write-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	if validator.req.Host != "203.0.113.10" || validator.req.User != "root" || string(validator.req.PrivateKeyPEM) != "PRIVATE KEY" {
		t.Fatalf("validator request = %#v", validator.req)
	}
	if keyStore.serverID != "srv_test" || string(keyStore.privateKeyPEM) != "PRIVATE KEY" {
		t.Fatalf("key store = %#v", keyStore)
	}
	if strings.Contains(rr.Body.String(), "PRIVATE KEY") || strings.Contains(rr.Body.String(), keyStore.ref) {
		t.Fatalf("response leaked private key material: %s", rr.Body.String())
	}

	var created api.ServerResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode server: %v", err)
	}
	if created.ID != "srv_test" || created.Status != "healthy" || created.DockerVersion == "" || created.OS == "" {
		t.Fatalf("created = %#v", created)
	}

	row, err := queries.UpdateServerStatus(ctx, store.UpdateServerStatusParams{ID: "srv_test", Status: "healthy"})
	if err != nil {
		t.Fatalf("load created server through update query: %v", err)
	}
	if !row.SshKeyRef.Valid || row.SshKeyRef.String != keyStore.ref {
		t.Fatalf("ssh key ref = %#v", row.SshKeyRef)
	}
}

func serverTestVerifier() api.TokenVerifier {
	return api.TokenVerifierFunc(func(_ context.Context, token string) (api.Principal, error) {
		switch token {
		case "apps-read-token":
			return api.Principal{TokenID: "tok_apps", Scopes: []string{"apps:read"}}, nil
		case "server-read-token":
			return api.Principal{TokenID: "tok_server_read", Scopes: []string{"servers:read"}}, nil
		case "server-write-token":
			return api.Principal{TokenID: "tok_server_write", Scopes: []string{"servers:read", "servers:write"}}, nil
		default:
			return api.Principal{}, api.ErrInvalidToken
		}
	})
}

type fakeServerService struct {
	createCalls int
	servers     []api.ServerResponse
}

func (service *fakeServerService) ListServers(context.Context) ([]api.ServerResponse, error) {
	return append([]api.ServerResponse(nil), service.servers...), nil
}

func (service *fakeServerService) CreateServer(_ context.Context, input api.CreateServerInput) (api.ServerResponse, error) {
	service.createCalls++
	server := api.ServerResponse{
		ID:     "srv_1",
		Name:   input.Name,
		Host:   input.Host,
		Status: "healthy",
	}
	service.servers = append(service.servers, server)
	return server, nil
}

type fakeServerValidator struct {
	req    remote.CheckRequest
	result remote.CheckResult
	err    error
}

func (validator *fakeServerValidator) Check(_ context.Context, req remote.CheckRequest) (remote.CheckResult, error) {
	validator.req = req
	return validator.result, validator.err
}

type fakeServerKeyStore struct {
	ref           string
	serverID      string
	privateKeyPEM []byte
}

func (store *fakeServerKeyStore) Put(_ context.Context, serverID string, privateKeyPEM []byte) (string, error) {
	store.serverID = serverID
	store.privateKeyPEM = append([]byte(nil), privateKeyPEM...)
	return store.ref, nil
}
