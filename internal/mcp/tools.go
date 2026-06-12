package mcpserver

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/6space7/porter/internal/api"
	coreauth "github.com/6space7/porter/internal/auth"
	secretcrypto "github.com/6space7/porter/internal/crypto"
	"github.com/6space7/porter/internal/deploy"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ToolListProjects           = "porter_list_projects"
	ToolCreateProject          = "porter_create_project"
	ToolListApps               = "porter_list_apps"
	ToolCreateApp              = "porter_create_app"
	ToolDeployApp              = "porter_deploy_app"
	ToolListDeployments        = "porter_list_deployments"
	ToolGetBuildLog            = "porter_get_build_log"
	ToolGetRuntimeLogs         = "porter_get_runtime_logs"
	ToolListEnvVars            = "porter_list_env_vars"
	ToolSetEnvVar              = "porter_set_env_var"
	ToolRollbackApp            = "porter_rollback_app"
	ToolSearchServiceTemplates = "porter_search_service_templates"
	ToolDeployService          = "porter_deploy_service"
	ToolAttachService          = "porter_attach_service"
	ToolDiagnoseLatest         = "porter_diagnose_latest_deployment"
)

const runtimeLogLimit = 64 * 1024

func registerTools(mcpServer *server.MCPServer, deps Dependencies) {
	handlers := toolHandlers{deps: deps}
	for _, spec := range toolSpecs() {
		mcpServer.AddTool(spec.tool, handlers.handler(spec.name))
	}
}

type toolHandlers struct {
	deps Dependencies
}

type toolSpec struct {
	name string
	tool mcpsdk.Tool
}

