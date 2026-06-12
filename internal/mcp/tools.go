package mcpserver

import (
	"context"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ToolListApps               = "porter_list_apps"
	ToolCreateApp              = "porter_create_app"
	ToolDeployApp              = "porter_deploy_app"
	ToolListDeployments        = "porter_list_deployments"
	ToolGetBuildLog            = "porter_get_build_log"
	ToolGetRuntimeLogs         = "porter_get_runtime_logs"
	ToolRollbackApp            = "porter_rollback_app"
	ToolSearchServiceTemplates = "porter_search_service_templates"
	ToolDeployService          = "porter_deploy_service"
	ToolAttachService          = "porter_attach_service"
	ToolDiagnoseLatest         = "porter_diagnose_latest_deployment"
)

func registerTools(mcpServer *server.MCPServer, deps Dependencies) {
	for _, spec := range toolSpecs() {
		mcpServer.AddTool(spec.tool, stubHandler(spec.name, deps))
	}
}

type toolSpec struct {
	name string
	tool mcpsdk.Tool
}

func toolSpecs() []toolSpec {
	return []toolSpec{
		{
			name: ToolListApps,
			tool: mcpsdk.NewTool(ToolListApps,
				mcpsdk.WithDescription("List apps visible to this porter token."),
			),
		},
		{
			name: ToolCreateApp,
			tool: mcpsdk.NewTool(ToolCreateApp,
				mcpsdk.WithDescription("Create a porter app from a Git repository."),
			),
		},
		{
			name: ToolDeployApp,
			tool: mcpsdk.NewTool(ToolDeployApp,
				mcpsdk.WithDescription("Deploy an existing porter app."),
			),
		},
		{
			name: ToolListDeployments,
			tool: mcpsdk.NewTool(ToolListDeployments,
				mcpsdk.WithDescription("List deployments for an app."),
			),
		},
		{
			name: ToolGetBuildLog,
			tool: mcpsdk.NewTool(ToolGetBuildLog,
				mcpsdk.WithDescription("Read a deployment build log."),
			),
		},
		{
			name: ToolGetRuntimeLogs,
			tool: mcpsdk.NewTool(ToolGetRuntimeLogs,
				mcpsdk.WithDescription("Read recent runtime logs for an app."),
			),
		},
		{
			name: ToolRollbackApp,
			tool: mcpsdk.NewTool(ToolRollbackApp,
				mcpsdk.WithDescription("Rollback an app to a previous successful deployment."),
			),
		},
		{
			name: ToolSearchServiceTemplates,
			tool: mcpsdk.NewTool(ToolSearchServiceTemplates,
				mcpsdk.WithDescription("Search porter service catalog templates."),
			),
		},
		{
			name: ToolDeployService,
			tool: mcpsdk.NewTool(ToolDeployService,
				mcpsdk.WithDescription("Deploy a service from the porter catalog."),
			),
		},
		{
			name: ToolAttachService,
			tool: mcpsdk.NewTool(ToolAttachService,
				mcpsdk.WithDescription("Attach a service to an app by injecting provided env vars."),
			),
		},
		{
			name: ToolDiagnoseLatest,
			tool: mcpsdk.NewTool(ToolDiagnoseLatest,
				mcpsdk.WithDescription("Diagnose the latest deployment for an app with stage, log tail, and hints."),
			),
		},
	}
}

func stubHandler(name string, _ Dependencies) server.ToolHandlerFunc {
	return func(context.Context, mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return mcpsdk.NewToolResultError(name + " is not implemented yet"), nil
	}
}
