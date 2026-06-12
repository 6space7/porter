package mcpserver

import (
	"github.com/6space7/porter/internal/api"
	"github.com/mark3labs/mcp-go/server"
)

const Version = "0.5.0"

type Dependencies struct {
	Projects    api.ProjectService
	Apps        api.AppService
	Deployments api.DeploymentService
	Logs        api.LogService
	Services    api.ServiceManager
	EnvVars     api.EnvVarService
}

func NewServer(deps Dependencies) *server.MCPServer {
	mcpServer := server.NewMCPServer(
		"porter",
		Version,
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)
	registerTools(mcpServer, deps)
	return mcpServer
}
