package mcpserver_test

import (
	"testing"

	portermcp "github.com/6space7/porter/internal/mcp"
)

func TestServerListsPorterTools(t *testing.T) {
	server := portermcp.NewServer(portermcp.Dependencies{})
	tools := server.ListTools()

	for _, name := range []string{
		"porter_list_apps",
		"porter_create_app",
		"porter_deploy_app",
		"porter_list_deployments",
		"porter_get_build_log",
		"porter_get_runtime_logs",
		"porter_rollback_app",
		"porter_search_service_templates",
		"porter_deploy_service",
		"porter_attach_service",
		"porter_diagnose_latest_deployment",
	} {
		if _, ok := tools[name]; !ok {
			t.Fatalf("tool %q not registered; got %#v", name, tools)
		}
	}
}
