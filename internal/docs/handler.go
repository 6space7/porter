package docs

import (
	"encoding/json"
	"net/http"
	"strings"

	portermcp "github.com/6space7/porter/internal/mcp"
)

type Config struct {
	PlatformDomain string
}

type Docs struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	APIBase     string    `json:"api_base"`
	MCPEndpoint string    `json:"mcp_endpoint"`
	Auth        AuthDoc   `json:"auth"`
	Tools       []ToolDoc `json:"tools"`
	Examples    []Example `json:"examples"`
}

type AuthDoc struct {
	Header  string `json:"header"`
	Scheme  string `json:"scheme"`
	Example string `json:"example"`
}

type ToolDoc struct {
	Name        string   `json:"name"`
	Scopes      []string `json:"scopes"`
	Description string   `json:"description"`
}

type Example struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Body        map[string]any `json:"body"`
}

func NewHandler(cfg Config, next http.Handler) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	doc := Build(cfg)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/llms.txt":
			if !allowReadMethod(w, r) {
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(LLMSText(doc)))
		case "/api/v1/docs":
			if !allowReadMethod(w, r) {
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(doc)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func Build(cfg Config) Docs {
	return Docs{
		Name:        "porter",
		Description: "AI-first self-hosted PaaS for deploying apps and service dependencies.",
		APIBase:     "/api/v1",
		MCPEndpoint: endpoint(cfg.PlatformDomain, "/api/v1/mcp"),
		Auth: AuthDoc{
			Header:  "Authorization",
			Scheme:  "Bearer",
			Example: "Authorization: Bearer <porter token>",
		},
		Tools:    toolDocs(),
		Examples: examples(),
	}
}

func LLMSText(doc Docs) string {
	var builder strings.Builder
	builder.WriteString("# porter\n\n")
	builder.WriteString(doc.Description)
	builder.WriteString("\n\n")
	builder.WriteString("Use porter with a scoped API token.\n\n")
	builder.WriteString("Authentication: ")
	builder.WriteString(doc.Auth.Example)
	builder.WriteString("\n")
	builder.WriteString("JSON API base: ")
	builder.WriteString(doc.APIBase)
	builder.WriteString("\n")
	builder.WriteString("MCP endpoint: ")
	builder.WriteString(doc.MCPEndpoint)
	builder.WriteString("\n")
	builder.WriteString("Machine-readable docs: /api/v1/docs\n\n")
	builder.WriteString("Tools:\n")
	for _, tool := range doc.Tools {
		builder.WriteString("- ")
		builder.WriteString(tool.Name)
		if len(tool.Scopes) > 0 {
			builder.WriteString(" (")
			builder.WriteString(strings.Join(tool.Scopes, ", "))
			builder.WriteString(")")
		}
		builder.WriteString(": ")
		builder.WriteString(tool.Description)
		builder.WriteString("\n")
	}
	return builder.String()
}

func toolDocs() []ToolDoc {
	return []ToolDoc{
		{Name: portermcp.ToolListProjects, Scopes: []string{"projects:read"}, Description: "List projects."},
		{Name: portermcp.ToolCreateProject, Scopes: []string{"projects:write"}, Description: "Create a project."},
		{Name: portermcp.ToolListApps, Scopes: []string{"apps:read"}, Description: "List apps."},
		{Name: portermcp.ToolCreateApp, Scopes: []string{"apps:write"}, Description: "Create an app from a Git repository."},
		{Name: portermcp.ToolDeployApp, Scopes: []string{"apps:deploy"}, Description: "Start a deployment for an app."},
		{Name: portermcp.ToolListDeployments, Scopes: []string{"apps:read"}, Description: "List deployments for an app."},
		{Name: portermcp.ToolGetBuildLog, Scopes: []string{"apps:read"}, Description: "Read a deployment build log."},
		{Name: portermcp.ToolGetRuntimeLogs, Scopes: []string{"apps:read"}, Description: "Read recent runtime logs."},
		{Name: portermcp.ToolListEnvVars, Scopes: []string{"apps:read"}, Description: "List app env vars with secrets masked."},
		{Name: portermcp.ToolSetEnvVar, Scopes: []string{"apps:write"}, Description: "Set an app env var."},
		{Name: portermcp.ToolRollbackApp, Scopes: []string{"apps:deploy"}, Description: "Rollback to a previous successful deployment."},
		{Name: portermcp.ToolSearchServiceTemplates, Scopes: []string{"services:read"}, Description: "Search the service catalog."},
		{Name: portermcp.ToolDeployService, Scopes: []string{"services:write"}, Description: "Deploy a service from the catalog."},
		{Name: portermcp.ToolAttachService, Scopes: []string{"services:write", "apps:write"}, Description: "Attach a service to an app by injecting env vars."},
		{Name: portermcp.ToolDiagnoseLatest, Scopes: []string{"apps:read"}, Description: "Summarize the latest deployment status and hints."},
	}
}

func examples() []Example {
	return []Example{
		{
			Name:        "initialize_mcp",
			Description: "Initialize the streamable HTTP MCP session.",
			Body: map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "initialize",
				"params": map[string]any{
					"protocolVersion": "2025-11-25",
					"capabilities":    map[string]any{},
					"clientInfo":      map[string]any{"name": "agent", "version": "1.0.0"},
				},
			},
		},
		{
			Name:        "deploy_app",
			Description: "Call the app deploy tool after initialization.",
			Body: map[string]any{
				"jsonrpc": "2.0",
				"id":      2,
				"method":  "tools/call",
				"params": map[string]any{
					"name":      portermcp.ToolDeployApp,
					"arguments": map[string]any{"app_id": "app_xxx"},
				},
			},
		},
	}
}

func endpoint(platformDomain, path string) string {
	platformDomain = strings.TrimSpace(platformDomain)
	if platformDomain == "" {
		return path
	}
	base := strings.TrimRight(platformDomain, "/")
	if strings.HasPrefix(base, "http://") || strings.HasPrefix(base, "https://") {
		return base + path
	}
	return "https://" + base + path
}

func allowReadMethod(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	w.Header().Set("Allow", "GET, HEAD")
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	return false
}