func toolSpecs() []toolSpec {
	return []toolSpec{
		{
			name: ToolListProjects,
			tool: mcpsdk.NewTool(ToolListProjects,
				mcpsdk.WithDescription("List projects visible to this porter token. Requires projects:read."),
			),
		},
		{
			name: ToolCreateProject,
			tool: mcpsdk.NewTool(ToolCreateProject,
				mcpsdk.WithDescription("Create a porter project. Requires projects:write."),
				mcpsdk.WithString("name", mcpsdk.Required(), mcpsdk.Description("Lowercase project name.")),
			),
		},
		{
			name: ToolListApps,
			tool: mcpsdk.NewTool(ToolListApps,
				mcpsdk.WithDescription("List apps visible to this porter token. Requires apps:read."),
			),
		},
		{
			name: ToolCreateApp,
			tool: mcpsdk.NewTool(ToolCreateApp,
				mcpsdk.WithDescription("Create a porter app from a Git repository. Requires apps:write."),
				mcpsdk.WithString("project_id", mcpsdk.Required(), mcpsdk.Description("Project id returned by porter.")),
				mcpsdk.WithString("name", mcpsdk.Required(), mcpsdk.Description("Lowercase app name.")),
				mcpsdk.WithString("git_url", mcpsdk.Required(), mcpsdk.Description("HTTPS or SSH Git repository URL.")),
				mcpsdk.WithString("branch", mcpsdk.Description("Git branch. Defaults to main.")),
				mcpsdk.WithString("build_type", mcpsdk.Description("dockerfile or nixpacks."), mcpsdk.Enum("dockerfile", "nixpacks")),
				mcpsdk.WithInteger("internal_port", mcpsdk.Description("Internal container port. Defaults to 3000."), mcpsdk.Min(1), mcpsdk.Max(65535)),
			),
		},
		{
			name: ToolDeployApp,
			tool: mcpsdk.NewTool(ToolDeployApp,
				mcpsdk.WithDescription("Deploy an existing porter app. Requires apps:deploy."),
				mcpsdk.WithString("app_id", mcpsdk.Required(), mcpsdk.Description("App id returned by porter.")),
			),
		},
		{
			name: ToolListDeployments,
			tool: mcpsdk.NewTool(ToolListDeployments,
				mcpsdk.WithDescription("List deployments for an app. Requires apps:read."),
				mcpsdk.WithString("app_id", mcpsdk.Required(), mcpsdk.Description("App id returned by porter.")),
			),
		},
		{
			name: ToolGetBuildLog,
			tool: mcpsdk.NewTool(ToolGetBuildLog,
				mcpsdk.WithDescription("Read a deployment build log. Requires apps:read."),
				mcpsdk.WithString("deployment_id", mcpsdk.Required(), mcpsdk.Description("Deployment id returned by porter.")),
			),
		},
		{
			name: ToolGetRuntimeLogs,
			tool: mcpsdk.NewTool(ToolGetRuntimeLogs,
				mcpsdk.WithDescription("Read recent runtime logs for an app. Requires apps:read."),
				mcpsdk.WithString("app_id", mcpsdk.Required(), mcpsdk.Description("App id returned by porter.")),
			),
		},
		{
			name: ToolListEnvVars,
			tool: mcpsdk.NewTool(ToolListEnvVars,
				mcpsdk.WithDescription("List app environment variables, masking secret values. Requires apps:read."),
				mcpsdk.WithString("app_id", mcpsdk.Required(), mcpsdk.Description("App id returned by porter.")),
			),
		},
		{
			name: ToolSetEnvVar,
			tool: mcpsdk.NewTool(ToolSetEnvVar,
				mcpsdk.WithDescription("Create or update an app environment variable. Requires apps:write."),
				mcpsdk.WithString("app_id", mcpsdk.Required(), mcpsdk.Description("App id returned by porter.")),
				mcpsdk.WithString("key", mcpsdk.Required(), mcpsdk.Description("Uppercase env var key.")),
				mcpsdk.WithString("value", mcpsdk.Required(), mcpsdk.Description("Env var value.")),
				mcpsdk.WithBoolean("is_secret", mcpsdk.Description("Mask and encrypt the value at rest.")),
			),
		},
		{
			name: ToolRollbackApp,
			tool: mcpsdk.NewTool(ToolRollbackApp,
				mcpsdk.WithDescription("Rollback an app to a previous successful deployment. Requires apps:deploy."),
				mcpsdk.WithString("app_id", mcpsdk.Required(), mcpsdk.Description("App id returned by porter.")),
				mcpsdk.WithString("deployment_id", mcpsdk.Required(), mcpsdk.Description("Successful deployment id to roll back to.")),
			),
		},
		{
			name: ToolSearchServiceTemplates,
			tool: mcpsdk.NewTool(ToolSearchServiceTemplates,
				mcpsdk.WithDescription("Search porter service catalog templates. Requires services:read."),
				mcpsdk.WithString("query", mcpsdk.Description("Optional search text.")),
			),
		},
		{
			name: ToolDeployService,
			tool: mcpsdk.NewTool(ToolDeployService,
				mcpsdk.WithDescription("Deploy a service from the porter catalog. Requires services:write."),
				mcpsdk.WithString("project_id", mcpsdk.Required(), mcpsdk.Description("Project id returned by porter.")),
				mcpsdk.WithString("template_slug", mcpsdk.Required(), mcpsdk.Description("Service template slug.")),
				mcpsdk.WithString("name", mcpsdk.Required(), mcpsdk.Description("Lowercase service name.")),
				mcpsdk.WithBoolean("exposed", mcpsdk.Description("Expose the service publicly when supported.")),
			),
		},
		{
			name: ToolAttachService,
			tool: mcpsdk.NewTool(ToolAttachService,
				mcpsdk.WithDescription("Attach a service to an app by injecting provided env vars. Requires services:write and apps:write."),
				mcpsdk.WithString("service_id", mcpsdk.Required(), mcpsdk.Description("Service id returned by porter.")),
				mcpsdk.WithString("app_id", mcpsdk.Required(), mcpsdk.Description("App id returned by porter.")),
			),
		},
		{
			name: ToolDiagnoseLatest,
			tool: mcpsdk.NewTool(ToolDiagnoseLatest,
				mcpsdk.WithDescription("Diagnose the latest deployment for an app with stage, log tail, and hints. Requires apps:read."),
				mcpsdk.WithString("app_id", mcpsdk.Required(), mcpsdk.Description("App id returned by porter.")),
			),
		},
	}
}

