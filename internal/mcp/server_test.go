package mcpserver_test

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/6space7/porter/internal/api"
	portermcp "github.com/6space7/porter/internal/mcp"
	"github.com/6space7/porter/internal/services"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpsdkserver "github.com/mark3labs/mcp-go/server"
)

func TestServerListsPorterTools(t *testing.T) {
	server := portermcp.NewServer(portermcp.Dependencies{})
	tools := server.ListTools()

	for _, name := range []string{
		"porter_list_projects",
		"porter_create_project",
		"porter_list_apps",
		"porter_create_app",
		"porter_deploy_app",
		"porter_list_deployments",
		"porter_get_build_log",
		"porter_get_runtime_logs",
		"porter_list_env_vars",
		"porter_set_env_var",
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

func TestProjectToolsCallProjectService(t *testing.T) {
	deps := newFakeDependencies()
	deps.projects.projects = []api.ProjectResponse{{ID: "proj_1", Name: "demo"}}
	server := portermcp.NewServer(deps.dependencies())

	listed := callToolJSON[[]api.ProjectResponse](t, server, portermcp.ToolListProjects, nil, "projects:read")
	if len(listed) != 1 || listed[0].ID != "proj_1" {
		t.Fatalf("listed projects = %#v", listed)
	}

	created := callToolJSON[api.ProjectResponse](t, server, portermcp.ToolCreateProject, map[string]any{
		"name": "agent-project",
	}, "projects:write")
	if created.Name != "agent-project" {
		t.Fatalf("created project = %#v", created)
	}
}

func TestAppToolsCallAppAndDeploymentServices(t *testing.T) {
	deps := newFakeDependencies()
	deps.apps.apps = []api.AppResponse{{
		ID:           "app_1",
		ProjectID:    "proj_1",
		Name:         "web",
		GitURL:       "https://github.com/example/web.git",
		Branch:       "main",
		BuildType:    "dockerfile",
		InternalPort: 3000,
		Status:       "running",
	}}
	deps.deployments.deployments = []api.DeploymentResponse{{
		ID: "dep_1", AppID: "app_1", Status: "succeeded", Stage: "running", ImageTag: "porter-web:dep_1",
	}}
	server := portermcp.NewServer(deps.dependencies())

	listed := callToolJSON[[]api.AppResponse](t, server, portermcp.ToolListApps, nil, "apps:read")
	if len(listed) != 1 || listed[0].ID != "app_1" {
		t.Fatalf("listed apps = %#v", listed)
	}

	missing := callTool(t, server, portermcp.ToolCreateApp, map[string]any{
		"project_id": "proj_1",
	}, "apps:write")
	if !missing.IsError || !strings.Contains(toolText(t, missing), "name") {
		t.Fatalf("missing name result = %#v text=%q", missing, toolText(t, missing))
	}

	created := callToolJSON[api.AppResponse](t, server, portermcp.ToolCreateApp, map[string]any{
		"project_id":    "proj_1",
		"name":          "web",
		"git_url":       "https://github.com/example/web.git",
		"branch":        "main",
		"build_type":    "nixpacks",
		"internal_port": 8080,
	}, "apps:write")
	if created.ID == "" || deps.apps.created.InternalPort != 8080 || deps.apps.created.BuildType != "nixpacks" {
		t.Fatalf("created app = %#v input=%#v", created, deps.apps.created)
	}

	deployed := callToolJSON[api.DeploymentResponse](t, server, portermcp.ToolDeployApp, map[string]any{
		"app_id": "app_1",
	}, "apps:deploy")
	if deployed.ID != "dep_new" || deps.deployments.deployedAppID != "app_1" {
		t.Fatalf("deployed = %#v appID=%q", deployed, deps.deployments.deployedAppID)
	}

	listedDeployments := callToolJSON[[]api.DeploymentResponse](t, server, portermcp.ToolListDeployments, map[string]any{
		"app_id": "app_1",
	}, "apps:read")
	if len(listedDeployments) != 1 || listedDeployments[0].ID != "dep_1" {
		t.Fatalf("listed deployments = %#v", listedDeployments)
	}
}

func TestEnvAndLogToolsReturnAgentFriendlyJSON(t *testing.T) {
	deps := newFakeDependencies()
	deps.envVars.values = []api.EnvVar{
		{AppID: "app_1", Key: "DATABASE_URL", Value: "postgres://secret", IsSecret: true},
		{AppID: "app_1", Key: "PUBLIC_MODE", Value: "test", IsSecret: false},
	}
	deps.logs.buildLog = api.BuildLogResponse{
		DeploymentID: "dep_1",
		AppID:        "app_1",
		Status:       "failed",
		Stage:        "building",
		BuildLog:     "npm ERR missing package",
	}
	deps.logs.runtimeLog = "listening on :3000\n"
	server := portermcp.NewServer(deps.dependencies())

	envVars := callToolJSON[[]api.EnvVarResponse](t, server, portermcp.ToolListEnvVars, map[string]any{
		"app_id": "app_1",
	}, "apps:read")
	if len(envVars) != 2 || envVars[0].Value == "postgres://secret" || envVars[1].Value != "test" {
		t.Fatalf("env vars = %#v", envVars)
	}

	saved := callToolJSON[api.EnvVarResponse](t, server, portermcp.ToolSetEnvVar, map[string]any{
		"app_id":    "app_1",
		"key":       "API_KEY",
		"value":     "secret-value",
		"is_secret": true,
	}, "apps:write")
	if saved.Key != "API_KEY" || saved.Value == "secret-value" || !saved.IsSecret {
		t.Fatalf("saved env var = %#v", saved)
	}

	buildLog := callToolJSON[api.BuildLogResponse](t, server, portermcp.ToolGetBuildLog, map[string]any{
		"deployment_id": "dep_1",
	}, "apps:read")
	if buildLog.BuildLog != "npm ERR missing package" {
		t.Fatalf("build log = %#v", buildLog)
	}

	runtimeLogs := callToolJSON[map[string]string](t, server, portermcp.ToolGetRuntimeLogs, map[string]any{
		"app_id": "app_1",
	}, "apps:read")
	if runtimeLogs["logs"] != deps.logs.runtimeLog {
		t.Fatalf("runtime logs = %#v", runtimeLogs)
	}
}

func TestServiceToolsEnforceScopesAndCallServiceManager(t *testing.T) {
	deps := newFakeDependencies()
	deps.services.templates = []services.Template{
		{Slug: "postgres", Name: "PostgreSQL", Category: "database", Image: "postgres:16", InternalPort: 5432},
	}
	server := portermcp.NewServer(deps.dependencies())

	templates := callToolJSON[[]services.Template](t, server, portermcp.ToolSearchServiceTemplates, map[string]any{
		"query": "post",
	}, "services:read")
	if len(templates) != 1 || templates[0].Slug != "postgres" {
		t.Fatalf("templates = %#v", templates)
	}

	forbidden := callTool(t, server, portermcp.ToolDeployService, map[string]any{
		"project_id":    "proj_1",
		"template_slug": "postgres",
		"name":          "db",
	}, "services:read")
	if !forbidden.IsError || !strings.Contains(toolText(t, forbidden), "services:write") {
		t.Fatalf("forbidden service deploy = %#v text=%q", forbidden, toolText(t, forbidden))
	}

	created := callToolJSON[api.CreateServiceResponse](t, server, portermcp.ToolDeployService, map[string]any{
		"project_id":    "proj_1",
		"template_slug": "postgres",
		"name":          "db",
		"exposed":       true,
	}, "services:write")
	if created.Service.ID != "svc_new" || !deps.services.created.Exposed {
		t.Fatalf("created service = %#v input=%#v", created, deps.services.created)
	}

	attached := callToolJSON[api.AttachServiceResponse](t, server, portermcp.ToolAttachService, map[string]any{
		"service_id": "svc_new",
		"app_id":     "app_1",
	}, "services:write", "apps:write")
	if attached.Env["DATABASE_URL"] == "" {
		t.Fatalf("attached service = %#v", attached)
	}
}

func TestDeploymentDiagnosticsSummarizeLatestFailure(t *testing.T) {
	deps := newFakeDependencies()
	deps.deployments.deployments = []api.DeploymentResponse{{
		ID: "dep_failed", AppID: "app_1", Status: "failed", Stage: "building",
	}}
	deps.logs.buildLog = api.BuildLogResponse{
		DeploymentID: "dep_failed",
		AppID:        "app_1",
		Status:       "failed",
		Stage:        "building",
		BuildLog:     strings.Repeat("x", 5000) + " missing package.json",
	}
	server := portermcp.NewServer(deps.dependencies())

	diagnosis := callToolJSON[map[string]any](t, server, portermcp.ToolDiagnoseLatest, map[string]any{
		"app_id": "app_1",
	}, "apps:read")
	if diagnosis["deployment_id"] != "dep_failed" || diagnosis["stage"] != "building" {
		t.Fatalf("diagnosis = %#v", diagnosis)
	}
	logTail, _ := diagnosis["log_tail"].(string)
	if !strings.Contains(logTail, "missing package.json") || len(logTail) > 4200 {
		t.Fatalf("diagnosis log_tail length=%d text suffix=%q", len(logTail), logTail[max(0, len(logTail)-80):])
	}
	hints, _ := diagnosis["hints"].([]any)
	if len(hints) == 0 {
		t.Fatalf("diagnosis hints = %#v", diagnosis)
	}
}

func callToolJSON[T any](t *testing.T, server interface {
	GetTool(string) *mcpsdkserver.ServerTool
}, name string, args map[string]any, scopes ...string) T {
	t.Helper()
	var decoded T
	result := callTool(t, server, name, args, scopes...)
	if result.IsError {
		t.Fatalf("%s returned error result: %s", name, toolText(t, result))
	}
	if err := json.Unmarshal([]byte(toolText(t, result)), &decoded); err != nil {
		t.Fatalf("decode %s JSON %q: %v", name, toolText(t, result), err)
	}
	return decoded
}

func callTool(t *testing.T, server interface {
	GetTool(string) *mcpsdkserver.ServerTool
}, name string, args map[string]any, scopes ...string) *mcpsdk.CallToolResult {
	t.Helper()
	tool := server.GetTool(name)
	if tool == nil {
		t.Fatalf("tool %q not registered", name)
	}
	ctx := api.ContextWithPrincipal(context.Background(), api.Principal{TokenID: "tok_agent", Scopes: scopes})
	result, err := tool.Handler(ctx, mcpsdk.CallToolRequest{
		Params: mcpsdk.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	})
	if err != nil {
		t.Fatalf("%s handler returned error: %v", name, err)
	}
	return result
}

func toolText(t *testing.T, result *mcpsdk.CallToolResult) string {
	t.Helper()
	if result == nil || len(result.Content) == 0 {
		t.Fatal("empty tool result")
	}
	text, ok := result.Content[0].(mcpsdk.TextContent)
	if !ok {
		t.Fatalf("content[0] = %#v, want TextContent", result.Content[0])
	}
	return text.Text
}

type fakeDependencies struct {
	projects    *fakeProjectService
	apps        *fakeAppService
	deployments *fakeDeploymentService
	logs        *fakeLogService
	services    *fakeServiceManager
	envVars     *fakeEnvVarService
}

func newFakeDependencies() fakeDependencies {
	return fakeDependencies{
		projects:    &fakeProjectService{},
		apps:        &fakeAppService{},
		deployments: &fakeDeploymentService{},
		logs:        &fakeLogService{},
		services:    &fakeServiceManager{},
		envVars:     &fakeEnvVarService{},
	}
}

func (deps fakeDependencies) dependencies() portermcp.Dependencies {
	return portermcp.Dependencies{
		Projects:    deps.projects,
		Apps:        deps.apps,
		Deployments: deps.deployments,
		Logs:        deps.logs,
		Services:    deps.services,
		EnvVars:     deps.envVars,
	}
}

type fakeProjectService struct {
	projects []api.ProjectResponse
}

func (svc *fakeProjectService) CreateProject(_ context.Context, name string) (api.ProjectResponse, error) {
	project := api.ProjectResponse{ID: "proj_new", Name: name}
	svc.projects = append(svc.projects, project)
	return project, nil
}

func (svc *fakeProjectService) ListProjects(context.Context) ([]api.ProjectResponse, error) {
	return append([]api.ProjectResponse(nil), svc.projects...), nil
}

func (svc *fakeProjectService) GetProject(context.Context, string) (api.ProjectResponse, error) {
	return api.ProjectResponse{}, api.ErrNotFound
}

func (svc *fakeProjectService) UpdateProject(context.Context, string, string) (api.ProjectResponse, error) {
	return api.ProjectResponse{}, api.ErrNotFound
}

func (svc *fakeProjectService) DeleteProject(context.Context, string) error {
	return api.ErrNotFound
}

type fakeAppService struct {
	apps    []api.AppResponse
	created api.CreateAppInput
}

func (svc *fakeAppService) CreateApp(_ context.Context, input api.CreateAppInput) (api.AppResponse, error) {
	svc.created = input
	app := api.AppResponse{
		ID:           "app_new",
		ProjectID:    input.ProjectID,
		Name:         input.Name,
		GitURL:       input.GitURL,
		Branch:       input.Branch,
		BuildType:    input.BuildType,
		InternalPort: input.InternalPort,
		Status:       "created",
	}
	return app, nil
}

func (svc *fakeAppService) ListApps(context.Context) ([]api.AppResponse, error) {
	return append([]api.AppResponse(nil), svc.apps...), nil
}

func (svc *fakeAppService) GetApp(context.Context, string) (api.AppResponse, error) {
	return api.AppResponse{}, api.ErrNotFound
}

func (svc *fakeAppService) UpdateApp(context.Context, string, api.UpdateAppInput) (api.AppResponse, error) {
	return api.AppResponse{}, api.ErrNotFound
}

func (svc *fakeAppService) DeleteApp(context.Context, string) error {
	return api.ErrNotFound
}

func (svc *fakeAppService) StopApp(context.Context, string) (api.AppResponse, error) {
	return api.AppResponse{}, api.ErrNotFound
}

func (svc *fakeAppService) StartApp(context.Context, string) (api.AppResponse, error) {
	return api.AppResponse{}, api.ErrNotFound
}

func (svc *fakeAppService) RestartApp(context.Context, string) (api.AppResponse, error) {
	return api.AppResponse{}, api.ErrNotFound
}

type fakeDeploymentService struct {
	deployments   []api.DeploymentResponse
	deployedAppID string
}

func (svc *fakeDeploymentService) DeployApp(_ context.Context, appID string) (api.DeploymentResponse, error) {
	svc.deployedAppID = appID
	return api.DeploymentResponse{ID: "dep_new", AppID: appID, Status: "queued", Stage: "queued"}, nil
}

func (svc *fakeDeploymentService) ListDeployments(_ context.Context, _ string) ([]api.DeploymentResponse, error) {
	return append([]api.DeploymentResponse(nil), svc.deployments...), nil
}

func (svc *fakeDeploymentService) RollbackApp(_ context.Context, appID, deploymentID string) (api.DeploymentResponse, error) {
	return api.DeploymentResponse{ID: "dep_rollback", AppID: appID, Status: "queued", Stage: "rollback", ImageTag: deploymentID}, nil
}

type fakeLogService struct {
	buildLog   api.BuildLogResponse
	runtimeLog string
}

func (svc *fakeLogService) GetBuildLog(context.Context, string) (api.BuildLogResponse, error) {
	return svc.buildLog, nil
}

func (svc *fakeLogService) StreamRuntimeLogs(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(svc.runtimeLog)), nil
}

type fakeServiceManager struct {
	templates []services.Template
	created   api.CreateServiceInput
}

func (svc *fakeServiceManager) ListTemplates(_ context.Context, query string) ([]services.Template, error) {
	if query == "" {
		return append([]services.Template(nil), svc.templates...), nil
	}
	var matches []services.Template
	for _, tmpl := range svc.templates {
		if strings.Contains(strings.ToLower(tmpl.Slug+" "+tmpl.Name), strings.ToLower(query)) {
			matches = append(matches, tmpl)
		}
	}
	return matches, nil
}

func (svc *fakeServiceManager) GetTemplate(context.Context, string) (services.Template, error) {
	return services.Template{}, api.ErrNotFound
}

func (svc *fakeServiceManager) CreateService(_ context.Context, input api.CreateServiceInput) (api.CreateServiceResponse, error) {
	svc.created = input
	return api.CreateServiceResponse{
		Service: api.ServiceResponse{
			ID:           "svc_new",
			ProjectID:    input.ProjectID,
			TemplateSlug: input.TemplateSlug,
			Name:         input.Name,
			Status:       "running",
			InternalPort: 5432,
			Exposed:      input.Exposed,
		},
		Credentials: map[string]string{"password": "one-time"},
		Provides:    map[string]string{"DATABASE_URL": "postgres://db"},
	}, nil
}

func (svc *fakeServiceManager) ListServices(context.Context) ([]api.ServiceResponse, error) {
	return nil, nil
}

func (svc *fakeServiceManager) GetService(context.Context, string) (api.ServiceResponse, error) {
	return api.ServiceResponse{}, api.ErrNotFound
}

func (svc *fakeServiceManager) AttachService(_ context.Context, serviceID, appID string) (api.AttachServiceResponse, error) {
	return api.AttachServiceResponse{
		ServiceID: serviceID,
		AppID:     appID,
		Env:       map[string]string{"DATABASE_URL": "postgres://db"},
	}, nil
}

type fakeEnvVarService struct {
	values []api.EnvVar
}

func (svc *fakeEnvVarService) SetEnvVar(_ context.Context, appID string, input api.SetEnvVarInput) (api.EnvVar, error) {
	envVar := api.EnvVar{AppID: appID, Key: input.Key, Value: input.Value, IsSecret: input.IsSecret}
	svc.values = append(svc.values, envVar)
	return envVar, nil
}

func (svc *fakeEnvVarService) ListEnvVars(context.Context, string) ([]api.EnvVar, error) {
	return append([]api.EnvVar(nil), svc.values...), nil
}
