package proxy_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/6space7/porter/internal/proxy"
)

func TestCaddyAdminClientAppliesConfigToAdminAPI(t *testing.T) {
	var method string
	var path string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	admin := proxy.CaddyAdminClient{BaseURL: server.URL, HTTPClient: server.Client()}
	err := admin.ApplyConfig(context.Background(), proxy.CaddyConfig{
		HTTP: proxy.CaddyHTTPConfig{
			AskURL: "http://127.0.0.1:8080/api/v1/caddy/ask",
			Routes: []proxy.CaddyRoute{
				{Hostname: "web.example.com", UpstreamDial: "porter-app_web:3000"},
			},
		},
	})
	if err != nil {
		t.Fatalf("apply config: %v", err)
	}

	if method != http.MethodPost || path != "/load" {
		t.Fatalf("request = %s %s, want POST /load", method, path)
	}

	apps := body["apps"].(map[string]any)
	adminConfig := body["admin"].(map[string]any)
	if adminConfig["listen"] != "0.0.0.0:2019" {
		t.Fatalf("admin config = %#v", adminConfig)
	}
	httpApp := apps["http"].(map[string]any)
	servers := httpApp["servers"].(map[string]any)
	porter := servers["porter"].(map[string]any)
	routes := porter["routes"].([]any)
	firstRoute := routes[0].(map[string]any)
	handle := firstRoute["handle"].([]any)[0].(map[string]any)
	upstreams := handle["upstreams"].([]any)
	if upstreams[0].(map[string]any)["dial"] != "porter-app_web:3000" {
		t.Fatalf("upstreams = %#v", upstreams)
	}
	tlsPolicies := porter["tls_connection_policies"].([]any)
	if len(tlsPolicies) != 1 {
		t.Fatalf("tls connection policies = %#v", tlsPolicies)
	}

	tlsApp := apps["tls"].(map[string]any)
	automation := tlsApp["automation"].(map[string]any)
	onDemand := automation["on_demand"].(map[string]any)
	permission, ok := onDemand["permission"].(map[string]any)
	if !ok {
		t.Fatalf("on demand permission missing: %#v", onDemand)
	}
	if permission["module"] != "http" || permission["endpoint"] != "http://127.0.0.1:8080/api/v1/caddy/ask" {
		t.Fatalf("on demand permission = %#v", permission)
	}
	if _, ok := onDemand["ask"]; ok {
		t.Fatalf("legacy on demand ask should not be emitted: %#v", onDemand)
	}
}

func TestCaddyAdminClientReturnsErrorOnFailedApply(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad config", http.StatusBadRequest)
	}))
	defer server.Close()

	admin := proxy.CaddyAdminClient{BaseURL: server.URL, HTTPClient: server.Client()}
	err := admin.ApplyConfig(context.Background(), proxy.CaddyConfig{})
	if err == nil {
		t.Fatal("expected failed admin response to return error")
	}
}

func TestCaddyAdminClientRetriesTransientApplyErrors(t *testing.T) {
	attempts := 0
	httpClient := &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return nil, errors.New("connection reset by peer")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{},
				Body:       http.NoBody,
			}, nil
		}),
	}

	admin := proxy.CaddyAdminClient{
		BaseURL:     "http://127.0.0.1:2019",
		HTTPClient:  httpClient,
		MaxAttempts: 2,
	}
	err := admin.ApplyConfig(context.Background(), proxy.CaddyConfig{})
	if err != nil {
		t.Fatalf("apply config after transient error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