func (handlers toolHandlers) handler(name string) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		switch name {
		case ToolListProjects:
			return handlers.listProjects(ctx)
		case ToolCreateProject:
			return handlers.createProject(ctx, request)
		case ToolListApps:
			return handlers.listApps(ctx)
		case ToolCreateApp:
			return handlers.createApp(ctx, request)
		case ToolDeployApp:
			return handlers.deployApp(ctx, request)
		case ToolListDeployments:
			return handlers.listDeployments(ctx, request)
		case ToolGetBuildLog:
			return handlers.getBuildLog(ctx, request)
		case ToolGetRuntimeLogs:
			return handlers.getRuntimeLogs(ctx, request)
		case ToolListEnvVars:
			return handlers.listEnvVars(ctx, request)
		case ToolSetEnvVar:
			return handlers.setEnvVar(ctx, request)
		case ToolRollbackApp:
			return handlers.rollbackApp(ctx, request)
		case ToolSearchServiceTemplates:
			return handlers.searchServiceTemplates(ctx, request)
		case ToolDeployService:
			return handlers.deployService(ctx, request)
		case ToolAttachService:
			return handlers.attachService(ctx, request)
		case ToolDiagnoseLatest:
			return handlers.diagnoseLatestDeployment(ctx, request)
		default:
			return mcpsdk.NewToolResultError("unknown porter tool " + name), nil
		}
	}
}

func (handlers toolHandlers) listProjects(ctx context.Context) (*mcpsdk.CallToolResult, error) {
	if result := requireScopes(ctx, "projects:read"); result != nil {
		return result, nil
	}
	if handlers.deps.Projects == nil {
		return toolError("project service is not configured"), nil
	}
	projects, err := handlers.deps.Projects.ListProjects(ctx)
	if err != nil {
		return toolErrorf("list projects: %v", err), nil
	}
	return jsonResult(projects)
}

func (handlers toolHandlers) createProject(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if result := requireScopes(ctx, "projects:write"); result != nil {
		return result, nil
	}
	if handlers.deps.Projects == nil {
		return toolError("project service is not configured"), nil
	}
	name, result := requireString(request, "name")
	if result != nil {
		return result, nil
	}
	if err := api.ValidateProjectName(name); err != nil {
		return toolErrorf("invalid name: %v", err), nil
	}
	project, err := handlers.deps.Projects.CreateProject(ctx, name)
	if err != nil {
		return toolErrorf("create project: %v", err), nil
	}
	return jsonResult(project)
}

func (handlers toolHandlers) listApps(ctx context.Context) (*mcpsdk.CallToolResult, error) {
	if result := requireScopes(ctx, "apps:read"); result != nil {
		return result, nil
	}
	if handlers.deps.Apps == nil {
		return toolError("app service is not configured"), nil
	}
	apps, err := handlers.deps.Apps.ListApps(ctx)
	if err != nil {
		return toolErrorf("list apps: %v", err), nil
	}
	return jsonResult(apps)
}

func (handlers toolHandlers) createApp(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if result := requireScopes(ctx, "apps:write"); result != nil {
		return result, nil
	}
	if handlers.deps.Apps == nil {
		return toolError("app service is not configured"), nil
	}
	projectID, result := requireString(request, "project_id")
	if result != nil {
		return result, nil
	}
	name, result := requireString(request, "name")
	if result != nil {
		return result, nil
	}
	gitURL, result := requireString(request, "git_url")
	if result != nil {
		return result, nil
	}
	input := api.CreateAppInput{
		ProjectID:    projectID,
		Name:         name,
		GitURL:       gitURL,
		Branch:       defaultString(request.GetString("branch", ""), "main"),
		BuildType:    defaultString(request.GetString("build_type", ""), "dockerfile"),
		InternalPort: int64(request.GetInt("internal_port", 3000)),
	}
	if input.InternalPort == 0 {
		input.InternalPort = 3000
	}
	if !validID(input.ProjectID) {
		return toolError("project_id is invalid"), nil
	}
	if err := api.ValidateAppName(input.Name); err != nil {
		return toolErrorf("invalid name: %v", err), nil
	}
	if err := deploy.ValidateGitURL(input.GitURL); err != nil {
		return toolErrorf("invalid git_url: %v", err), nil
	}
	if err := api.ValidateBranchName(input.Branch); err != nil {
		return toolErrorf("invalid branch: %v", err), nil
	}
	if err := api.ValidateBuildType(input.BuildType); err != nil {
		return toolErrorf("invalid build_type: %v", err), nil
	}
	if input.InternalPort < 1 || input.InternalPort > 65535 {
		return toolError("internal_port must be between 1 and 65535"), nil
	}
	app, err := handlers.deps.Apps.CreateApp(ctx, input)
	if err != nil {
		return toolErrorf("create app: %v", err), nil
	}
	return jsonResult(app)
}

