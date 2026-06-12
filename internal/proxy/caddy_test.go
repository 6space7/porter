package proxy_test

import (
	"context"
	"encoding/json"
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

	if method != http.MethodPut || path != "/config" {
		t.Fatalf("request = %s %s, want PUT /config", method, path)
	}

	apps := body["apps"].(map[string]any)
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
	if onDemand["ask"] != "http://127.0.0.1:8080/api/v1/caddy/ask" {
		t.Fatalf("on demand ask = %#v", onDemand)
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