func (handlers toolHandlers) deployApp(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if result := requireScopes(ctx, "apps:deploy"); result != nil {
		return result, nil
	}
	if handlers.deps.Deployments == nil {
		return toolError("deployment service is not configured"), nil
	}
	appID, result := requireID(request, "app_id")
	if result != nil {
		return result, nil
	}
	deployment, err := handlers.deps.Deployments.DeployApp(ctx, appID)
	if err != nil && deployment.ID == "" {
		return toolErrorf("deploy app: %v", err), nil
	}
	return jsonResult(deployment)
}

func (handlers toolHandlers) listDeployments(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if result := requireScopes(ctx, "apps:read"); result != nil {
		return result, nil
	}
	if handlers.deps.Deployments == nil {
		return toolError("deployment service is not configured"), nil
	}
	appID, result := requireID(request, "app_id")
	if result != nil {
		return result, nil
	}
	deployments, err := handlers.deps.Deployments.ListDeployments(ctx, appID)
	if err != nil {
		return toolErrorf("list deployments: %v", err), nil
	}
	return jsonResult(deployments)
}

func (handlers toolHandlers) getBuildLog(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if result := requireScopes(ctx, "apps:read"); result != nil {
		return result, nil
	}
	if handlers.deps.Logs == nil {
		return toolError("log service is not configured"), nil
	}
	deploymentID, result := requireID(request, "deployment_id")
	if result != nil {
		return result, nil
	}
	buildLog, err := handlers.deps.Logs.GetBuildLog(ctx, deploymentID)
	if err != nil {
		return toolErrorf("get build log: %v", err), nil
	}
	return jsonResult(buildLog)
}

func (handlers toolHandlers) getRuntimeLogs(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if result := requireScopes(ctx, "apps:read"); result != nil {
		return result, nil
	}
	if handlers.deps.Logs == nil {
		return toolError("log service is not configured"), nil
	}
	appID, result := requireID(request, "app_id")
	if result != nil {
		return result, nil
	}
	stream, err := handlers.deps.Logs.StreamRuntimeLogs(ctx, appID)
	if err != nil {
		return toolErrorf("get runtime logs: %v", err), nil
	}
	defer stream.Close()
	body, err := io.ReadAll(io.LimitReader(stream, runtimeLogLimit))
	if err != nil {
		return toolErrorf("read runtime logs: %v", err), nil
	}
	return jsonResult(map[string]string{
		"app_id": appID,
		"logs":   string(body),
	})
}

func (handlers toolHandlers) listEnvVars(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if result := requireScopes(ctx, "apps:read"); result != nil {
		return result, nil
	}
	if handlers.deps.EnvVars == nil {
		return toolError("env var service is not configured"), nil
	}
	appID, result := requireID(request, "app_id")
	if result != nil {
		return result, nil
	}
	envVars, err := handlers.deps.EnvVars.ListEnvVars(ctx, appID)
	if err != nil {
		return toolErrorf("list env vars: %v", err), nil
	}
	response := make([]api.EnvVarResponse, 0, len(envVars))
	for _, envVar := range envVars {
		response = append(response, envVarResponse(envVar))
	}
	return jsonResult(response)
}

func (handlers toolHandlers) setEnvVar(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if result := requireScopes(ctx, "apps:write"); result != nil {
		return result, nil
	}
	if handlers.deps.EnvVars == nil {
		return toolError("env var service is not configured"), nil
	}
	appID, result := requireID(request, "app_id")
	if result != nil {
		return result, nil
	}
	key, result := requireString(request, "key")
	if result != nil {
		return result, nil
	}
	value, result := requireString(request, "value")
	if result != nil {
		return result, nil
	}
	if err := api.ValidateEnvKey(key); err != nil {
		return toolErrorf("invalid key: %v", err), nil
	}
	envVar, err := handlers.deps.EnvVars.SetEnvVar(ctx, appID, api.SetEnvVarInput{
		Key:      key,
		Value:    value,
		IsSecret: request.GetBool("is_secret", false),
	})
	if err != nil {
		return toolErrorf("set env var: %v", err), nil
	}
	return jsonResult(envVarResponse(envVar))
}

func (handlers toolHandlers) rollbackApp(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if result := requireScopes(ctx, "apps:deploy"); result != nil {
		return result, nil
	}
	if handlers.deps.Deployments == nil {
		return toolError("deployment service is not configured"), nil
	}
	appID, result := requireID(request, "app_id")
	if result != nil {
		return result, nil
	}
	deploymentID, result := requireID(request, "deployment_id")
	if result != nil {
		return result, nil
	}
	deployment, err := handlers.deps.Deployments.RollbackApp(ctx, appID, deploymentID)
	if err != nil && deployment.ID == "" {
		return toolErrorf("rollback app: %v", err), nil
	}
	return jsonResult(deployment)
}

func (handlers toolHandlers) searchServiceTemplates(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if result := requireScopes(ctx, "services:read"); result != nil {
		return result, nil
	}
	if handlers.deps.Services == nil {
		return toolError("service manager is not configured"), nil
	}
	templates, err := handlers.deps.Services.ListTemplates(ctx, request.GetString("query", ""))
	if err != nil {
		return toolErrorf("search service templates: %v", err), nil
	}
	return jsonResult(templates)
}

func (handlers toolHandlers) deployService(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if result := requireScopes(ctx, "services:write"); result != nil {
		return result, nil
	}
	if handlers.deps.Services == nil {
		return toolError("service manager is not configured"), nil
	}
	projectID, result := requireID(request, "project_id")
	if result != nil {
		return result, nil
	}
	templateSlug, result := requireString(request, "template_slug")
	if result != nil {
		return result, nil
	}
	name, result := requireString(request, "name")
	if result != nil {
		return result, nil
	}
	if err := api.ValidateAppName(templateSlug); err != nil {
		return toolErrorf("invalid template_slug: %v", err), nil
	}
	if err := api.ValidateAppName(name); err != nil {
		return toolErrorf("invalid name: %v", err), nil
	}
	response, err := handlers.deps.Services.CreateService(ctx, api.CreateServiceInput{
		ProjectID:    projectID,
		TemplateSlug: templateSlug,
		Name:         name,
		Exposed:      request.GetBool("exposed", false),
	})
	if err != nil {
		return toolErrorf("deploy service: %v", err), nil
	}
	return jsonResult(response)
}

func (handlers toolHandlers) attachService(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if result := requireScopes(ctx, "services:write", "apps:write"); result != nil {
		return result, nil
	}
	if handlers.deps.Services == nil {
		return toolError("service manager is not configured"), nil
	}
	serviceID, result := requireID(request, "service_id")
	if result != nil {
		return result, nil
	}
	appID, result := requireID(request, "app_id")
	if result != nil {
		return result, nil
	}
	response, err := handlers.deps.Services.AttachService(ctx, serviceID, appID)
	if err != nil {
		return toolErrorf("attach service: %v", err), nil
	}
	return jsonResult(response)
}

func (handlers toolHandlers) diagnoseLatestDeployment(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if result := requireScopes(ctx, "apps:read"); result != nil {
		return result, nil
	}
	if handlers.deps.Deployments == nil {
		return toolError("deployment service is not configured"), nil
	}
	appID, result := requireID(request, "app_id")
	if result != nil {
		return result, nil
	}
	deployments, err := handlers.deps.Deployments.ListDeployments(ctx, appID)
	if err != nil {
		return toolErrorf("list deployments: %v", err), nil
	}
	if len(deployments) == 0 {
		return jsonResult(map[string]any{
			"app_id":  appID,
			"summary": "No deployments were found for this app.",
			"hints":   []string{"Create or select an app, then run porter_deploy_app."},
		})
	}
	latest := deployments[0]
	buildLog := latest.BuildLog
	if handlers.deps.Logs != nil && latest.ID != "" {
		logResponse, err := handlers.deps.Logs.GetBuildLog(ctx, latest.ID)
		if err == nil && logResponse.BuildLog != "" {
			buildLog = logResponse.BuildLog
		}
	}
	return jsonResult(map[string]any{
		"app_id":        appID,
		"deployment_id": latest.ID,
		"status":        latest.Status,
		"stage":         latest.Stage,
		"summary":       deploymentSummary(latest),
		"log_tail":      tailString(buildLog, 4000),
		"hints":         deploymentHints(latest),
	})
}

func requireScopes(ctx context.Context, scopes ...string) *mcpsdk.CallToolResult {
	principal, ok := api.PrincipalFromContext(ctx)
	if !ok {
		return toolError("authentication is required")
	}
	for _, scope := range scopes {
		if !coreauth.HasScope(principal.Scopes, scope) {
			return toolErrorf("required scope %q is missing", scope)
		}
	}
	return nil
}

func requireID(request mcpsdk.CallToolRequest, key string) (string, *mcpsdk.CallToolResult) {
	value, result := requireString(request, key)
	if result != nil {
		return "", result
	}
	if !validID(value) {
		return "", toolErrorf("%s is invalid", key)
	}
	return value, nil
}

func requireString(request mcpsdk.CallToolRequest, key string) (string, *mcpsdk.CallToolResult) {
	value, err := request.RequireString(key)
	if err != nil {
		return "", toolError(err.Error())
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", toolErrorf("required argument %q is empty", key)
	}
	return value, nil
}

func envVarResponse(envVar api.EnvVar) api.EnvVarResponse {
	value := envVar.Value
	if envVar.IsSecret {
		value = secretcrypto.MaskSecret()
	}
	return api.EnvVarResponse{
		Key:      envVar.Key,
		Value:    value,
		IsSecret: envVar.IsSecret,
	}
}

func deploymentSummary(deployment api.DeploymentResponse) string {
	if deployment.Status == "" {
		return fmt.Sprintf("Latest deployment %s has no recorded status.", deployment.ID)
	}
	if deployment.Stage == "" {
		return fmt.Sprintf("Latest deployment %s is %s.", deployment.ID, deployment.Status)
	}
	return fmt.Sprintf("Latest deployment %s is %s at stage %s.", deployment.ID, deployment.Status, deployment.Stage)
}

func deploymentHints(deployment api.DeploymentResponse) []string {
	switch strings.ToLower(deployment.Stage) {
	case "cloning":
		return []string{"Verify the git_url, branch, and repository access for this app."}
	case "building":
		return []string{"Check Dockerfile or Nixpacks build output and confirm required build files are committed."}
	case "starting", "running":
		if strings.EqualFold(deployment.Status, "failed") {
			return []string{"Check internal_port, startup command, and required runtime environment variables."}
		}
	case "rollback":
		return []string{"Inspect the target deployment image tag and app runtime logs."}
	}
	if strings.EqualFold(deployment.Status, "failed") {
		return []string{"Read the build log and runtime logs, then retry after fixing the reported error."}
	}
	if strings.EqualFold(deployment.Status, "succeeded") {
		return []string{"Latest deployment succeeded; use porter_get_runtime_logs if the app is unhealthy."}
	}
	return []string{"Use porter_get_build_log and porter_get_runtime_logs for more detail."}
}

func tailString(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	return value[len(value)-maxBytes:]
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func validID(value string) bool {
	return len(value) >= 3 && len(value) <= 128
}

func toolError(message string) *mcpsdk.CallToolResult {
	return mcpsdk.NewToolResultError(message)
}

func toolErrorf(format string, args ...any) *mcpsdk.CallToolResult {
	return mcpsdk.NewToolResultErrorf(format, args...)
}
